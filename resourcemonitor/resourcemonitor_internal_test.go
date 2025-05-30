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

	"github.com/aosedge/aos_common/aoserrors"
	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"
	"github.com/aosedge/aos_common/utils/alertutils"
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
	alerts []interface{}
}

type trafficMonitoringData struct {
	download, upload uint64
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
	alerts            []interface{}
	monitoringData    aostypes.NodeMonitoring
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

	alert := createAlertProcessorPoints(
		"Test",
		&sourceValue,
		func(time time.Time, value uint64, status string) {
			receivedAlerts = append(receivedAlerts, AlertItem{time, value, status})
		},
		aostypes.AlertRulePoints{
			MinTimeout:   aostypes.Duration{Duration: 3 * time.Second},
			MinThreshold: 80,
			MaxThreshold: 90,
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
			NodeID:   "testNode",
			NodeType: "testNode",
			MaxDMIPs: 10000,
			TotalRAM: 10000,
			Partitions: []cloudprotocol.PartitionInfo{
				{Name: cloudprotocol.GenericPartition, Path: ".", TotalSize: 10000},
			},
		},
	}

	nodeConfigProvider := &testNodeConfigProvider{
		nodeConfig: cloudprotocol.NodeConfig{
			AlertRules: &aostypes.AlertRules{
				CPU: &aostypes.AlertRulePercents{
					MinTimeout:   aostypes.Duration{},
					MinThreshold: 80,
					MaxThreshold: 90,
				},
				RAM: &aostypes.AlertRulePercents{
					MinTimeout:   aostypes.Duration{},
					MinThreshold: 80,
					MaxThreshold: 90,
				},
				Partitions: []aostypes.PartitionAlertRule{
					{
						AlertRulePercents: aostypes.AlertRulePercents{
							MinTimeout:   aostypes.Duration{},
							MinThreshold: 80,
							MaxThreshold: 90,
						},
						Name: cloudprotocol.GenericPartition,
					},
				},
				Download: &aostypes.AlertRulePoints{
					MinTimeout:   aostypes.Duration{},
					MinThreshold: 100,
					MaxThreshold: 200,
				},
				Upload: &aostypes.AlertRulePoints{
					MinTimeout:   aostypes.Duration{},
					MinThreshold: 100,
					MaxThreshold: 200,
				},
			},
		},
	}

	systemCPUPercent = getSystemCPUPercent
	systemVirtualMemory = getSystemRAM
	systemDiskUsage = getSystemDisk

	config := Config{PollPeriod: aostypes.Duration{Duration: duration}}

	nodeID := nodeInfoProvider.nodeInfo.NodeID

	testData := []testData{
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{download: 150, upload: 150},
			},
			monitoringData: aostypes.NodeMonitoring{
				NodeID: "testNode",
				NodeData: aostypes.MonitoringData{
					CPU:        3500,
					RAM:        1100,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2300}},
					Download:   150,
					Upload:     150,
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
				systemTraffic: trafficMonitoringData{download: 150, upload: 250},
			},
			monitoringData: aostypes.NodeMonitoring{
				NodeID: "testNode",
				NodeData: aostypes.MonitoringData{
					RAM:        1100,
					CPU:        10000,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2300}},
					Download:   150,
					Upload:     250,
				},
			},
			usageData: testUsageData{
				cpu:  100,
				ram:  1100,
				disk: 2300,
			},
			alerts: []interface{}{
				prepareSystemAlertItem(nodeID, "cpu", time.Time{}, 10000, "raise"),
				prepareSystemAlertItem(nodeID, "upload", time.Time{}, 250, "raise"),
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{download: 350, upload: 250},
			},
			monitoringData: aostypes.NodeMonitoring{
				NodeData: aostypes.MonitoringData{
					CPU:        10000,
					RAM:        10000,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 10000}},
					Download:   350,
					Upload:     250,
				},
			},
			usageData: testUsageData{
				cpu:  100,
				ram:  10000,
				disk: 10000,
			},
			alerts: []interface{}{
				prepareSystemAlertItem(nodeID, "cpu", time.Time{}, 10000, "raise"),
				prepareSystemAlertItem(nodeID, "ram", time.Time{}, 10000, "raise"),
				prepareSystemAlertItem(nodeID, "generic", time.Time{}, 10000, "raise"),
				prepareSystemAlertItem(nodeID, "download", time.Time{}, 350, "raise"),
				prepareSystemAlertItem(nodeID, "upload", time.Time{}, 250, "raise"),
			},
		},
	}

	for i, item := range testData {
		t.Logf("Test case: %d", i)

		alertSender := &testAlertsSender{}
		trafficMonitoring := item.trafficMonitoring
		systemUsageData = item.usageData

		monitor, err := New(config, nodeInfoProvider, nodeConfigProvider,
			&trafficMonitoring, alertSender)
		if err != nil {
			t.Fatalf("Can't create monitoring instance: %s", err)
		}

		select {
		case monitoringData := <-monitor.GetNodeMonitoringChannel():
			monitoringData.NodeData.Timestamp = time.Time{}

			if !reflect.DeepEqual(monitoringData.NodeData, item.monitoringData.NodeData) {
				t.Errorf("Incorrect system monitoring data: %v", monitoringData.NodeData)
			}

			if !AlertSlicesEqual(alertSender.alerts, item.alerts) {
				t.Errorf("Incorrect system alerts: %v", alertSender.alerts)
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
			MaxDMIPs: 10000,
			TotalRAM: 10000,
			Partitions: []cloudprotocol.PartitionInfo{
				{Name: cloudprotocol.ServicesPartition, Path: ".", TotalSize: 10000},
				{Name: cloudprotocol.LayersPartition, Path: ".", TotalSize: 10000},
				{Name: cloudprotocol.StatesPartition, Path: ".", TotalSize: 10000},
			},
		},
	}
	nodeConfigProvider := &testNodeConfigProvider{}
	trafficMonitoring := &testTrafficMonitoring{
		instanceTraffic: make(map[string]trafficMonitoringData),
	}
	alertSender := &testAlertsSender{}
	testInstancesUsage := newTestInstancesUsage()

	instanceUsage = testInstancesUsage
	defer func() {
		instanceUsage = nil
	}()

	monitor, err := New(Config{
		PollPeriod: aostypes.Duration{Duration: duration},
	},
		nodeInfoProvider, nodeConfigProvider, trafficMonitoring, alertSender)
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
					"instance0": {download: 150, upload: 150},
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
					CPU: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					RAM: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					Partitions: []aostypes.PartitionAlertRule{
						{
							AlertRulePercents: aostypes.AlertRulePercents{
								MinTimeout:   aostypes.Duration{},
								MinThreshold: 80,
								MaxThreshold: 90,
							},
							Name: cloudprotocol.ServicesPartition,
						},
					},
					Download: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
					Upload: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: aostypes.MonitoringData{
							RAM: 1100,
							CPU: 3500,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 2300},
							},
							Download: 150,
							Upload:   150,
						},
					},
				},
			},
		},
		{
			instanceID: "instance1",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance1": {download: 250, upload: 150},
				},
			},
			usageData: testUsageData{
				cpu:  25,
				ram:  9000,
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
					CPU: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					RAM: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					Partitions: []aostypes.PartitionAlertRule{
						{
							AlertRulePercents: aostypes.AlertRulePercents{
								MinTimeout:   aostypes.Duration{},
								MinThreshold: 80,
								MaxThreshold: 90,
							},
							Name: cloudprotocol.LayersPartition,
						},
					},
					Download: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
					Upload: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.LayersPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service2",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: aostypes.MonitoringData{
							CPU: 2500,
							RAM: 9000,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.LayersPartition, UsedSize: 2300},
							},
							Download: 250,
							Upload:   150,
						},
					},
				},
			},
			alerts: []interface{}{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				}, "ram", time.Time{}, 9000, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service2",
					SubjectID: "subject1",
					Instance:  1,
				}, "download", time.Time{}, 250, "raise"),
			},
		},
		{
			instanceID: "instance2",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance2": {download: 150, upload: 250},
				},
			},
			usageData: testUsageData{
				cpu:  90,
				ram:  9000,
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
					CPU: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					RAM: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					Partitions: []aostypes.PartitionAlertRule{
						{
							AlertRulePercents: aostypes.AlertRulePercents{
								MinTimeout:   aostypes.Duration{},
								MinThreshold: 80,
								MaxThreshold: 90,
							},
							Name: cloudprotocol.ServicesPartition,
						},
					},
					Download: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
					Upload: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject2",
							Instance:  2,
						},
						MonitoringData: aostypes.MonitoringData{
							CPU: 9000,
							RAM: 9000,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 2300},
							},
							Download: 150,
							Upload:   250,
						},
					},
				},
			},
			alerts: []interface{}{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "ram", time.Time{}, 9000, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "cpu", time.Time{}, 9000, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "upload", time.Time{}, 250, "raise"),
			},
		},
		{
			instanceID: "instance3",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance3": {download: 150, upload: 250},
				},
			},
			usageData: testUsageData{
				cpu:  90,
				ram:  9000,
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
					CPU: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					RAM: &aostypes.AlertRulePercents{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 80,
						MaxThreshold: 90,
					},
					Partitions: []aostypes.PartitionAlertRule{
						{
							AlertRulePercents: aostypes.AlertRulePercents{
								MinTimeout:   aostypes.Duration{},
								MinThreshold: 80,
								MaxThreshold: 90,
							},
							Name: cloudprotocol.StatesPartition,
						},
					},
					Download: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
					Upload: &aostypes.AlertRulePoints{
						MinTimeout:   aostypes.Duration{},
						MinThreshold: 100,
						MaxThreshold: 200,
					},
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.StatesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject2",
							Instance:  2,
						},
						MonitoringData: aostypes.MonitoringData{
							CPU: 9000,
							RAM: 9000,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.StatesPartition, UsedSize: 2300},
							},
							Download: 150,
							Upload:   250,
						},
					},
				},
			},
			alerts: []interface{}{
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "ram", time.Time{}, 9000, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "cpu", time.Time{}, 9000, "raise"),
				prepareInstanceAlertItem(aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject2",
					Instance:  2,
				}, "upload", time.Time{}, 250, "raise"),
			},
		},
	}

	var expectedInstanceAlertCount int

	processesData = make(map[int32]testUsageData)

	for i, item := range testData {
		t.Logf("Test case: %d", i)

		testInstancesUsage.instances[item.instanceID] = testUsageData{
			cpu: item.usageData.cpu, ram: item.usageData.ram,
		}

		processesData[int32(item.monitoringConfig.UID)] = item.usageData
		trafficMonitoring.instanceTraffic[item.instanceID] = item.trafficMonitoring.instanceTraffic[item.instanceID]

		if err := monitor.StartInstanceMonitor(item.instanceID, item.monitoringConfig); err != nil {
			t.Fatalf("Can't start monitoring instance: %s", err)
		}

		expectedInstanceAlertCount += len(item.alerts)
	}

	select {
	case monitoringData := <-monitor.GetNodeMonitoringChannel():
		if len(monitoringData.InstancesData) != len(testData) {
			t.Fatalf("Incorrect instance monitoring count: %d", len(monitoringData.InstancesData))
		}

	monitoringLoop:
		for _, receivedMonitoring := range monitoringData.InstancesData {
			for _, item := range testData {
				receivedMonitoring.MonitoringData.Timestamp = time.Time{}

				if reflect.DeepEqual(item.monitoringData.InstancesData[0], receivedMonitoring) {
					continue monitoringLoop
				}
			}

			t.Errorf("Wrong monitoring data: %v", receivedMonitoring)
		}

	case <-time.After(duration * 2):
		t.Fatal("Monitoring data timeout")
	}

	if len(alertSender.alerts) != expectedInstanceAlertCount {
		t.Fatalf("Incorrect alerts number: %d != %d", len(alertSender.alerts), expectedInstanceAlertCount)
	}

	for i, item := range testData {
	alertLoop:
		for _, expectedAlert := range item.alerts {
			for _, receivedAlert := range alertSender.alerts {
				if alertutils.AlertsPayloadEqual(expectedAlert, receivedAlert) {
					continue alertLoop
				}
			}

			t.Errorf("Incorrect instance alert payload: %v", expectedAlert)
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
	case monitoringData := <-monitor.GetNodeMonitoringChannel():
		if len(monitoringData.InstancesData) != 0 {
			t.Fatalf("Incorrect instance monitoring count: %d", len(monitoringData.InstancesData))
		}

	case <-time.After(duration * 2):
		t.Fatal("Monitoring data timeout")
	}
}

func TestSystemAveraging(t *testing.T) {
	duration := 100 * time.Millisecond

	nodeInfoProvider := &testNodeInfoProvider{
		nodeInfo: cloudprotocol.NodeInfo{
			NodeID:     "testNode",
			NodeType:   "testNode",
			Partitions: []cloudprotocol.PartitionInfo{{Name: cloudprotocol.GenericPartition, Path: "."}},
			MaxDMIPs:   10000,
		},
	}

	nodeConfigProvider := &testNodeConfigProvider{}
	alertSender := &testAlertsSender{}
	trafficMonitoring := &testTrafficMonitoring{}

	systemCPUPercent = getSystemCPUPercent
	systemVirtualMemory = getSystemRAM
	systemDiskUsage = getSystemDisk

	config := Config{
		PollPeriod:    aostypes.Duration{Duration: duration},
		AverageWindow: aostypes.Duration{Duration: duration * 3},
	}

	testData := []testData{
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{download: 100, upload: 200},
			},
			usageData: testUsageData{cpu: 10, ram: 1000, disk: 2000},
			monitoringData: aostypes.NodeMonitoring{
				NodeID: "testNode",
				NodeData: aostypes.MonitoringData{
					CPU:        1000,
					RAM:        1000,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 2000}},
					Download:   100,
					Upload:     200,
				},
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{download: 200, upload: 300},
			},
			usageData: testUsageData{cpu: 20, ram: 2000, disk: 4000},
			monitoringData: aostypes.NodeMonitoring{
				NodeID: "testNode",
				NodeData: aostypes.MonitoringData{
					CPU:        1500,
					RAM:        1500,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 3000}},
					Download:   150,
					Upload:     250,
				},
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{download: 300, upload: 400},
			},
			usageData: testUsageData{cpu: 30, ram: 3000, disk: 6000},
			monitoringData: aostypes.NodeMonitoring{
				NodeID: "testNode",
				NodeData: aostypes.MonitoringData{
					CPU:        2000,
					RAM:        2000,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 4000}},
					Download:   200,
					Upload:     300,
				},
			},
		},
		{
			trafficMonitoring: testTrafficMonitoring{
				systemTraffic: trafficMonitoringData{download: 500, upload: 600},
			},
			usageData: testUsageData{cpu: 20, ram: 2000, disk: 4000},
			monitoringData: aostypes.NodeMonitoring{
				NodeID: "testNode",
				NodeData: aostypes.MonitoringData{
					CPU:        2000,
					RAM:        2000,
					Partitions: []aostypes.PartitionUsage{{Name: cloudprotocol.GenericPartition, UsedSize: 4000}},
					Download:   300,
					Upload:     400,
				},
			},
		},
	}

	monitor, err := New(config, nodeInfoProvider, nodeConfigProvider,
		trafficMonitoring, alertSender)
	if err != nil {
		t.Fatalf("Can't create monitoring instance: %s", err)
	}
	defer monitor.Close()

	for _, item := range testData {
		*trafficMonitoring = item.trafficMonitoring
		systemUsageData = item.usageData

		select {
		case <-monitor.GetNodeMonitoringChannel():
			averageData, err := monitor.GetAverageMonitoring()
			if err != nil {
				t.Errorf("Can't get average monitoring data: %s", err)
			}

			averageData.NodeData.Timestamp = time.Time{}

			if !reflect.DeepEqual(averageData.NodeData, item.monitoringData.NodeData) {
				t.Errorf("Incorrect average monitoring data: %v", averageData)
			}

		case <-time.After(duration * 2):
			t.Fatal("Monitoring data timeout")
		}
	}
}

func TestInstanceAveraging(t *testing.T) {
	duration := 100 * time.Millisecond

	nodeInfoProvider := &testNodeInfoProvider{
		nodeInfo: cloudprotocol.NodeInfo{
			NodeID:   "testNode",
			NodeType: "testNode",
			MaxDMIPs: 10000,
		},
	}
	nodeConfigProvider := &testNodeConfigProvider{}
	trafficMonitoring := &testTrafficMonitoring{
		instanceTraffic: make(map[string]trafficMonitoringData),
	}
	alertSender := &testAlertsSender{}
	testInstancesUsage := newTestInstancesUsage()

	instanceUsage = testInstancesUsage
	defer func() {
		instanceUsage = nil
	}()

	monitor, err := New(Config{
		PollPeriod:    aostypes.Duration{Duration: duration},
		AverageWindow: aostypes.Duration{Duration: duration * 3},
	},
		nodeInfoProvider, nodeConfigProvider, trafficMonitoring, alertSender)
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
					"instance0": {download: 100, upload: 100},
				},
			},
			usageData: testUsageData{
				cpu:  10,
				ram:  1000,
				disk: 2000,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject1",
					Instance:  1,
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: aostypes.MonitoringData{
							RAM: 1000,
							CPU: 1000,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 2000},
							},
							Download: 100,
							Upload:   100,
						},
					},
				},
			},
		},
		{
			instanceID: "instance0",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance0": {download: 200, upload: 200},
				},
			},
			usageData: testUsageData{
				cpu:  20,
				ram:  2000,
				disk: 3000,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject1",
					Instance:  1,
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: aostypes.MonitoringData{
							RAM: 1500,
							CPU: 1500,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 2500},
							},
							Download: 150,
							Upload:   150,
						},
					},
				},
			},
		},
		{
			instanceID: "instance0",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance0": {download: 300, upload: 300},
				},
			},
			usageData: testUsageData{
				cpu:  30,
				ram:  3000,
				disk: 4000,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject1",
					Instance:  1,
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: aostypes.MonitoringData{
							RAM: 2000,
							CPU: 2000,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 3000},
							},
							Download: 200,
							Upload:   200,
						},
					},
				},
			},
		},
		{
			instanceID: "instance0",
			trafficMonitoring: testTrafficMonitoring{
				instanceTraffic: map[string]trafficMonitoringData{
					"instance0": {download: 200, upload: 200},
				},
			},
			usageData: testUsageData{
				cpu:  20,
				ram:  2000,
				disk: 3000,
			},
			monitoringConfig: ResourceMonitorParams{
				InstanceIdent: aostypes.InstanceIdent{
					ServiceID: "service1",
					SubjectID: "subject1",
					Instance:  1,
				},
				Partitions: []PartitionParam{{Name: cloudprotocol.ServicesPartition, Path: "."}},
			},
			monitoringData: aostypes.NodeMonitoring{
				InstancesData: []aostypes.InstanceMonitoring{
					{
						InstanceIdent: aostypes.InstanceIdent{
							ServiceID: "service1",
							SubjectID: "subject1",
							Instance:  1,
						},
						MonitoringData: aostypes.MonitoringData{
							RAM: 2000,
							CPU: 2000,
							Partitions: []aostypes.PartitionUsage{
								{Name: cloudprotocol.ServicesPartition, UsedSize: 3000},
							},
							Download: 200,
							Upload:   200,
						},
					},
				},
			},
		},
	}

	processesData = map[int32]testUsageData{}

	if err := monitor.StartInstanceMonitor(testData[0].instanceID, testData[0].monitoringConfig); err != nil {
		t.Fatalf("Can't start monitoring instance: %s", err)
	}

	for _, item := range testData {
		testInstancesUsage.instances[item.instanceID] = testUsageData{cpu: item.usageData.cpu, ram: item.usageData.ram}
		processesData[int32(item.monitoringConfig.UID)] = item.usageData
		trafficMonitoring.instanceTraffic[item.instanceID] = item.trafficMonitoring.instanceTraffic[item.instanceID]

		select {
		case <-monitor.GetNodeMonitoringChannel():
			averageData, err := monitor.GetAverageMonitoring()
			if err != nil {
				t.Errorf("Can't get average monitoring data: %s", err)
			}

			averageData.InstancesData[0].Timestamp = time.Time{}

			if !reflect.DeepEqual(averageData.InstancesData[0].MonitoringData,
				item.monitoringData.InstancesData[0].MonitoringData) {
				t.Errorf("Incorrect average monitoring data: %v", averageData.InstancesData[0].MonitoringData)
			}

		case <-time.After(duration * 2):
			t.Fatal("Monitoring data timeout")
		}
	}
}

/***********************************************************************************************************************
 * Interfaces
 **********************************************************************************************************************/

func (sender *testAlertsSender) SendAlert(alert interface{}) {
	sender.alerts = append(sender.alerts, alert)
}

func (provider *testNodeInfoProvider) GetCurrentNodeInfo() (cloudprotocol.NodeInfo, error) {
	return provider.nodeInfo, nil
}

func (provider *testNodeInfoProvider) NodeInfoChangedChannel() <-chan cloudprotocol.NodeInfo {
	return nil
}

func (provider *testNodeConfigProvider) GetCurrentNodeConfig() (cloudprotocol.NodeConfig, error) {
	return provider.nodeConfig, nil
}

func (provider *testNodeConfigProvider) SubscribeCurrentNodeConfigChange() <-chan cloudprotocol.NodeConfig {
	return nil
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func (trafficMonitoring *testTrafficMonitoring) GetSystemTraffic() (inputTraffic, outputTraffic uint64, err error) {
	return trafficMonitoring.systemTraffic.download, trafficMonitoring.systemTraffic.upload, nil
}

func (trafficMonitoring *testTrafficMonitoring) GetInstanceTraffic(instanceID string) (
	inputTraffic, outputTraffic uint64, err error,
) {
	trafficMonitoringData, ok := trafficMonitoring.instanceTraffic[instanceID]
	if !ok {
		return 0, 0, aoserrors.New("incorrect instance ID")
	}

	return trafficMonitoringData.download, trafficMonitoringData.upload, nil
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

	instance.monitoring.CPU = uint64(math.Round(data.cpu))
	instance.monitoring.RAM = data.ram

	return nil
}

func AlertSlicesEqual(alerts1, alerts2 []interface{}) bool {
	if len(alerts1) != len(alerts2) {
		return false
	}

	for i := range alerts1 {
		if !alertutils.AlertsPayloadEqual(alerts1[i], alerts2[i]) {
			return false
		}
	}

	return true
}
