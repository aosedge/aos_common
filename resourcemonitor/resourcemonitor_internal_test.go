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
	systemAlerts   []SystemQuotaAlert
	instanceAlerts []InstanceQuotaAlert
}

type testMonitoringSender struct {
	monitoringData chan cloudprotocol.NodeMonitoringData
}

type trafficMonitoringData struct {
	inTraffic, outTraffic uint64
}

type testTrafficMonitoring struct {
	systemTraffic   trafficMonitoringData
	instanceTraffic map[string]trafficMonitoringData
}

type testUsageData struct {
	cpu       float64
	ram       uint64
	totalRAM  uint64
	disk      uint64
	totalDisk uint64
}

type testData struct {
	systemAlerts      []SystemQuotaAlert
	instanceAlerts    []InstanceQuotaAlert
	monitoringData    cloudprotocol.NodeMonitoringData
	trafficMonitoring testTrafficMonitoring
	usageData         testUsageData
	monitoringConfig  ResourceMonitorParams
	instanceID        string
}

type testInstancesUsage struct {
	instances map[string]testUsageData
}

type testNodeInfoProvider struct {
	nodeInfo cloudprotocol.NodeInfo
}

type testNodeConfigProvider struct {
	nodeConfig cloudprotocol.NodeConfig
}

/***********************************************************************************************************************
 * Variable
 **********************************************************************************************************************/

var (
	systemUsageData testUsageData
	processesData   map[int32]testUsageData
	numCPU          = runtime.NumCPU()
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
	type AlertItem struct {
		time   time.Time
		value  uint64
		status string
	}

	var (
		sourceValue    uint64
		receivedAlerts []AlertItem
	)

	alert := createAlertProcessor(
		"Test",
		&sourceValue,
		func(time time.Time, value uint64, status string) {
			receivedAlerts = append(receivedAlerts, AlertItem{time, value, status})
		},
		aostypes.AlertRuleParam{
			Timeout: aostypes.Duration{Duration: 3 * time.Second},
			Low:     80,
			High:    90,
		})

	values := []uint64{
		50, 91, 79, 92, 93, 94, 95, 94, 79, 91, 92, 93, 94, 32, 91, 92, 93, 94, 95, 96, 85, 79, 77, 76, 75, 74, 73, 72,
	}

	currentTime := time.Time{}

	expectedAlerts := []AlertItem{
		{currentTime.Add(6 * time.Second), 95, AlertStatusRaise},
		{currentTime.Add(9 * time.Second), 91, AlertStatusContinue},
		{currentTime.Add(12 * time.Second), 94, AlertStatusContinue},
		{currentTime.Add(15 * time.Second), 92, AlertStatusContinue},
		{currentTime.Add(18 * time.Second), 95, AlertStatusContinue},
		{currentTime.Add(21 * time.Second), 79, AlertStatusContinue},
		{currentTime.Add(24 * time.Second), 75, AlertStatusFall},
	}

	for _, value := range values {
		sourceValue = value

		alert.checkAlertDetection(currentTime)

		currentTime = currentTime.Add(time.Second)
	}

	if !reflect.DeepEqual(receivedAlerts, expectedAlerts) {
		t.Errorf("Incorrect alerts received: %v, expected: %v", receivedAlerts, expectedAlerts)
	}
}

func TestSystemAlerts(t *testing.T) {
	duration := 100 * time.Millisecond

	nodeInfoProvider := &testNodeInfoProvider{
		nodeInfo: cloudprotocol.NodeInfo{
			NodeID:     "testNode",
			NodeType:   "testNode",
			Partitions: []cloudprotocol.PartitionInfo{{Name: cloudprotocol.GenericPartition, Path: "."}},
		},
	}

	nodeConfigProvider := &testNodeConfigProvider{
		nodeConfig: cloudprotocol.NodeConfig{
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
		},
	}

	monitoringSender := &testMonitoringSender{
		monitoringData: make(chan cloudprotocol.NodeMonitoringData),
	}

	systemCPUPercent = getSystemCPUPercent
	systemVirtualMemory = getSystemRAM
	systemDiskUsage = getSystemDisk

	config := Config{PollPeriod: aostypes.Duration{Duration: duration}}

	testData := []testData{
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{inTraffic: 150, outTraffic: 150},
			},
			monitoringData: cloudprotocol.NodeMonitoringData{
				MonitoringData: cloudprotocol.MonitoringData{
					RAM:        1100,
					CPU:        35,
					Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2300}},
					InTraffic:  150,
					OutTraffic: 150,
				},
			},
			usageData: testUsageData{
				cpu:  35,
				ram:  1100,
				disk: 2300,
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{inTraffic: 150, outTraffic: 250},
			},
			monitoringData: cloudprotocol.NodeMonitoringData{
				MonitoringData: cloudprotocol.MonitoringData{
					RAM:        1100,
					CPU:        45,
					Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2300}},
					InTraffic:  150,
					OutTraffic: 250,
				},
			},
			usageData: testUsageData{
				cpu:  45,
				ram:  1100,
				disk: 2300,
			},
			systemAlerts: []SystemQuotaAlert{
				prepareSystemAlertItem("cpu", time.Time{}, 45, "raise"),
				prepareSystemAlertItem("outTraffic", time.Time{}, 250, "raise"),
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{inTraffic: 350, outTraffic: 250},
			},
			monitoringData: cloudprotocol.NodeMonitoringData{
				MonitoringData: cloudprotocol.MonitoringData{
					RAM:        2100,
					CPU:        45,
					Disk:       []cloudprotocol.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 4300}},
					InTraffic:  350,
					OutTraffic: 250,
				},
			},
			usageData: testUsageData{
				cpu:  45,
				ram:  2100,
				disk: 4300,
			},
			systemAlerts: []SystemQuotaAlert{
				prepareSystemAlertItem("cpu", time.Time{}, 45, "raise"),
				prepareSystemAlertItem("ram", time.Time{}, 2100, "raise"),
				prepareSystemAlertItem("generic", time.Time{}, 4300, "raise"),
				prepareSystemAlertItem("inTraffic", time.Time{}, 350, "raise"),
				prepareSystemAlertItem("outTraffic", time.Time{}, 250, "raise"),
			},
		},
	}

	for _, item := range testData {
		alertSender := &testAlertsSender{}
		trafficMonitoring := item.trafficMonitoring
		systemUsageData = item.usageData

		monitor, err := New(config, nodeInfoProvider, nodeConfigProvider,
			&trafficMonitoring, alertSender, monitoringSender)
		if err != nil {
			t.Fatalf("Can't create monitoring instance: %s", err)
		}

		select {
		case monitoringData := <-monitoringSender.monitoringData:
			if !reflect.DeepEqual(monitoringData.MonitoringData, item.monitoringData.MonitoringData) {
				t.Errorf("Incorrect system monitoring data: %v", monitoringData.MonitoringData)
			}

			if !reflect.DeepEqual(alertSender.systemAlerts, item.systemAlerts) {
				t.Errorf("Incorrect system alerts: %v", alertSender.systemAlerts)
			}

		case <-time.After(duration * 2):
			t.Fatal("Monitoring data timeout")
		}

		monitor.Close()
	}
}

func TestInstances(t *testing.T) {
	duration := 100 * time.Millisecond

	nodeInfoProvider := &testNodeInfoProvider{
		nodeInfo: cloudprotocol.NodeInfo{
			NodeID:   "testNode",
			NodeType: "testNode",
		},
	}
	nodeConfigProvider := &testNodeConfigProvider{}
	trafficMonitoring := &testTrafficMonitoring{
		instanceTraffic: make(map[string]trafficMonitoringData),
	}
	alertSender := &testAlertsSender{}
	monitoringSender := &testMonitoringSender{
		monitoringData: make(chan cloudprotocol.NodeMonitoringData, 1),
	}

	testInstancesUsage := newTestInstancesUsage()

	instanceUsage = testInstancesUsage
	defer func() {
		instanceUsage = nil
	}()

	monitor, err := New(Config{
		PollPeriod: aostypes.Duration{Duration: duration},
	},
		nodeInfoProvider, nodeConfigProvider, trafficMonitoring, alertSender, monitoringSender)
	if err != nil {
		t.Fatalf("Can't create monitoring instance: %s", err)
	}
	defer monitor.Close()

	getUserFSQuotaUsage = testUserFSQuotaUsage

	testData := []testData{
		{
			instanceID: "instance0",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance0": {inTraffic: 150, outTraffic: 150},
				},
			},
			usageData: testUsageData{
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
			monitoringData: cloudprotocol.NodeMonitoringData{
				ServiceInstances: []cloudprotocol.InstanceMonitoringData{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: cloudprotocol.MonitoringData{
							RAM: 1100,
							CPU: 35,
							Disk: []cloudprotocol.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 2300},
							},
							InTraffic:  150,
							OutTraffic: 150,
						},
					},
				},
			},
		},
		{
			instanceID: "instance1",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance1": {inTraffic: 250, outTraffic: 150},
				},
			},
			usageData: testUsageData{
				cpu:  25,
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
			monitoringData: cloudprotocol.NodeMonitoringData{
				ServiceInstances: []cloudprotocol.InstanceMonitoringData{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service2",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: cloudprotocol.MonitoringData{
							RAM: 2100,
							CPU: 25,
							Disk: []cloudprotocol.PartitionUsage{
								{Name: cloudprotocol.LayersPartition, UsedSize: 2300},
							},
							InTraffic:  250,
							OutTraffic: 150,
						},
					},
				},
			},
			instanceAlerts: []InstanceQuotaAlert{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				}, "ram", time.Time{}, 2100, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				}, "inTraffic", time.Time{}, 250, "raise"),
			},
		},
		{
			instanceID: "instance2",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance2": {inTraffic: 150, outTraffic: 250},
				},
			},
			usageData: testUsageData{
				cpu:  90,
				ram:  2200,
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
			monitoringData: cloudprotocol.NodeMonitoringData{
				ServiceInstances: []cloudprotocol.InstanceMonitoringData{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject2",
							Instance:  2,
						},
						MonitoringData: cloudprotocol.MonitoringData{
							RAM: 2200,
							CPU: 90,
							Disk: []cloudprotocol.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 2300},
							},
							InTraffic:  150,
							OutTraffic: 250,
						},
					},
				},
			},
			instanceAlerts: []InstanceQuotaAlert{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "ram", time.Time{}, 2200, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "cpu", time.Time{}, 90, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "outTraffic", time.Time{}, 250, "raise"),
			},
		},
		{
			instanceID: "instance3",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance3": {inTraffic: 150, outTraffic: 250},
				},
			},
			usageData: testUsageData{
				cpu:  90,
				ram:  2200,
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
			monitoringData: cloudprotocol.NodeMonitoringData{
				ServiceInstances: []cloudprotocol.InstanceMonitoringData{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject2",
							Instance:  2,
						},
						MonitoringData: cloudprotocol.MonitoringData{
							RAM: 2200,
							CPU: 90,
							Disk: []cloudprotocol.PartitionUsage{
								{Name: cloudprotocol.StatesPartition, UsedSize: 2300},
							},
							InTraffic:  150,
							OutTraffic: 250,
						},
					},
				},
			},
			instanceAlerts: []InstanceQuotaAlert{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "ram", time.Time{}, 2200, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "cpu", time.Time{}, 90, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "outTraffic", time.Time{}, 250, "raise"),
			},
		},
	}

	var expectedInstanceAlertCount int

	processesData = make(map[int32]testUsageData)

	for _, item := range testData {
		testInstancesUsage.instances[item.instanceID] = testUsageData{
			cpu: item.usageData.cpu, ram: item.usageData.ram,
		}

		processesData[int32(item.monitoringConfig.UID)] = item.usageData
		trafficMonitoring.instanceTraffic[item.instanceID] = item.trafficMonitoring.instanceTraffic[item.instanceID]

		if err := monitor.StartInstanceMonitor(item.instanceID, item.monitoringConfig); err != nil {
			t.Fatalf("Can't start monitoring instance: %s", err)
		}

		expectedInstanceAlertCount += len(item.instanceAlerts)
	}

	select {
	case monitoringData := <-monitoringSender.monitoringData:
		if len(monitoringData.ServiceInstances) != len(testData) {
			t.Fatalf("Incorrect instance monitoring count: %d", len(monitoringData.ServiceInstances))
		}

	monitoringLoop:
		for _, receivedMonitoring := range monitoringData.ServiceInstances {
			for _, item := range testData {
				if reflect.DeepEqual(item.monitoringData.ServiceInstances[0], receivedMonitoring) {
					continue monitoringLoop
				}
			}

			t.Errorf("Wrong monitoring data: %v", receivedMonitoring)
		}

	case <-time.After(duration * 2):
		t.Fatal("Monitoring data timeout")
	}

	if len(alertSender.instanceAlerts) != expectedInstanceAlertCount {
		t.Fatalf("Incorrect alerts number: %d", len(alertSender.instanceAlerts))
	}

	for i, item := range testData {
	alertLoop:
		for _, expectedAlert := range item.instanceAlerts {
			for _, receivedAlert := range alertSender.instanceAlerts {
				if expectedAlert == receivedAlert {
					continue alertLoop
				}
			}

			t.Error("Incorrect instance alert payload: ", expectedAlert)
		}

		instanceID := fmt.Sprintf("instance%d", i)

		delete(testInstancesUsage.instances, instanceID)

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

func (sender *testAlertsSender) SendSystemQuotaAlert(alert SystemQuotaAlert) {
	alert.Timestamp = time.Time{}
	sender.systemAlerts = append(sender.systemAlerts, alert)
}

func (sender *testAlertsSender) SendInstanceQuotaAlert(alert InstanceQuotaAlert) {
	alert.Timestamp = time.Time{}
	sender.instanceAlerts = append(sender.instanceAlerts, alert)
}

func (sender *testMonitoringSender) SendMonitoringData(monitoringData cloudprotocol.NodeMonitoringData) {
	sender.monitoringData <- monitoringData
}

func (provider *testNodeInfoProvider) GetNodeInfo() (cloudprotocol.NodeInfo, error) {
	return provider.nodeInfo, nil
}

func (provider *testNodeInfoProvider) NodeInfoChangedChannel() <-chan cloudprotocol.NodeInfo {
	return nil
}

func (provider *testNodeConfigProvider) GetNodeConfig() (cloudprotocol.NodeConfig, error) {
	return provider.nodeConfig, nil
}

func (provider *testNodeConfigProvider) NodeConfigChangedChannel() <-chan cloudprotocol.NodeConfig {
	return nil
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func (trafficMonitoring *testTrafficMonitoring) GetSystemTraffic() (inputTraffic, outputTraffic uint64, err error) {
	return trafficMonitoring.systemTraffic.inTraffic, trafficMonitoring.systemTraffic.outTraffic, nil
}

func (trafficMonitoring *testTrafficMonitoring) GetInstanceTraffic(instanceID string) (
	inputTraffic, outputTraffic uint64, err error,
) {
	trafficMonitoringData, ok := trafficMonitoring.instanceTraffic[instanceID]
	if !ok {
		return 0, 0, aoserrors.New("incorrect instance ID")
	}

	return trafficMonitoringData.inTraffic, trafficMonitoringData.outTraffic, nil
}

func getSystemCPUPercent(interval time.Duration, percpu bool) (percent []float64, err error) {
	return []float64{systemUsageData.cpu * float64(cpuCount)}, nil
}

func getSystemRAM() (virtualMemory *mem.VirtualMemoryStat, err error) {
	return &mem.VirtualMemoryStat{Used: systemUsageData.ram, Total: systemUsageData.totalRAM}, nil
}

func getSystemDisk(path string) (diskUsage *disk.UsageStat, err error) {
	return &disk.UsageStat{Used: systemUsageData.disk, Total: systemUsageData.totalDisk}, nil
}

func testUserFSQuotaUsage(path string, uid, gid uint32) (byteUsed uint64, err error) {
	usageData, ok := processesData[int32(uid)]
	if !ok {
		return 0, aoserrors.New("UID not found")
	}

	return usageData.disk, nil
}

func newTestInstancesUsage() *testInstancesUsage {
	return &testInstancesUsage{instances: map[string]testUsageData{}}
}

func (host *testInstancesUsage) CacheSystemInfos() {
}

func (host *testInstancesUsage) FillSystemInfo(instanceID string, instance *instanceMonitoring) error {
	data, ok := host.instances[instanceID]
	if !ok {
		return aoserrors.Errorf("instance %s not found", instanceID)
	}

	instance.monitoringData.CPU = uint64(math.Round(data.cpu))
	instance.monitoringData.RAM = data.ram

	return nil
}
