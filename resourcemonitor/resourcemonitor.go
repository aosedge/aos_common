// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2022 Renesas Electronics Corporation.
// Copyright (C) 2022 EPAM Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package resourcemonitor AOS Core Monitoring Component
package resourcemonitor

import (
	"container/list"
	"context"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/aosedge/aos_common/aoserrors"
	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"
	"github.com/aosedge/aos_common/utils/fs"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	log "github.com/sirupsen/logrus"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

// Service status.
const (
	MinutePeriod = iota
	HourPeriod
	DayPeriod
	MonthPeriod
	YearPeriod
)

// For optimization capacity should be equals numbers of measurement values
// 5 - RAM, CPU, UsedDisk, InTraffic, OutTraffic.
const capacityAlertProcessorElements = 5

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

type SystemUsageProvider interface {
	CacheSystemInfos()
	FillSystemInfo(instanceID string, instance *instanceMonitoring) error
}

// QuotaAlert quota alert structure.
type QuotaAlert struct {
	Timestamp time.Time
	Parameter string
	Value     uint64
	Status    string
}

// SystemQuotaAlert system quota alert structure.
type SystemQuotaAlert struct {
	QuotaAlert
}

// InstanceQuotaAlert instance quota alert structure.
type InstanceQuotaAlert struct {
	InstanceIdent aostypes.InstanceIdent
	QuotaAlert
}

// AlertSender interface to send resource alerts.
type AlertSender interface {
	SendSystemQuotaAlert(alert SystemQuotaAlert)
	SendInstanceQuotaAlert(alert InstanceQuotaAlert)
}

// MonitoringSender sends monitoring data.
type MonitoringSender interface {
	SendMonitoringData(monitoringData cloudprotocol.NodeMonitoringData)
}

// TrafficMonitoring interface to get network traffic.
type TrafficMonitoring interface {
	GetSystemTraffic() (inputTraffic, outputTraffic uint64, err error)
	GetInstanceTraffic(instanceID string) (inputTraffic, outputTraffic uint64, err error)
}

// PartitionConfig partition information.
type PartitionConfig struct {
	Name  string   `json:"name"`
	Types []string `json:"types"`
	Path  string   `json:"path"`
}

// Config configuration for resource monitoring.
type Config struct {
	aostypes.AlertRules
	SendPeriod aostypes.Duration `json:"sendPeriod"`
	PollPeriod aostypes.Duration `json:"pollPeriod"`
	Partitions []PartitionConfig `json:"partitions"`
	Source     string            `json:"source"`
}

type SystemInfo struct {
	NumCPUs    uint64                        `json:"numCpus"`
	TotalRAM   uint64                        `json:"totalRam"`
	Partitions []cloudprotocol.PartitionInfo `json:"partitions"`
}

// ResourceMonitor instance.
type ResourceMonitor struct {
	sync.Mutex

	alertSender      AlertSender
	monitoringSender MonitoringSender

	config Config
	nodeID string

	sendTimer *time.Ticker
	pollTimer *time.Ticker

	nodeMonitoringData cloudprotocol.MonitoringData
	systemInfo         SystemInfo

	alertProcessors *list.List

	instanceMonitoringMap map[string]*instanceMonitoring
	trafficMonitoring     TrafficMonitoring
	sourceSystemUsage     SystemUsageProvider

	cancelFunction context.CancelFunc
}

// PartitionParam partition instance information.
type PartitionParam struct {
	Name string
	Path string
}

// ResourceMonitorParams instance resource monitor parameters.
type ResourceMonitorParams struct {
	aostypes.InstanceIdent
	UID        int
	GID        int
	AlertRules *aostypes.AlertRules
	Partitions []PartitionParam
}

type instanceMonitoring struct {
	uid                    uint32
	gid                    uint32
	partitions             []PartitionParam
	monitoringData         cloudprotocol.InstanceMonitoringData
	alertProcessorElements []*list.Element
	prevCPU                uint64
	prevTime               time.Time
}

/***********************************************************************************************************************
 * Variable
 **********************************************************************************************************************/

// These global variables are used to be able to mocking the functionality of getting quota in tests.
//
//nolint:gochecknoglobals
var (
	systemCPUPercent                            = cpu.Percent
	systemVirtualMemory                         = mem.VirtualMemory
	systemDiskUsage                             = disk.Usage
	getUserFSQuotaUsage                         = fs.GetUserFSQuotaUsage
	cpuCount                                    = runtime.NumCPU()
	hostSystemUsageInstance SystemUsageProvider = nil
)

/***********************************************************************************************************************
 * Public
 **********************************************************************************************************************/

// New creates new resource monitor instance.
func New(
	nodeID string, config Config, alertsSender AlertSender, monitoringSender MonitoringSender,
	trafficMonitoring TrafficMonitoring) (
	monitor *ResourceMonitor, err error,
) {
	log.Debug("Create monitor")

	monitor = &ResourceMonitor{
		alertSender:       alertsSender,
		monitoringSender:  monitoringSender,
		trafficMonitoring: trafficMonitoring,
		config:            config,
		nodeID:            nodeID,
		sourceSystemUsage: getSourceSystemUsage(config.Source),
	}

	monitor.alertProcessors = list.New()

	if monitor.config.CPU != nil {
		monitor.alertProcessors.PushBack(createAlertProcessor(
			"System CPU",
			&monitor.nodeMonitoringData.CPU,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendSystemQuotaAlert(prepareSystemAlertItem("cpu", time, value, status))
			},
			*monitor.config.CPU))
	}

	if monitor.config.RAM != nil {
		monitor.alertProcessors.PushBack(createAlertProcessor(
			"System RAM",
			&monitor.nodeMonitoringData.RAM,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendSystemQuotaAlert(prepareSystemAlertItem("ram", time, value, status))
			},
			*monitor.config.RAM))
	}

	monitor.nodeMonitoringData.Disk = make([]cloudprotocol.PartitionUsage, len(config.Partitions))

	for i, partitionParam := range config.Partitions {
		monitor.nodeMonitoringData.Disk[i].Name = partitionParam.Name
	}

	if len(monitor.config.UsedDisks) > 0 {
		for _, diskRule := range monitor.config.UsedDisks {
			for i := 0; i < len(monitor.nodeMonitoringData.Disk); i++ {
				if diskRule.Name != monitor.nodeMonitoringData.Disk[i].Name {
					continue
				}

				monitor.alertProcessors.PushBack(createAlertProcessor(
					"Partition "+monitor.nodeMonitoringData.Disk[i].Name,
					&monitor.nodeMonitoringData.Disk[i].UsedSize,
					func(time time.Time, value uint64, status string) {
						monitor.alertSender.SendSystemQuotaAlert(prepareSystemAlertItem(
							monitor.nodeMonitoringData.Disk[i].Name, time, value, status))
					},
					diskRule.AlertRuleParam))

				break
			}
		}
	}

	if monitor.config.InTraffic != nil {
		monitor.alertProcessors.PushBack(createAlertProcessor(
			"IN Traffic",
			&monitor.nodeMonitoringData.InTraffic,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendSystemQuotaAlert(prepareSystemAlertItem("inTraffic", time, value, status))
			},
			*monitor.config.InTraffic))
	}

	if monitor.config.OutTraffic != nil {
		monitor.alertProcessors.PushBack(createAlertProcessor(
			"OUT Traffic",
			&monitor.nodeMonitoringData.OutTraffic,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendSystemQuotaAlert(prepareSystemAlertItem("outTraffic", time, value, status))
			},
			*monitor.config.OutTraffic))
	}

	monitor.instanceMonitoringMap = make(map[string]*instanceMonitoring)

	ctx, cancelFunc := context.WithCancel(context.Background())
	monitor.cancelFunction = cancelFunc

	monitor.pollTimer = time.NewTicker(monitor.config.PollPeriod.Duration)
	monitor.sendTimer = time.NewTicker(monitor.config.SendPeriod.Duration)

	if err = monitor.gatheringSystemInfo(); err != nil {
		return nil, err
	}

	go monitor.run(ctx)

	return monitor, nil
}

// Close closes monitor instance.
func (monitor *ResourceMonitor) Close() {
	log.Debug("Close monitor")

	if monitor.sendTimer != nil {
		monitor.sendTimer.Stop()
	}

	if monitor.pollTimer != nil {
		monitor.pollTimer.Stop()
	}

	if monitor.cancelFunction != nil {
		monitor.cancelFunction()
	}
}

func (monitor *ResourceMonitor) GetSystemInfo() SystemInfo {
	return monitor.systemInfo
}

// StartInstanceMonitor starts monitoring service.
func (monitor *ResourceMonitor) StartInstanceMonitor(
	instanceID string, monitoringConfig ResourceMonitorParams,
) error {
	monitor.Lock()
	defer monitor.Unlock()

	if _, ok := monitor.instanceMonitoringMap[instanceID]; ok {
		log.WithField("id", instanceID).Warning("Service already under monitoring")

		return nil
	}

	log.WithFields(log.Fields{"id": instanceID}).Debug("Start instance monitoring")

	monitor.instanceMonitoringMap[instanceID] = monitor.createInstanceMonitoring(
		instanceID, monitoringConfig.AlertRules, monitoringConfig)

	return nil
}

// StopInstanceMonitor stops monitoring service.
func (monitor *ResourceMonitor) StopInstanceMonitor(instanceID string) error {
	monitor.Lock()
	defer monitor.Unlock()

	log.WithField("id", instanceID).Debug("Stop instance monitoring")

	if _, ok := monitor.instanceMonitoringMap[instanceID]; !ok {
		return nil
	}

	for _, e := range monitor.instanceMonitoringMap[instanceID].alertProcessorElements {
		monitor.alertProcessors.Remove(e)
	}

	delete(monitor.instanceMonitoringMap, instanceID)

	return nil
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func (monitor *ResourceMonitor) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-monitor.sendTimer.C:
			monitor.Lock()
			monitoringData := monitor.prepareMonitoringData()
			monitor.sendMonitoringData(monitoringData)
			monitor.Unlock()

		case <-monitor.pollTimer.C:
			monitor.Lock()
			monitor.sourceSystemUsage.CacheSystemInfos()
			monitor.getCurrentSystemData()
			monitor.getCurrentInstanceData()
			monitor.processAlerts()
			monitor.Unlock()
		}
	}
}

func (monitor *ResourceMonitor) createInstanceMonitoring(
	instanceID string, rules *aostypes.AlertRules, monitoringConfig ResourceMonitorParams,
) *instanceMonitoring {
	serviceMonitoring := &instanceMonitoring{
		uid:            uint32(monitoringConfig.UID),
		gid:            uint32(monitoringConfig.GID),
		partitions:     monitoringConfig.Partitions,
		monitoringData: cloudprotocol.InstanceMonitoringData{InstanceIdent: monitoringConfig.InstanceIdent},
	}

	if monitor.alertSender == nil {
		return serviceMonitoring
	}

	serviceMonitoring.monitoringData.Disk = make(
		[]cloudprotocol.PartitionUsage, len(monitoringConfig.Partitions))

	for i, partitionParam := range monitoringConfig.Partitions {
		serviceMonitoring.monitoringData.Disk[i].Name = partitionParam.Name
	}

	if rules == nil {
		return serviceMonitoring
	}

	serviceMonitoring.alertProcessorElements = make([]*list.Element, 0, capacityAlertProcessorElements)

	if rules.CPU != nil {
		e := monitor.alertProcessors.PushBack(createAlertProcessor(
			instanceID+" CPU",
			&serviceMonitoring.monitoringData.CPU,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendInstanceQuotaAlert(
					prepareInstanceAlertItem(monitoringConfig.InstanceIdent, "cpu", time, value, status))
			}, *rules.CPU))

		serviceMonitoring.alertProcessorElements = append(serviceMonitoring.alertProcessorElements, e)
	}

	if rules.RAM != nil {
		e := monitor.alertProcessors.PushBack(createAlertProcessor(
			instanceID+" RAM",
			&serviceMonitoring.monitoringData.RAM,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendInstanceQuotaAlert(
					prepareInstanceAlertItem(monitoringConfig.InstanceIdent, "ram", time, value, status))
			}, *rules.RAM))

		serviceMonitoring.alertProcessorElements = append(serviceMonitoring.alertProcessorElements, e)
	}

	if len(rules.UsedDisks) > 0 {
		for _, diskRule := range rules.UsedDisks {
			for i := 0; i < len(serviceMonitoring.monitoringData.Disk); i++ {
				if diskRule.Name != serviceMonitoring.monitoringData.Disk[i].Name {
					continue
				}

				e := monitor.alertProcessors.PushBack(createAlertProcessor(
					instanceID+" Partition "+serviceMonitoring.monitoringData.Disk[i].Name,
					&serviceMonitoring.monitoringData.Disk[i].UsedSize,
					func(time time.Time, value uint64, status string) {
						monitor.alertSender.SendInstanceQuotaAlert(
							prepareInstanceAlertItem(
								monitoringConfig.InstanceIdent, serviceMonitoring.monitoringData.Disk[i].Name,
								time, value, status))
					}, diskRule.AlertRuleParam))

				serviceMonitoring.alertProcessorElements = append(serviceMonitoring.alertProcessorElements, e)

				break
			}
		}
	}

	if rules.InTraffic != nil {
		e := monitor.alertProcessors.PushBack(createAlertProcessor(
			instanceID+" Traffic IN",
			&serviceMonitoring.monitoringData.InTraffic,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendInstanceQuotaAlert(
					prepareInstanceAlertItem(monitoringConfig.InstanceIdent, "inTraffic", time, value, status))
			}, *rules.InTraffic))

		serviceMonitoring.alertProcessorElements = append(serviceMonitoring.alertProcessorElements, e)
	}

	if rules.OutTraffic != nil {
		e := monitor.alertProcessors.PushBack(createAlertProcessor(
			instanceID+" Traffic OUT",
			&serviceMonitoring.monitoringData.OutTraffic,
			func(time time.Time, value uint64, status string) {
				monitor.alertSender.SendInstanceQuotaAlert(
					prepareInstanceAlertItem(monitoringConfig.InstanceIdent, "outTraffic", time, value, status))
			}, *rules.OutTraffic))

		serviceMonitoring.alertProcessorElements = append(serviceMonitoring.alertProcessorElements, e)
	}

	return serviceMonitoring
}

func (monitor *ResourceMonitor) gatheringSystemInfo() (err error) {
	monitor.systemInfo.NumCPUs = uint64(cpuCount)

	memStat, err := systemVirtualMemory()
	if err != nil {
		return aoserrors.Wrap(err)
	}

	monitor.systemInfo.TotalRAM = memStat.Total

	monitor.systemInfo.Partitions = make([]cloudprotocol.PartitionInfo, len(monitor.config.Partitions))
	for i, partition := range monitor.config.Partitions {
		monitor.systemInfo.Partitions[i].Name = partition.Name
		monitor.systemInfo.Partitions[i].Types = append(monitor.systemInfo.Partitions[i].Types, partition.Types...)

		usageStat, err := systemDiskUsage(partition.Path)
		if err != nil {
			return aoserrors.Wrap(err)
		}

		monitor.systemInfo.Partitions[i].TotalSize = usageStat.Total
	}

	return nil
}

func (monitor *ResourceMonitor) prepareMonitoringData() cloudprotocol.NodeMonitoringData {
	monitoringData := cloudprotocol.NodeMonitoringData{
		MonitoringData:   monitor.nodeMonitoringData,
		NodeID:           monitor.nodeID,
		Timestamp:        time.Now(),
		ServiceInstances: make([]cloudprotocol.InstanceMonitoringData, 0, len(monitor.instanceMonitoringMap)),
	}

	for _, instance := range monitor.instanceMonitoringMap {
		monitoringData.ServiceInstances = append(monitoringData.ServiceInstances, instance.monitoringData)
	}

	return monitoringData
}

func (monitor *ResourceMonitor) sendMonitoringData(nodeMonitoringData cloudprotocol.NodeMonitoringData) {
	monitor.monitoringSender.SendMonitoringData(nodeMonitoringData)
}

func (monitor *ResourceMonitor) getCurrentSystemData() {
	cpu, err := getSystemCPUUsage()
	if err != nil {
		log.Errorf("Can't get system CPU: %s", err)
	}

	monitor.nodeMonitoringData.CPU = uint64(math.Round(cpu))

	monitor.nodeMonitoringData.RAM, err = getSystemRAMUsage()
	if err != nil {
		log.Errorf("Can't get system RAM: %s", err)
	}

	if len(monitor.nodeMonitoringData.Disk) > 0 {
		for i, partitionParam := range monitor.config.Partitions {
			monitor.nodeMonitoringData.Disk[i].UsedSize, err = getSystemDiskUsage(partitionParam.Path)
			if err != nil {
				log.Errorf("Can't get system Disk usage: %v", err)
			}
		}
	}

	if monitor.trafficMonitoring != nil {
		inTraffic, outTraffic, err := monitor.trafficMonitoring.GetSystemTraffic()
		if err != nil {
			log.Errorf("Can't get system traffic value: %s", err)
		}

		monitor.nodeMonitoringData.InTraffic = inTraffic
		monitor.nodeMonitoringData.OutTraffic = outTraffic
	}

	log.WithFields(log.Fields{
		"CPU":  monitor.nodeMonitoringData.CPU,
		"RAM":  monitor.nodeMonitoringData.RAM,
		"Disk": monitor.nodeMonitoringData.Disk,
		"IN":   monitor.nodeMonitoringData.InTraffic,
		"OUT":  monitor.nodeMonitoringData.OutTraffic,
	}).Debug("Monitoring data")
}

func (monitor *ResourceMonitor) getCurrentInstanceData() {
	for instanceID, value := range monitor.instanceMonitoringMap {
		err := monitor.sourceSystemUsage.FillSystemInfo(instanceID, value)
		if err != nil {
			log.Errorf("Can't fill system usage info: %v", err)
		}

		for i, partitionParam := range value.partitions {
			value.monitoringData.Disk[i].UsedSize, err = getInstanceDiskUsage(partitionParam.Path, value.uid, value.gid)
			if err != nil {
				log.Errorf("Can't get service Disc usage: %v", err)
			}
		}

		if monitor.trafficMonitoring != nil {
			inTraffic, outTraffic, err := monitor.trafficMonitoring.GetInstanceTraffic(instanceID)
			if err != nil {
				log.Errorf("Can't get service traffic: %s", err)
			}

			value.monitoringData.InTraffic = inTraffic
			value.monitoringData.OutTraffic = outTraffic
		}

		log.WithFields(log.Fields{
			"id":   instanceID,
			"CPU":  value.monitoringData.CPU,
			"RAM":  value.monitoringData.RAM,
			"Disk": value.monitoringData.Disk,
			"IN":   value.monitoringData.InTraffic,
			"OUT":  value.monitoringData.OutTraffic,
		}).Debug("Instance monitoring data")
	}
}

func (monitor *ResourceMonitor) processAlerts() {
	currentTime := time.Now()

	for e := monitor.alertProcessors.Front(); e != nil; e = e.Next() {
		alertProcessor, ok := e.Value.(*alertProcessor)

		if !ok {
			log.Error("Unexpected alert processors type")
			return
		}

		alertProcessor.checkAlertDetection(currentTime)
	}
}

// getSystemCPUUsage returns CPU usage in percent.
func getSystemCPUUsage() (cpuUse float64, err error) {
	v, err := systemCPUPercent(0, false)
	if err != nil {
		return 0, aoserrors.Wrap(err)
	}

	cpuUse = v[0]

	return cpuUse, nil
}

// getSystemRAMUsage returns RAM usage in bytes.
func getSystemRAMUsage() (ram uint64, err error) {
	v, err := systemVirtualMemory()
	if err != nil {
		return ram, aoserrors.Wrap(err)
	}

	return v.Used, nil
}

// getSystemDiskUsage returns disc usage in bytes.
func getSystemDiskUsage(path string) (discUse uint64, err error) {
	v, err := systemDiskUsage(path)
	if err != nil {
		return discUse, aoserrors.Wrap(err)
	}

	return v.Used, nil
}

// getServiceDiskUsage returns service disk usage in bytes.
func getInstanceDiskUsage(path string, uid, gid uint32) (diskUse uint64, err error) {
	if diskUse, err = getUserFSQuotaUsage(path, uid, gid); err != nil {
		return diskUse, aoserrors.Wrap(err)
	}

	return diskUse, nil
}

func prepareSystemAlertItem(parameter string, timestamp time.Time, value uint64, status string) SystemQuotaAlert {
	return SystemQuotaAlert{
		QuotaAlert: QuotaAlert{
			Timestamp: timestamp,
			Parameter: parameter,
			Value:     value,
			Status:    status,
		},
	}
}

func prepareInstanceAlertItem(
	instanceIndent aostypes.InstanceIdent, parameter string, timestamp time.Time, value uint64, status string,
) InstanceQuotaAlert {
	return InstanceQuotaAlert{
		InstanceIdent: instanceIndent,
		QuotaAlert: QuotaAlert{
			Timestamp: timestamp,
			Parameter: parameter,
			Value:     value,
			Status:    status,
		},
	}
}

func getSourceSystemUsage(source string) SystemUsageProvider {
	if source == "xentop" {
		return &xenSystemUsage{}
	}

	if hostSystemUsageInstance != nil {
		return hostSystemUsageInstance
	}

	return &cgroupsSystemUsage{}
}
