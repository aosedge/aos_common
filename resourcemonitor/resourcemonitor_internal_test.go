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

package resourcemonitor

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/aostypes"
	"github.com/aoscloud/aos_common/api/cloudprotocol"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"
)

/***********************************************************************************************************************
 * Init
 **********************************************************************************************************************/

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: false,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		FullTimestamp:    true,
	})
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stdout)
}

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

type testAlertsSender struct {
	alertCallback func(alert cloudprotocol.AlertItem)
}

type testMonitoringSender struct {
	monitoringData chan cloudprotocol.NodeMonitoringData
}

type testTrafficMonitoring struct {
	inputTraffic, outputTraffic uint64
}

type testQuotaData struct {
	cpu       float64
	ram       uint64
	totalRAM  uint64
	disk      uint64
	totalDisk uint64
	cores     int
}

type testAlertData struct {
	alerts            []cloudprotocol.AlertItem
	monitoringData    cloudprotocol.NodeMonitoringData
	trafficMonitoring testTrafficMonitoring
	quotaData         testQuotaData
	monitoringConfig  ResourceMonitorParams
}

type testProcessData struct {
	uid       int32
	quotaData testQuotaData
}

type testHostSystemUsage struct {
	instances map[string]struct{ cpu, ram uint64 }
}

/***********************************************************************************************************************
 * Variable
 **********************************************************************************************************************/

var (
	systemQuotaData               testQuotaData
	instanceTrafficMonitoringData map[string]testTrafficMonitoring
	processesData                 []*testProcessData
	numCPU                        = runtime.NumCPU()
)

/***********************************************************************************************************************
 * Main
 **********************************************************************************************************************/

func TestMain(m *testing.M) {
	numCPU = 2

	ret := m.Run()

	os.Exit(ret)
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestAlertProcessor(t *testing.T) {
	var sourceValue uint64

	destination := make([]uint64, 0, 2)

	alert := createAlertProcessor(
		"Test",
		&sourceValue,
		func(time time.Time, value uint64) {
			log.Debugf("T: %s, %d", time, value)
			destination = append(destination, value)
		},
		aostypes.AlertRuleParam{
			Timeout: aostypes.Duration{Duration: 3 * time.Second},
			Low:     80,
			High:    90,
		})

	values := []uint64{50, 91, 79, 92, 93, 94, 95, 94, 79, 91, 92, 93, 94, 32, 91, 92, 93, 94, 95, 96}
	alertsCount := []int{0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3}

	currentTime := time.Now()

	for i, value := range values {
		sourceValue = value

		alert.checkAlertDetection(currentTime)

		if alertsCount[i] != len(destination) {
			t.Errorf("Wrong alert count %d at %d", len(destination), i)
		}

		currentTime = currentTime.Add(time.Second)
	}
}

func TestSystemAlerts(t *testing.T) {
	duration := 100 * time.Millisecond

	var alerts []cloudprotocol.AlertItem

	sender := &testAlertsSender{
		alertCallback: func(alert cloudprotocol.AlertItem) {
			alerts = append(alerts, alert)
		},
	}

	monitoringSender := &testMonitoringSender{
		monitoringData: make(chan cloudprotocol.NodeMonitoringData, 1),
	}

	var trafficMonitoring testTrafficMonitoring

	systemCPUPercent = getSystemCPUPercent
	systemVirtualMemory = getSystemRAM
	systemDiskUsage = getSystemDisk

	config := Config{
		AlertRules: aostypes.AlertRules{
			CPU: &aostypes.AlertRuleParam{
				Timeout: aostypes.Duration{},
				Low:     30,
				High:    40,
			},
			RAM: &aostypes.AlertRuleParam{
				Timeout: aostypes.Duration{},
				Low:     1000,
				High:    2000,
			},
			UsedDisks: []aostypes.PartitionAlertRuleParam{
				{
					AlertRuleParam: aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     2000,
						High:    4000,
					},
					Name: cloudprotocol.GenericPartition,
				},
			},
			InTraffic: &aostypes.AlertRuleParam{
				Timeout: aostypes.Duration{},
				Low:     100,
				High:    200,
			},
			OutTraffic: &aostypes.AlertRuleParam{
				Timeout: aostypes.Duration{},
				Low:     100,
				High:    200,
			},
		},
		SendPeriod: aostypes.Duration{Duration: duration},
		PollPeriod: aostypes.Duration{Duration: duration},
		Partitions: []PartitionConfig{{Name: cloudprotocol.GenericPartition, Path: "."}},
	}

	testData := []testAlertData{
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 150, outputTraffic: 150},
			monitoringData: cloudprotocol.NodeMonitoringData{
				MonitoringData: cloudprotocol.MonitoringData{
					RAM:        1100,
					CPU:        35,
					Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2300}},
					InTraffic:  150,
					OutTraffic: 150,
				},
			},
			quotaData: testQuotaData{
				cpu:  35,
				ram:  1100,
				disk: 2300,
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 150, outputTraffic: 250},
			monitoringData: cloudprotocol.NodeMonitoringData{
				MonitoringData: cloudprotocol.MonitoringData{
					RAM:        1100,
					CPU:        45,
					Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2300}},
					InTraffic:  150,
					OutTraffic: 250,
				},
			},
			quotaData: testQuotaData{
				cpu:  45,
				ram:  1100,
				disk: 2300,
			},
			alerts: []cloudprotocol.AlertItem{
				prepareSystemAlertItem("cpu", time.Now(), 45),
				prepareSystemAlertItem("outTraffic", time.Now(), 250),
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 350, outputTraffic: 250},
			monitoringData: cloudprotocol.NodeMonitoringData{
				MonitoringData: cloudprotocol.MonitoringData{
					RAM:        2100,
					CPU:        45,
					Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 4300}},
					InTraffic:  350,
					OutTraffic: 250,
				},
			},
			quotaData: testQuotaData{
				cpu:  45,
				ram:  2100,
				disk: 4300,
			},
			alerts: []cloudprotocol.AlertItem{
				prepareSystemAlertItem("cpu", time.Now(), 45),
				prepareSystemAlertItem("ram", time.Now(), 2100),
				prepareSystemAlertItem("generic", time.Now(), 4300),
				prepareSystemAlertItem("inTraffic", time.Now(), 350),
				prepareSystemAlertItem("outTraffic", time.Now(), 250),
			},
		},
	}

	for _, item := range testData {
		trafficMonitoring = item.trafficMonitoring
		systemQuotaData = item.quotaData

		monitor, err := New("node1", config, sender, monitoringSender, &trafficMonitoring)
		if err != nil {
			t.Fatalf("Can't create monitoring instance: %s", err)
		}

		select {
		case monitoringData := <-monitoringSender.monitoringData:
			if !reflect.DeepEqual(monitoringData.MonitoringData, item.monitoringData.MonitoringData) {
				t.Errorf("Incorrect system monitoring data: %v", monitoringData.MonitoringData)
			}

			if len(alerts) != len(item.alerts) {
				t.Fatalf("Incorrect alerts number: %d", len(alerts))
			}

			for i, currentAlert := range alerts {
				if item.alerts[i].Payload != currentAlert.Payload {
					t.Errorf("Incorrect system alert payload: %v", currentAlert.Payload)
				}
			}

		case <-time.After(duration * 2):
			t.Fatal("Monitoring data timeout")
		}

		alerts = nil

		monitor.Close()
	}
}

func TestGetSystemInfo(t *testing.T) {
	duration := 100 * time.Millisecond

	systemVirtualMemory = getSystemRAM
	systemDiskUsage = getSystemDisk

	testData := []struct {
		config             Config
		quotaData          testQuotaData
		expectedSystemInfo SystemInfo
	}{
		{
			config: Config{
				Partitions: []PartitionConfig{{
					Name:  cloudprotocol.GenericPartition,
					Path:  ".",
					Types: []string{"storage"},
				}},
				SendPeriod: aostypes.Duration{Duration: duration},
				PollPeriod: aostypes.Duration{Duration: duration},
			},
			quotaData: testQuotaData{
				cores:     2,
				totalRAM:  1000,
				totalDisk: 1000,
			},
			expectedSystemInfo: SystemInfo{
				NumCPUs:  2,
				TotalRAM: 1000,
				Partitions: []cloudprotocol.PartitionInfo{
					{
						Name:      cloudprotocol.GenericPartition,
						Types:     []string{"storage"},
						TotalSize: 1000,
					},
				},
			},
		},
		{
			config: Config{
				Partitions: []PartitionConfig{{
					Name:  cloudprotocol.StatesPartition,
					Path:  ".",
					Types: []string{"state"},
				}},
				SendPeriod: aostypes.Duration{Duration: duration},
				PollPeriod: aostypes.Duration{Duration: duration},
			},
			quotaData: testQuotaData{
				cores:     3,
				totalRAM:  2000,
				totalDisk: 4000,
			},
			expectedSystemInfo: SystemInfo{
				NumCPUs:  3,
				TotalRAM: 2000,
				Partitions: []cloudprotocol.PartitionInfo{
					{
						Name:      cloudprotocol.StatesPartition,
						Types:     []string{"state"},
						TotalSize: 4000,
					},
				},
			},
		},
	}

	sender := &testAlertsSender{}
	monitoringSender := &testMonitoringSender{}

	defer func() {
		cpuCount = runtime.NumCPU()
	}()

	for _, item := range testData {
		systemQuotaData = item.quotaData
		cpuCount = systemQuotaData.cores

		monitor, err := New("node1", item.config, sender, monitoringSender, &testTrafficMonitoring{})
		if err != nil {
			t.Errorf("Can't create monitoring instance: %s", err)
		}

		systemInfo := monitor.GetSystemInfo()

		if !reflect.DeepEqual(item.expectedSystemInfo, systemInfo) {
			t.Error("Unexpected system info")
		}
	}
}

func TestInstances(t *testing.T) {
	duration := 100 * time.Millisecond
	instanceTrafficMonitoringData = make(map[string]testTrafficMonitoring)

	var alerts []cloudprotocol.AlertItem

	sender := &testAlertsSender{
		alertCallback: func(alert cloudprotocol.AlertItem) {
			alerts = append(alerts, alert)
		},
	}

	trafficMonitoring := &testTrafficMonitoring{}

	monitoringSender := &testMonitoringSender{
		monitoringData: make(chan cloudprotocol.NodeMonitoringData, 1),
	}

	testHostSystemUsageInstance := newTestHostSystemUsage()

	hostSystemUsageInstance = testHostSystemUsageInstance
	defer func() {
		hostSystemUsageInstance = nil
	}()

	monitor, err := New("node1", Config{
		SendPeriod: aostypes.Duration{Duration: duration},
		PollPeriod: aostypes.Duration{Duration: duration},
		Partitions: []PartitionConfig{{Name: cloudprotocol.GenericPartition, Path: "."}},
	},
		sender, monitoringSender, trafficMonitoring)
	if err != nil {
		t.Fatalf("Can't create monitoring instance: %s", err)
	}
	defer monitor.Close()

	getUserFSQuotaUsage = testUserFSQuotaUsage

	testData := []testAlertData{
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 150, outputTraffic: 150},
			quotaData: testQuotaData{
				cpu:  35,
				ram:  1100,
				disk: 2300,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject1",
					Instance:  1,
				},
				UID: 5000,
				GID: 5000,
				AlertRules: &aostypes.AlertRules{
					CPU: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     30,
						High:    40,
					},
					RAM: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     1000,
						High:    2000,
					},
					UsedDisks: []aostypes.PartitionAlertRuleParam{
						{
							AlertRuleParam: aostypes.AlertRuleParam{
								Timeout: aostypes.Duration{},
								Low:     2000,
								High:    3000,
							},
							Name: cloudprotocol.ServicesPartition,
						},
					},
					InTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
					OutTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 250, outputTraffic: 150},
			quotaData: testQuotaData{
				cpu:  45,
				ram:  2100,
				disk: 2300,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				},
				UID: 3000,
				GID: 5000,
				AlertRules: &aostypes.AlertRules{
					CPU: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     30,
						High:    40,
					},
					RAM: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     1000,
						High:    2000,
					},
					UsedDisks: []aostypes.PartitionAlertRuleParam{
						{
							AlertRuleParam: aostypes.AlertRuleParam{
								Timeout: aostypes.Duration{},
								Low:     2000,
								High:    3000,
							},
							Name: cloudprotocol.LayersPartition,
						},
					},
					InTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
					OutTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.LayersPartition, Path: "."}},
			},
			alerts: []cloudprotocol.AlertItem{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				}, "ram", time.Now(), 2100),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				}, "inTraffic", time.Now(), 250),
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 150, outputTraffic: 250},
			quotaData: testQuotaData{
				cpu:  45,
				ram:  1100,
				disk: 2300,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				},
				UID: 2000,
				GID: 5000,
				AlertRules: &aostypes.AlertRules{
					CPU: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     30,
						High:    40,
					},
					RAM: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     1000,
						High:    2000,
					},
					UsedDisks: []aostypes.PartitionAlertRuleParam{
						{
							AlertRuleParam: aostypes.AlertRuleParam{
								Timeout: aostypes.Duration{},
								Low:     2000,
								High:    3000,
							},
							Name: cloudprotocol.ServicesPartition,
						},
					},
					InTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
					OutTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			alerts: []cloudprotocol.AlertItem{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "ram", time.Now(), 2200),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "cpu", time.Now(), uint64(math.Round((45+45)/float64(numCPU)))),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "outTraffic", time.Now(), 250),
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{inputTraffic: 150, outputTraffic: 250},
			quotaData: testQuotaData{
				cpu:  45,
				ram:  1100,
				disk: 2300,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				},
				UID: 2000,
				GID: 5000,
				AlertRules: &aostypes.AlertRules{
					CPU: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     30,
						High:    40,
					},
					RAM: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     1000,
						High:    2000,
					},
					UsedDisks: []aostypes.PartitionAlertRuleParam{
						{
							AlertRuleParam: aostypes.AlertRuleParam{
								Timeout: aostypes.Duration{},
								Low:     2000,
								High:    3000,
							},
							Name: cloudprotocol.StatesPartition,
						},
					},
					InTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
					OutTraffic: &aostypes.AlertRuleParam{
						Timeout: aostypes.Duration{},
						Low:     100,
						High:    200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.StatesPartition, Path: "."}},
			},
			alerts: []cloudprotocol.AlertItem{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "ram", time.Now(), 2200),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "cpu", time.Now(), uint64(math.Round((45+45)/float64(numCPU)))),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "outTraffic", time.Now(), 250),
			},
		},
	}

	var expectedInstanceAlertCount int

	monitoringInstances := []cloudprotocol.InstanceMonitoringData{
		{
			InstanceIdent: aostypes.InstanceIdent{
				ServiceID: "service1",
				SubjectID: "subject1",
				Instance:  1,
			},
			MonitoringData: cloudprotocol.MonitoringData{
				RAM:        1100,
				CPU:        uint64(math.Round(35 / float64(numCPU))),
				Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.ServicesPartition, UsedSize: 2300}},
				InTraffic:  150,
				OutTraffic: 150,
			},
		},
		{
			InstanceIdent: aostypes.InstanceIdent{
				ServiceID: "service2",
				SubjectID: "subject1",
				Instance:  1,
			},
			MonitoringData: cloudprotocol.MonitoringData{
				RAM:        2100,
				CPU:        uint64(math.Round(45 / float64(numCPU))),
				Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.LayersPartition, UsedSize: 2300}},
				InTraffic:  250,
				OutTraffic: 150,
			},
		},
		{
			InstanceIdent: aostypes.InstanceIdent{
				ServiceID: "service1",
				SubjectID: "subject2",
				Instance:  2,
			},
			MonitoringData: cloudprotocol.MonitoringData{
				RAM:        2200,
				CPU:        uint64(math.Round((45 + 45) / float64(numCPU))),
				Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.ServicesPartition, UsedSize: 2300}},
				InTraffic:  150,
				OutTraffic: 250,
			},
		},
		{
			InstanceIdent: aostypes.InstanceIdent{
				ServiceID: "service1",
				SubjectID: "subject2",
				Instance:  2,
			},
			MonitoringData: cloudprotocol.MonitoringData{
				RAM:        2200,
				CPU:        uint64(math.Round((45 + 45) / float64(numCPU))),
				Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.StatesPartition, UsedSize: 2300}},
				InTraffic:  150,
				OutTraffic: 250,
			},
		},
	}

	for i, instance := range monitoringInstances {
		instanceID := fmt.Sprintf("instance%d", i)

		testHostSystemUsageInstance.instances[instanceID] = struct{ cpu, ram uint64 }{instance.CPU, instance.RAM}
	}

	for i, item := range testData {
		processesData = append(processesData, &testProcessData{
			uid:       int32(item.monitoringConfig.UID),
			quotaData: item.quotaData,
		})

		instanceID := fmt.Sprintf("instance%d", i)

		if err := monitor.StartInstanceMonitor(instanceID, item.monitoringConfig); err != nil {
			t.Fatalf("Can't start monitoring instance: %s", err)
		}

		instanceTrafficMonitoringData[instanceID] = item.trafficMonitoring
		expectedInstanceAlertCount += len(item.alerts)
	}

	defer func() {
		processesData = nil
	}()

	select {
	case monitoringData := <-monitoringSender.monitoringData:
		if len(monitoringData.ServiceInstances) != len(monitoringInstances) {
			t.Fatalf("Incorrect instance monitoring count: %d", len(monitoringData.ServiceInstances))
		}

	monitoringLoop:
		for _, receivedMonitoring := range monitoringData.ServiceInstances {
			for _, expectedMonitoring := range monitoringInstances {
				if reflect.DeepEqual(expectedMonitoring, receivedMonitoring) {
					continue monitoringLoop
				}
			}

			t.Errorf("Unexpected monitoring data: %v", receivedMonitoring)
		}

	case <-time.After(duration * 2):
		t.Fatal("Monitoring data timeout")
	}

	if len(alerts) != expectedInstanceAlertCount {
		t.Fatalf("Incorrect alerts number: %d", len(alerts))
	}

	for i, item := range testData {
	alertLoop:
		for _, expectedAlert := range item.alerts {
			for _, receivedAlert := range alerts {
				if expectedAlert.Payload == receivedAlert.Payload {
					continue alertLoop
				}
			}

			t.Error("Incorrect system alert payload")
		}

		instanceID := fmt.Sprintf("instance%d", i)

		delete(testHostSystemUsageInstance.instances, instanceID)

		if err := monitor.StopInstanceMonitor(instanceID); err != nil {
			t.Fatalf("Can't stop monitoring instance: %s", err)
		}
	}

	// this select is used to make sure that the monitoring of the instances has been stopped
	// and monitoring data is not received on them
	select {
	case monitoringData := <-monitoringSender.monitoringData:
		if len(monitoringData.ServiceInstances) != 0 {
			t.Fatalf("Incorrect instance monitoring count: %d", len(monitoringData.ServiceInstances))
		}

	case <-time.After(duration * 2):
		t.Fatal("Monitoring data timeout")
	}
}

/***********************************************************************************************************************
 * Interfaces
 **********************************************************************************************************************/

func (sender *testAlertsSender) SendAlert(alert cloudprotocol.AlertItem) {
	sender.alertCallback(alert)
}

func (sender *testMonitoringSender) SendMonitoringData(monitoringData cloudprotocol.NodeMonitoringData) {
	sender.monitoringData <- monitoringData
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func (trafficMonitoring *testTrafficMonitoring) GetSystemTraffic() (inputTraffic, outputTraffic uint64, err error) {
	return trafficMonitoring.inputTraffic, trafficMonitoring.outputTraffic, nil
}

func (trafficMonitoring *testTrafficMonitoring) GetInstanceTraffic(instanceID string) (
	inputTraffic, outputTraffic uint64, err error,
) {
	trafficMonitoringData, ok := instanceTrafficMonitoringData[instanceID]
	if !ok {
		return 0, 0, aoserrors.New("incorrect instance ID")
	}

	return trafficMonitoringData.inputTraffic, trafficMonitoringData.outputTraffic, nil
}

func getSystemCPUPercent(interval time.Duration, percpu bool) (persent []float64, err error) {
	return []float64{systemQuotaData.cpu}, nil
}

func getSystemRAM() (virtualMemory *mem.VirtualMemoryStat, err error) {
	return &mem.VirtualMemoryStat{Used: systemQuotaData.ram, Total: systemQuotaData.totalRAM}, nil
}

func getSystemDisk(path string) (diskUsage *disk.UsageStat, err error) {
	return &disk.UsageStat{Used: systemQuotaData.disk, Total: systemQuotaData.totalDisk}, nil
}

func (p *testProcessData) Uids() ([]int32, error) {
	return []int32{p.uid}, nil
}

func (p *testProcessData) CPUPercent() (float64, error) {
	return p.quotaData.cpu, nil
}

func (p *testProcessData) MemoryInfo() (*process.MemoryInfoStat, error) {
	return &process.MemoryInfoStat{
		RSS: p.quotaData.ram,
	}, nil
}

func testUserFSQuotaUsage(path string, uid, gid uint32) (byteUsed uint64, err error) {
	for _, quota := range processesData {
		if quota.uid == int32(uid) {
			return quota.quotaData.disk, nil
		}
	}

	return 0, aoserrors.New("incorrect uid")
}

func newTestHostSystemUsage() *testHostSystemUsage {
	return &testHostSystemUsage{instances: map[string]struct{ cpu, ram uint64 }{}}
}

func (host *testHostSystemUsage) CacheSystemInfos() {
}

func (host *testHostSystemUsage) FillSystemInfo(instanceID string, instance *instanceMonitoring) error {
	data, ok := host.instances[instanceID]
	if !ok {
		return aoserrors.Errorf("instance %s not found", instanceID)
	}

	instance.monitoringData.CPU = data.cpu
	instance.monitoringData.RAM = data.ram

	return nil
}
