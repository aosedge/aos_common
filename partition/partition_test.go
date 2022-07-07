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

package partition_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/aoscloud/aos_common/partition"
	"github.com/aoscloud/aos_common/utils/testtools"
)

/*******************************************************************************
 * Vars
 ******************************************************************************/

var disk *testtools.TestDisk

var tmpDir string

var mountPoint string

/*******************************************************************************
 * Init
 ******************************************************************************/

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: false,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		FullTimestamp:    true,
	})
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stdout)
}

/*******************************************************************************
 * Main
 ******************************************************************************/

func TestMain(m *testing.M) {
	var err error

	if tmpDir, err = ioutil.TempDir("", "um_"); err != nil {
		log.Fatalf("Error creating tmp dir: %s", err)
	}

	mountPoint = path.Join(tmpDir, "mount")

	if disk, err = testtools.NewTestDisk(
		path.Join(tmpDir, "testdisk.img"),
		[]testtools.PartDesc{
			{Type: "vfat", Label: "efi", Size: 16},
			{Type: "ext4", Label: "platform", Size: 32},
		}); err != nil {
		log.Fatalf("Can't create test disk: %s", err)
	}

	ret := m.Run()

	if err = disk.Close(); err != nil {
		log.Fatalf("Can't close test disk: %s", err)
	}

	if err = os.RemoveAll(tmpDir); err != nil {
		log.Fatalf("Error removing tmp dir: %s", err)
	}

	os.Exit(ret)
}

/*******************************************************************************
 * Tests
 ******************************************************************************/

func TestGetPartitionNum(t *testing.T) {
	for i, part := range disk.Partitions {
		partNum, err := partition.GetPartitionNum(part.Device)
		if err != nil {
			t.Fatalf("Can't get partition num %s", part.Device)
		}

		if i+1 != partNum {
			t.Fatalf("Wrong partition num %d", i)
		}
	}
}

func TestGetPartitionDevice(t *testing.T) {
	for _, part := range disk.Partitions {
		device, err := partition.GetParentDevice(part.Device)
		if err != nil {
			t.Fatalf("Can't get partition num %s", part.Device)
		}

		if disk.Device != device {
			t.Fatalf("Error, wrong device. Expected %s, got %s", disk.Device, device)
		}
	}
}

func TestGetPartitionInfo(t *testing.T) {
	for _, part := range disk.Partitions {
		info, err := partition.GetPartInfo(part.Device)
		if err != nil {
			t.Fatalf("Can't get partition info: %s", err)
		}

		if part.Device != info.Device {
			t.Errorf("Wrong device: %s", info.Device)
		}

		if part.Type != info.FSType {
			t.Errorf("Wrong partition type: %s", info.FSType)
		}

		if part.Label != info.Label {
			t.Errorf("Wrong partition label: %s", info.Label)
		}

		if part.PartUUID != info.PartUUID {
			t.Errorf("Wrong partition UUID: %s", info.PartUUID)
		}
	}
}
