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

package cloudprotocol_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/aosedge/aos_common/api/cloudprotocol"
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
 * Tests
 **********************************************************************************************************************/

func TestNewInstanceFilter(t *testing.T) {
	type filterConstructData struct {
		serviceID string
		subjectID string
		instance  int64
	}

	type testFilter struct {
		constructData  filterConstructData
		expectedFilter cloudprotocol.InstanceFilter
	}

	var (
		subject   = "subj1"
		instance  = uint64(2)
		serviceID = "s1"
	)

	testData := []testFilter{
		{
			expectedFilter: cloudprotocol.InstanceFilter{
				ServiceID: &serviceID, SubjectID: &subject, Instance: &instance,
			},
			constructData: filterConstructData{serviceID: "s1", subjectID: "subj1", instance: 2},
		},
		{
			expectedFilter: cloudprotocol.InstanceFilter{ServiceID: &serviceID, SubjectID: nil, Instance: &instance},
			constructData:  filterConstructData{serviceID: "s1", subjectID: "", instance: 2},
		},
		{
			expectedFilter: cloudprotocol.InstanceFilter{ServiceID: &serviceID, SubjectID: &subject, Instance: nil},
			constructData:  filterConstructData{serviceID: "s1", subjectID: "subj1", instance: -1},
		},
		{
			expectedFilter: cloudprotocol.InstanceFilter{ServiceID: &serviceID},
			constructData:  filterConstructData{serviceID: "s1", subjectID: "", instance: -1},
		},
		{
			expectedFilter: cloudprotocol.InstanceFilter{},
			constructData:  filterConstructData{serviceID: "", subjectID: "", instance: -1},
		},
	}

	for _, testItem := range testData {
		filter := cloudprotocol.NewInstanceFilter(
			testItem.constructData.serviceID, testItem.constructData.subjectID, testItem.constructData.instance)

		if !reflect.DeepEqual(filter, testItem.expectedFilter) {
			t.Error("Incorrect filter")
		}
	}
}

func TestNodeInfoAttrs(t *testing.T) {
	nodeInfo := cloudprotocol.NodeInfo{
		Attrs: map[string]interface{}{
			cloudprotocol.NodeAttrMainNode:      "",
			cloudprotocol.NodeAttrAosComponents: "iam, sm, um",
			cloudprotocol.NodeAttrRunners:       "runc, crun",
		},
	}

	if !nodeInfo.IsMainNode() {
		t.Error("Incorrect main node")
	}

	aosComponents, err := nodeInfo.GetAosComponents()
	if err != nil {
		t.Fatalf("Failed to get AOS components: %v", err)
	}

	if !reflect.DeepEqual(aosComponents, []string{
		cloudprotocol.AosComponentIAM, cloudprotocol.AosComponentSM, cloudprotocol.AosComponentUM,
	}) {
		t.Error("Incorrect AOS components")
	}

	runners, err := nodeInfo.GetNodeRunners()
	if err != nil {
		t.Fatalf("Failed to get runners: %v", err)
	}

	if !reflect.DeepEqual(runners, []string{"runc", "crun"}) {
		t.Error("Incorrect runners")
	}
}
