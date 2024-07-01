// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2023 Renesas Electronics Corporation.
// Copyright (C) 2023 EPAM Systems, Inc.
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

package xentop_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/aosedge/aos_common/utils/xentop"
	log "github.com/sirupsen/logrus"
)

type testShellCommand struct{}

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

func TestGetSystemInfo(t *testing.T) {
	xentop.ExecContext = newTestShellCommander

	expectedSystemInfos := map[string]xentop.SystemInfo{
		"Domain-0": {
			Name:           "Domain-0",
			State:          "-----r",
			CPUTime:        487,
			CPUFraction:    0.0,
			Memory:         737280,
			MemoryFraction: 8.9,
			VirtualCpus:    4,
			SSID:           2,
		},
		"DomD": {
			Name:              "DomD",
			State:             "--b---",
			CPUTime:           180,
			CPUFraction:       0.0,
			Memory:            2084104,
			MaxMemory:         2098176,
			MaxMemoryFraction: 25.4,
			MemoryFraction:    25.2,
			VirtualCpus:       4,
			SSID:              14,
		},
	}

	systemInfos, err := xentop.GetSystemInfos()
	if err != nil {
		t.Fatalf("Can't get system infos: %v", err)
	}

	if !reflect.DeepEqual(systemInfos, expectedSystemInfos) {
		t.Error("Unexpected system infos")
	}
}

func (cmd testShellCommand) CombinedOutput() ([]byte, error) {
	header := "   NAME  STATE   CPU(sec) CPU(%)     MEM(k) MEM(%)  MAXMEM(k) MAXMEM(%) VCPUS NETS NETTX(k) NETRX(k) " +
		"VBDS   VBD_OO   VBD_RD   VBD_WR  VBD_RSECT  VBD_WSECT SSID"

	dom0 := "Domain-0 -----r        487    0.0     737280    8.9   no limit       n/a     4    0        0        0 " +
		"0        0        0        0          0          0    2"

	domD := "   DomD --b---        180    0.0    2084104   25.2    2098176      25.4     4    0        0        0 " +
		"0        0        0        0          0          0   14"

	return []byte(fmt.Sprintf("%s\n%s\n%s", header, dom0, domD)), nil
}

func newTestShellCommander(name string, arg ...string) xentop.ShellCommand {
	return testShellCommand{}
}
