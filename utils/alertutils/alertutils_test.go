// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2021 Renesas Electronics Corporation.
// Copyright (C) 2021 EPAM Systems, Inc.
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

package alertutils_test

import (
	"testing"
	"time"

	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"
	"github.com/aosedge/aos_common/utils/alertutils"
)

func TestSameAlerts(t *testing.T) {
	alerts := []struct {
		alert1 interface{}
		alert2 interface{}
		result bool
	}{
		{
			alert1: cloudprotocol.SystemAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemError},
				NodeID:    "mainNode",
				Message:   "system crash",
			},
			alert2: cloudprotocol.SystemAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemError},
				NodeID:    "mainNode",
				Message:   "system crash",
			},
			result: true,
		},

		{
			alert1: cloudprotocol.CoreAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagAosCore},
				NodeID:    "mainNode",
				Message:   "system crash",
			},
			alert2: cloudprotocol.CoreAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagAosCore},
				NodeID:    "mainNode",
				Message:   "system crash",
			},
			result: true,
		},

		{
			alert1: cloudprotocol.DownloadAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagDownloadProgress},
				TargetID:  "mainTarget",
				Message:   "system crash",
			},
			alert2: cloudprotocol.DownloadAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemError},
				TargetID:  "TargetID",
				Message:   "download crash",
			},
			result: false,
		},

		{
			alert1: cloudprotocol.SystemQuotaAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				NodeID:    "mainNode",
				Status:    "ok",
			},
			alert2: cloudprotocol.SystemQuotaAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				NodeID:    "mainNode",
				Status:    "ok",
			},
			result: true,
		},

		{
			alert1: cloudprotocol.InstanceQuotaAlert{
				AlertItem:     cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "mainService", SubjectID: "mainSubject", Instance: 42},
				Status:        "ok",
			},
			alert2: cloudprotocol.InstanceQuotaAlert{
				AlertItem:     cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "mainService", SubjectID: "mainSubject", Instance: 42},
				Status:        "error",
			},
			result: false,
		},

		{
			alert1: cloudprotocol.DeviceAllocateAlert{
				AlertItem:     cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "mainService", SubjectID: "mainSubject", Instance: 42},
				Message:       "device allocate error",
			},
			alert2: cloudprotocol.DeviceAllocateAlert{
				AlertItem:     cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "mainService", SubjectID: "mainSubject", Instance: 42},
				Message:       "device allocate error",
			},
			result: true,
		},

		{
			alert1: cloudprotocol.ResourceValidateAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				NodeID:    "mainNode",
			},
			alert2: cloudprotocol.ResourceValidateAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				NodeID:    "secondaryNode",
			},
			result: false,
		},

		{
			alert1: cloudprotocol.ServiceInstanceAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				Message:   "service alert",
			},
			alert2: cloudprotocol.ServiceInstanceAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				Message:   "system alert",
			},
			result: false,
		},
	}

	for _, testCase := range alerts {
		if alertutils.AlertsPayloadEqual(testCase.alert1, testCase.alert2) != testCase.result {
			t.Fatalf("Alerts comparison wrong, alert1=%v, alert2=%v, expected=%v",
				testCase.alert1, testCase.alert2, testCase.result)
		}
	}
}

func TestDifferentAlerts(t *testing.T) {
	systemAlert := cloudprotocol.SystemAlert{
		AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemError},
		NodeID:    "mainNode",
		Message:   "system crash",
	}

	alerts := []struct {
		alert1 interface{}
		alert2 interface{}
	}{
		{
			alert1: systemAlert,
			alert2: cloudprotocol.CoreAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagAosCore},
				NodeID:    "mainNode",
				Message:   "system crash",
			},
		},

		{
			alert1: cloudprotocol.CoreAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagAosCore},
				NodeID:    "mainNode",
				Message:   "system crash",
			},
			alert2: systemAlert,
		},

		{
			alert1: cloudprotocol.DownloadAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagDownloadProgress},
				TargetID:  "mainTarget",
				Message:   "system crash",
			},
			alert2: systemAlert,
		},

		{
			alert1: cloudprotocol.SystemQuotaAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				NodeID:    "mainNode",
				Status:    "ok",
			},
			alert2: systemAlert,
		},

		{
			alert1: cloudprotocol.InstanceQuotaAlert{
				AlertItem:     cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "mainService", SubjectID: "mainSubject", Instance: 42},
				Status:        "ok",
			},
			alert2: systemAlert,
		},

		{
			alert1: cloudprotocol.DeviceAllocateAlert{
				AlertItem:     cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "mainService", SubjectID: "mainSubject", Instance: 42},
				Message:       "device allocate error",
			},
			alert2: systemAlert,
		},

		{
			alert1: cloudprotocol.ResourceValidateAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				NodeID:    "mainNode",
			},
			alert2: systemAlert,
		},

		{
			alert1: cloudprotocol.ServiceInstanceAlert{
				AlertItem: cloudprotocol.AlertItem{Timestamp: time.Now(), Tag: cloudprotocol.AlertTagSystemQuota},
				Message:   "service alert",
			},
			alert2: systemAlert,
		},
	}

	for _, testCase := range alerts {
		if alertutils.AlertsPayloadEqual(testCase.alert1, testCase.alert2) {
			t.Fatalf("Alerts comparison wrong, alert1=%v, alert2=%v", testCase.alert1, testCase.alert2)
		}
	}
}

func TestCompareNotAlerts(t *testing.T) {
	type Tmp struct {
		data string
	}

	item1 := Tmp{data: "tmp"}
	item2 := Tmp{data: "tmp"}

	if alertutils.AlertsPayloadEqual(item1, item2) {
		t.Fatal("Alerts comparison wrong")
	}
}
