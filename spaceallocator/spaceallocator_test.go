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

package spaceallocator_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/spaceallocator"
	"github.com/aoscloud/aos_common/utils/testtools"
	log "github.com/sirupsen/logrus"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const kilobyte = uint64(1 << 10)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

type testDisk struct {
	mountPoint string
	fileName   string
}

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var tmpDir string

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
 * Main
 **********************************************************************************************************************/

func TestMain(m *testing.M) {
	var err error

	tmpDir, err = ioutil.TempDir("", "aos_")
	if err != nil {
		log.Fatalf("Error creating tmp dir: %v", err)
	}

	ret := m.Run()

	if err = os.RemoveAll(tmpDir); err != nil {
		log.Errorf("Can't remove tmp dir: %v", err)
	}

	os.Exit(ret)
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestAllocate(t *testing.T) {
	mountPoint := filepath.Join(tmpDir, "test")

	disk, err := newTestDisk(mountPoint, 1)
	if err != nil {
		t.Fatalf("Can't create test disk: %v", err)
	}

	defer disk.close()

	allocator, err := spaceallocator.New(mountPoint, 0, nil)
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocator.Close()

	space1, err := allocator.AllocateSpace(256 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	space2, err := allocator.AllocateSpace(512 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	// Allocate space3 should fail due to not enough space: 256 + 512 + 512 > 1024

	if _, err := allocator.AllocateSpace(512 * kilobyte); !errors.Is(err, spaceallocator.ErrNoSpace) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	if err = space2.Release(); err != nil {
		t.Errorf("Can't release space: %v", err)
	}

	// Now space3 should succeed as space2 released: 256 + 512 <= 1024

	space3, err := allocator.AllocateSpace(512 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	// Check allocator free space

	if _, err := allocator.AllocateSpace(256 * kilobyte); !errors.Is(err, spaceallocator.ErrNoSpace) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	allocator.FreeSpace(256 * kilobyte)

	space4, err := allocator.AllocateSpace(256 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	// Finalize spaces

	if err = space4.Release(); err != nil {
		t.Errorf("Can't release space: %v", err)
	}

	if err = space3.Accept(); err != nil {
		t.Errorf("Can't accept space: %v", err)
	}

	if err = space1.Accept(); err != nil {
		t.Errorf("Can't accept space: %v", err)
	}

	// Check accept and release already accepted space

	if err = space1.Accept(); !errors.Is(err, spaceallocator.ErrNoAllocation) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	if err = space1.Release(); !errors.Is(err, spaceallocator.ErrNoAllocation) {
		t.Errorf("Wrong allocator error: %v", err)
	}
}

func TestMultipleAllocators(t *testing.T) {
	mountPoint := filepath.Join(tmpDir, "test")

	disk, err := newTestDisk(mountPoint, 1)
	if err != nil {
		t.Fatalf("Can't create test disk: %v", err)
	}

	defer disk.close()

	allocator1, err := spaceallocator.New(mountPoint, 0, nil)
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocator1.Close()

	allocator2, err := spaceallocator.New(mountPoint, 0, nil)
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocator2.Close()

	allocator3, err := spaceallocator.New(mountPoint, 0, nil)
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocator3.Close()

	space1, err := allocator1.AllocateSpace(256 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	space2, err := allocator2.AllocateSpace(512 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	// Allocate space3 should fail due to not enough space: 256 + 512 + 512 > 1024

	if _, err := allocator3.AllocateSpace(512 * kilobyte); !errors.Is(err, spaceallocator.ErrNoSpace) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	if err = space2.Release(); err != nil {
		t.Errorf("Can't release space: %v", err)
	}

	// Now space3 should succeed as space2 released: 256 + 512 <= 1024

	space3, err := allocator3.AllocateSpace(512 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	if err = space3.Accept(); err != nil {
		t.Errorf("Can't accept space: %v", err)
	}

	if err = space1.Accept(); err != nil {
		t.Errorf("Can't accept space: %v", err)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func newTestDisk(mountPoint string, sizeMB uint64) (disk *testDisk, err error) {
	file, err := os.CreateTemp(tmpDir, "*.ext4")
	if err != nil {
		return nil, aoserrors.Wrap(err)
	}

	file.Close()

	defer func() {
		if err != nil {
			os.RemoveAll(file.Name())
		}
	}()

	if err = os.MkdirAll(mountPoint, 0o755); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	defer func() {
		if err != nil {
			os.RemoveAll(mountPoint)
		}
	}()

	if err = testtools.CreateFilePartition(file.Name(), "ext4", sizeMB, nil, false); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if output, err := exec.Command("mount", file.Name(), mountPoint).CombinedOutput(); err != nil {
		return nil, aoserrors.Errorf("%v (%s)", err, (string(output)))
	}

	return &testDisk{
		mountPoint: mountPoint,
		fileName:   file.Name(),
	}, nil
}

func (disk *testDisk) close() error {
	if output, err := exec.Command("umount", disk.mountPoint).CombinedOutput(); err != nil {
		return aoserrors.Errorf("%v (%s)", err, (string(output)))
	}

	if err := os.RemoveAll(disk.mountPoint); err != nil {
		return aoserrors.Wrap(err)
	}

	if err := os.Remove(disk.fileName); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}
