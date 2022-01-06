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

package fs_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/aoscloud/aos_common/fs"
	"github.com/aoscloud/aos_common/utils/testtools"
)

/*******************************************************************************
 * Vars
 ******************************************************************************/

var (
	disk       *testtools.TestDisk
	mountPoint string
	tmpDir     string
)

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

func TestMountUmountContinued(t *testing.T) {
	for i := 0; i < 50; i++ {
		mountUmount(t)
	}
}

func TestMountAlreadyMounted(t *testing.T) {
	for _, part := range disk.Partitions {
		if err := fs.Mount(part.Device, mountPoint, part.Type, 0, ""); err != nil {
			t.Fatalf("Can't mount partition: %s", err)
		}

		if err := fs.Mount(part.Device, mountPoint, part.Type, 0, ""); err != nil {
			t.Fatalf("Can't mount partition: %s", err)
		}

		if err := fs.Umount(mountPoint); err != nil {
			t.Fatalf("Can't umount partition: %s", err)
		}
	}
}

/*******************************************************************************
 * Private
 ******************************************************************************/

func mountUmount(t *testing.T) {
	t.Helper()

	for _, part := range disk.Partitions {
		if err := fs.Mount(part.Device, mountPoint, part.Type, 0, ""); err != nil {
			t.Fatalf("Can't mount partition: %s", err)
		}

		if err := fs.Umount(mountPoint); err != nil {
			t.Fatalf("Can't umount partition: %s", err)
		}
	}
}
