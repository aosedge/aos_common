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

package pbconvert_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"
	pbcommon "github.com/aosedge/aos_common/api/common"
	pbiam "github.com/aosedge/aos_common/api/iamanager"
	pbsm "github.com/aosedge/aos_common/api/servicemanager"

	"github.com/aosedge/aos_common/utils/pbconvert"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
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
 * Tests
 **********************************************************************************************************************/

func TestInstanceFilter(t *testing.T) {
	type testFilter struct {
		pbFilter  *pbsm.InstanceFilter
		aosFilter cloudprotocol.InstanceFilter
	}

	testData := []testFilter{
		{
			pbFilter:  &pbsm.InstanceFilter{ServiceId: "s1", SubjectId: "subj1", Instance: 1},
			aosFilter: cloudprotocol.NewInstanceFilter("s1", "subj1", 1),
		},
		{
			pbFilter:  &pbsm.InstanceFilter{ServiceId: "s1", SubjectId: "", Instance: 1},
			aosFilter: cloudprotocol.NewInstanceFilter("s1", "", 1),
		},
		{
			pbFilter:  &pbsm.InstanceFilter{ServiceId: "s1", SubjectId: "subj1", Instance: -1},
			aosFilter: cloudprotocol.NewInstanceFilter("s1", "subj1", -1),
		},
		{
			pbFilter:  &pbsm.InstanceFilter{ServiceId: "s1", SubjectId: "", Instance: -1},
			aosFilter: cloudprotocol.NewInstanceFilter("s1", "", -1),
		},
		{
			pbFilter:  &pbsm.InstanceFilter{ServiceId: "", SubjectId: "", Instance: -1},
			aosFilter: cloudprotocol.NewInstanceFilter("", "", -1),
		},
	}

	for _, testItem := range testData {
		pbFilter := pbconvert.InstanceFilterToPB(testItem.aosFilter)

		if !proto.Equal(pbFilter, testItem.pbFilter) {
			t.Error("Incorrect instance")
		}

		aosFilter := pbconvert.InstanceFilterFromPB(testItem.pbFilter)

		if !reflect.DeepEqual(aosFilter, testItem.aosFilter) {
			t.Error("Incorrect instance")
		}
	}
}

func TestInstanceIdentToPB(t *testing.T) {
	expectedInstance := &pbcommon.InstanceIdent{ServiceId: "s1", SubjectId: "subj1", Instance: 2}

	pbInstance := pbconvert.InstanceIdentToPB(
		aostypes.InstanceIdent{ServiceID: "s1", SubjectID: "subj1", Instance: 2})

	if !proto.Equal(pbInstance, expectedInstance) {
		t.Error("Incorrect instance")
	}
}

func TestInstanceIdentFromPB(t *testing.T) {
	expectedInstance := aostypes.InstanceIdent{ServiceID: "s1", SubjectID: "subj1", Instance: 2}

	receivedInstance := pbconvert.InstanceIdentFromPB(
		&pbcommon.InstanceIdent{ServiceId: "s1", SubjectId: "subj1", Instance: 2})

	if expectedInstance != receivedInstance {
		t.Error("Incorrect instance")
	}
}

func TestNetworkParametersToPB(t *testing.T) {
	expectedNetwork := &pbsm.NetworkParameters{
		Ip:         "172.18.0.1",
		Subnet:     "172.18.0.0/16",
		DnsServers: []string{"10.10.0.1"},
		Rules: []*pbsm.FirewallRule{
			{
				Proto:   "tcp",
				DstIp:   "172.19.0.1",
				SrcIp:   "172.18.0.1",
				DstPort: "8080",
			},
		},
	}

	pbNetwork := pbconvert.NetworkParametersToPB(
		aostypes.NetworkParameters{
			IP:         "172.18.0.1",
			Subnet:     "172.18.0.0/16",
			DNSServers: []string{"10.10.0.1"},
			FirewallRules: []aostypes.FirewallRule{
				{
					Proto:   "tcp",
					DstIP:   "172.19.0.1",
					SrcIP:   "172.18.0.1",
					DstPort: "8080",
				},
			},
		})

	if !proto.Equal(pbNetwork, expectedNetwork) {
		t.Error("Incorrect network parameters")
	}
}

func TestNetworkParametersFromPB(t *testing.T) {
	expectedNetwork := aostypes.NetworkParameters{
		IP:         "172.18.0.1",
		Subnet:     "172.18.0.0/16",
		DNSServers: []string{"10.10.0.1"},
		FirewallRules: []aostypes.FirewallRule{
			{
				Proto:   "tcp",
				DstIP:   "172.19.0.1",
				SrcIP:   "172.18.0.1",
				DstPort: "8080",
			},
		},
	}

	receivedNetwork := pbconvert.NewNetworkParametersFromPB(
		&pbsm.NetworkParameters{
			Ip:         "172.18.0.1",
			Subnet:     "172.18.0.0/16",
			DnsServers: []string{"10.10.0.1"},
			Rules: []*pbsm.FirewallRule{
				{
					Proto:   "tcp",
					DstIp:   "172.19.0.1",
					SrcIp:   "172.18.0.1",
					DstPort: "8080",
				},
			},
		})

	if !reflect.DeepEqual(expectedNetwork, receivedNetwork) {
		t.Error("Incorrect network parameters")
	}
}

func TestErrorInfoToPB(t *testing.T) {
	expectedErrorInfo := &pbcommon.ErrorInfo{AosCode: 42, ExitCode: 5, Message: "error"}

	pbErrorInfo := pbconvert.ErrorInfoToPB(
		&cloudprotocol.ErrorInfo{AosCode: 42, ExitCode: 5, Message: "error"})

	if !proto.Equal(pbErrorInfo, expectedErrorInfo) {
		t.Error("Incorrect instance")
	}
}

func TestErrorInfoFromPB(t *testing.T) {
	expectedErrorInfo := &cloudprotocol.ErrorInfo{AosCode: 42, ExitCode: 5, Message: "error"}

	receivedErrorInfo := pbconvert.ErrorInfoFromPB(
		&pbcommon.ErrorInfo{AosCode: 42, ExitCode: 5, Message: "error"})

	if *expectedErrorInfo != *receivedErrorInfo {
		t.Error("Incorrect error info")
	}
}

func TestNodeInfoFromPB(t *testing.T) {
	expectedNodeInfo := cloudprotocol.NodeInfo{
		NodeID:   "node1",
		NodeType: "type1",
		Name:     "name1",
		Status:   "status1",
		CPUs: []cloudprotocol.CPUInfo{
			{
				ModelName:  "model1",
				NumCores:   1,
				NumThreads: 2,
				Arch:       "arch1",
				ArchFamily: "family1",
				MaxDMIPs:   3,
			},
			{
				ModelName:  "model1",
				NumCores:   1,
				NumThreads: 2,
				Arch:       "arch1",
				ArchFamily: "family1",
				MaxDMIPs:   3,
			},
		},
		OSType:   "os1",
		MaxDMIPs: 4,
		TotalRAM: 5,
		Attrs: map[string]interface{}{
			"attr1": "value1",
			"attr2": "value2",
		},
		Partitions: []cloudprotocol.PartitionInfo{{
			Name:      "part1",
			Types:     []string{"type1", "type2"},
			TotalSize: 6,
		}, {
			Name:      "part2",
			Types:     []string{"type3", "type4"},
			TotalSize: 12,
		}},
		ErrorInfo: &cloudprotocol.ErrorInfo{AosCode: 42, ExitCode: 5, Message: "error"},
	}

	receivedNodInfo := pbconvert.NewNodeInfoFromPB(
		&pbiam.NodeInfo{
			NodeId:   "node1",
			NodeType: "type1",
			Name:     "name1",
			Status:   "status1",
			Cpus: []*pbiam.CPUInfo{
				{ModelName: "model1", NumCores: 1, NumThreads: 2, Arch: "arch1", ArchFamily: "family1", MaxDmips: 3},
				{ModelName: "model1", NumCores: 1, NumThreads: 2, Arch: "arch1", ArchFamily: "family1", MaxDmips: 3},
			},
			OsType:   "os1",
			MaxDmips: 4,
			TotalRam: 5,
			Attrs: []*pbiam.NodeAttribute{
				{Name: "attr1", Value: "value1"},
				{Name: "attr2", Value: "value2"},
			},
			Partitions: []*pbiam.PartitionInfo{
				{Name: "part1", Types: []string{"type1", "type2"}, TotalSize: 6},
				{Name: "part2", Types: []string{"type3", "type4"}, TotalSize: 12},
			},
			Error: &pbcommon.ErrorInfo{AosCode: 42, ExitCode: 5, Message: "error"},
		})

	if !reflect.DeepEqual(expectedNodeInfo, receivedNodInfo) {
		t.Error("Incorrect node info")
	}
}
