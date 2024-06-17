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

	"github.com/aoscloud/aos_common/aostypes"
	"github.com/aoscloud/aos_common/api/cloudprotocol"
	pb "github.com/aoscloud/aos_common/api/servicemanager"
	"github.com/aoscloud/aos_common/utils/pbconvert"
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

func TestInstanceFilterToPB(t *testing.T) {
	type testFilter struct {
		expectedIdent *pb.InstanceIdent
		filter        cloudprotocol.InstanceFilter
	}

	testData := []testFilter{
		{
			expectedIdent: &pb.InstanceIdent{ServiceId: "s1", SubjectId: "subj1", Instance: 1},
			filter:        cloudprotocol.NewInstanceFilter("s1", "subj1", 1),
		},
		{
			expectedIdent: &pb.InstanceIdent{ServiceId: "s1", SubjectId: "", Instance: 1},
			filter:        cloudprotocol.NewInstanceFilter("s1", "", 1),
		},
		{
			expectedIdent: &pb.InstanceIdent{ServiceId: "s1", SubjectId: "subj1", Instance: -1},
			filter:        cloudprotocol.NewInstanceFilter("s1", "subj1", -1),
		},
		{
			expectedIdent: &pb.InstanceIdent{ServiceId: "s1", SubjectId: "", Instance: -1},
			filter:        cloudprotocol.NewInstanceFilter("s1", "", -1),
		},
		{
			expectedIdent: &pb.InstanceIdent{ServiceId: "", SubjectId: "", Instance: -1},
			filter:        cloudprotocol.NewInstanceFilter("", "", -1),
		},
	}

	for _, testItem := range testData {
		instance := pbconvert.InstanceFilterToPB(testItem.filter)

		if !proto.Equal(instance, testItem.expectedIdent) {
			t.Error("Incorrect instance")
		}
	}
}

func TestInstanceIdentToPB(t *testing.T) {
	expectdInstance := &pb.InstanceIdent{ServiceId: "s1", SubjectId: "subj1", Instance: 2}

	pbInstance := pbconvert.InstanceIdentToPB(
		aostypes.InstanceIdent{ServiceID: "s1", SubjectID: "subj1", Instance: 2})

	if !proto.Equal(pbInstance, expectdInstance) {
		t.Error("Incorrect instance")
	}
}

func TestInstanceIdentFromPB(t *testing.T) {
	expectedInstance := aostypes.InstanceIdent{ServiceID: "s1", SubjectID: "subj1", Instance: 2}

	receivedInstance := pbconvert.NewInstanceIdentFromPB(
		&pb.InstanceIdent{ServiceId: "s1", SubjectId: "subj1", Instance: 2})

	if expectedInstance != receivedInstance {
		t.Error("Incorrect instance")
	}
}

func TestNetworkParametersToPB(t *testing.T) {
	expectedNetwork := &pb.NetworkParameters{
		Ip:         "172.18.0.1",
		Subnet:     "172.18.0.0/16",
		DnsServers: []string{"10.10.0.1"},
		Rules: []*pb.FirewallRule{
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
		&pb.NetworkParameters{
			Ip:         "172.18.0.1",
			Subnet:     "172.18.0.0/16",
			DnsServers: []string{"10.10.0.1"},
			Rules: []*pb.FirewallRule{
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
