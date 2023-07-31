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
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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

	tmpDir, err = os.MkdirTemp("", "aos_")
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

	disk, err := newTestDisk(mountPoint, 1, nil)
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

	disk, err := newTestDisk(mountPoint, 1, nil)
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

func TestOutdatedItems(t *testing.T) {
	type testFile struct {
		name      string
		size      uint64
		timestamp time.Time
	}

	timestamp := time.Now()

	// Total outdated files 768 Kb, clean part ~900 Kb, available 123 Kb
	outdatedFiles := []testFile{
		{name: "file1.data", size: 128 * kilobyte, timestamp: timestamp.Add(-1 * time.Hour)},
		{name: "file2.data", size: 32 * kilobyte, timestamp: timestamp.Add(-6 * time.Hour)},
		{name: "file3.data", size: 64 * kilobyte, timestamp: timestamp.Add(-5 * time.Hour)},
		{name: "file4.data", size: 256 * kilobyte, timestamp: timestamp.Add(-4 * time.Hour)},
		{name: "file5.data", size: 32 * kilobyte, timestamp: timestamp.Add(-2 * time.Hour)},
		{name: "file6.data", size: 256 * kilobyte, timestamp: timestamp.Add(-3 * time.Hour)},
	}

	mountPoint := filepath.Join(tmpDir, "test")

	disk, err := newTestDisk(mountPoint, 1, func(mountPoint string) error {
		for _, outdatedFile := range outdatedFiles {
			file, err := os.Create(filepath.Join(mountPoint, outdatedFile.name))
			if err != nil {
				return aoserrors.Wrap(err)
			}

			buffer := bytes.NewBuffer(make([]byte, outdatedFile.size))

			if _, err := io.Copy(file, buffer); err != nil {
				return aoserrors.Wrap(err)
			}

			file.Close()
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Can't create test disk: %v", err)
	}

	defer disk.close()

	removedFiles := make([]string, 0)

	allocator, err := spaceallocator.New(mountPoint, 100, func(id string) error {
		removedFiles = append(removedFiles, id)

		if err := os.RemoveAll(filepath.Join(mountPoint, id)); err != nil {
			return aoserrors.Wrap(err)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocator.Close()

	for _, outdatedFile := range outdatedFiles {
		if err := allocator.AddOutdatedItem(
			outdatedFile.name, outdatedFile.size, outdatedFile.timestamp); err != nil {
			t.Fatalf("Can't add outdated item: %s", err)
		}
	}

	// Need more 132 - 256 ~
	space, err := allocator.AllocateSpace(256 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	// file2.dat, file3.dat, file4.dat should be removed: 32 + 64 + 256
	if !reflect.DeepEqual(removedFiles, []string{outdatedFiles[1].name, outdatedFiles[2].name, outdatedFiles[3].name}) {
		t.Errorf("Wrong files removed: %v", removedFiles)
	}

	if err = space.Release(); err != nil {
		t.Errorf("Can't release space: %v", err)
	}

	// Check allocate more than can be freed

	removedFiles = nil

	if _, err := allocator.AllocateSpace(1024 * kilobyte); !errors.Is(err, spaceallocator.ErrNoSpace) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	if removedFiles != nil {
		t.Errorf("Wrong files removed: %v", removedFiles)
	}

	// Check restore outdated files

	for _, id := range []string{outdatedFiles[0].name, outdatedFiles[4].name, outdatedFiles[5].name} {
		allocator.RestoreOutdatedItem(id)
	}

	if _, err := allocator.AllocateSpace(512 * kilobyte); !errors.Is(err, spaceallocator.ErrNoSpace) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	// Check add outdated files without remover

	allocatorWORemoved, err := spaceallocator.New(mountPoint, 0, nil)
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocatorWORemoved.Close()

	if err := allocatorWORemoved.AddOutdatedItem("testItem", 1024, time.Now()); err == nil {
		t.Error("Error should be returned")
	}
}

func TestPartLimit(t *testing.T) {
	type testFile struct {
		name string
		size uint64
	}

	// Total exists files 224 Kb, total size ~992 Kb, size limit 50% 496 Kb
	existFiles := []testFile{
		{name: "file1.data", size: 96 * kilobyte},
		{name: "file2.data", size: 32 * kilobyte},
		{name: "file3.data", size: 64 * kilobyte},
	}

	mountPoint := filepath.Join(tmpDir, "test")

	disk, err := newTestDisk(mountPoint, 1, func(mountPoint string) error {
		for _, existFile := range existFiles {
			file, err := os.Create(filepath.Join(mountPoint, existFile.name))
			if err != nil {
				return aoserrors.Wrap(err)
			}

			buffer := bytes.NewBuffer(make([]byte, existFile.size))

			if _, err := io.Copy(file, buffer); err != nil {
				return aoserrors.Wrap(err)
			}

			file.Close()
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Can't create test disk: %v", err)
	}

	defer disk.close()

	removedFiles := make([]string, 0)

	allocator, err := spaceallocator.New(mountPoint, 50, func(id string) error {
		removedFiles = append(removedFiles, id)

		if err := os.RemoveAll(filepath.Join(mountPoint, id)); err != nil {
			return aoserrors.Wrap(err)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Can't create allocator: %v", err)
	}
	defer allocator.Close()

	// 496 Kb - 224 Kb = 272 Kb available

	space1, err := allocator.AllocateSpace(256 * kilobyte)
	if err != nil {
		t.Fatalf("Can't allocate space: %v", err)
	}

	// More should fail due to partition limit

	if _, err := allocator.AllocateSpace(128 * kilobyte); !errors.Is(err, spaceallocator.ErrNoSpace) {
		t.Errorf("Wrong allocator error: %v", err)
	}

	// Test free space

	allocator.FreeSpace(128 * kilobyte)

	space2, err := allocator.AllocateSpace(128 * kilobyte)
	if err != nil {
		t.Errorf("Can't allocate space: %v", err)
	}

	if err := space2.Release(); err != nil {
		t.Errorf("Can't release space: %v", err)
	}

	if err := space1.Accept(); err != nil {
		t.Errorf("Can't accept space: %v", err)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func newTestDisk(
	mountPoint string, sizeMB uint64, contentCreator func(mountPoint string) error,
) (disk *testDisk, err error) {
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

	if err = testtools.CreateFilePartition(file.Name(), "ext4", sizeMB, contentCreator, false); err != nil {
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
