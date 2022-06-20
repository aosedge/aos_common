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
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/utils/fs"
	"github.com/aoscloud/aos_common/utils/testtools"
)

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var (
	disk       *testtools.TestDisk
	mountPoint string
	tmpDir     string
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
 * Main
 **********************************************************************************************************************/

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

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

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

func TestAvailableSize(t *testing.T) {
	for _, part := range disk.Partitions {
		if err := fs.Mount(part.Device, mountPoint, part.Type, 0, ""); err != nil {
			t.Fatalf("Can't mount partition: %v", err)
		}

		firstDir := filepath.Join(mountPoint, "dir1")

		if err := os.MkdirAll(firstDir, 0o755); err != nil {
			t.Fatalf("Can't create directory: %v", err)
		}

		availableSize, err := fs.GetAvailableSize(firstDir)
		if err != nil {
			t.Errorf("Can't get available size: %v", err)
		}

		expectedSize, err := getAvailableSize(firstDir)
		if err != nil {
			t.Fatalf("Can't get available size: %v", err)
		}

		if availableSize != expectedSize {
			t.Errorf("Wrong available size: %d", availableSize)
		}

		if err := fs.Umount(mountPoint); err != nil {
			t.Fatalf("Can't umount partition: %v", err)
		}
	}
}

func TestGetDirSize(t *testing.T) {
	partition := disk.Partitions[0]

	mountPoint := path.Join(tmpDir, "mount1")

	if err := fs.Mount(partition.Device, mountPoint, partition.Type, 0, ""); err != nil {
		t.Fatalf("Can't mount partition: %s", err)
	}

	defer func() {
		if err := fs.Umount(mountPoint); err != nil {
			t.Fatalf("Can't umount partition: %s", err)
		}
	}()

	firstDir := filepath.Join(mountPoint, "dirSize")

	if _, err := fs.GetDirSize(firstDir); err == nil {
		t.Error("Should be error: not exist directory")
	}

	if err := createDirContent(firstDir, []string{"file0"}); err != nil {
		t.Fatalf("Can't create dir content: %s", err)
	}

	if err := ioutil.WriteFile(path.Join(firstDir, "file0"), []byte("content"), 0o600); err != nil {
		return
	}

	size, err := fs.GetDirSize(firstDir)
	if err != nil {
		t.Fatalf("Can't get directory size: %v", err)
	}

	expectedSize, err := getDirSize(firstDir)
	if err != nil {
		t.Fatalf("Can't get directory size: %v", err)
	}

	if size != expectedSize {
		t.Errorf("Wrong directory size: %d", size)
	}
}

func TestMountPoint(t *testing.T) {
	type testData struct {
		mountPointFirst  string
		mountPointSecond string
		compare          func(string, string) bool
	}

	testsData := []testData{
		{
			mountPointFirst:  path.Join(tmpDir, "mount1"),
			mountPointSecond: path.Join(tmpDir, "mount1"),
			compare:          func(mount1 string, mount2 string) bool { return mount1 == mount2 },
		},
		{
			mountPointFirst:  path.Join(tmpDir, "mount1"),
			mountPointSecond: path.Join(tmpDir, "mount2"),
			compare:          func(mount1 string, mount2 string) bool { return mount1 != mount2 },
		},
	}

	partition := disk.Partitions[0]

	for _, data := range testsData {
		if err := fs.Mount(partition.Device, data.mountPointFirst, partition.Type, 0, ""); err != nil {
			t.Fatalf("Can't mount partition: %s", err)
		}

		if err := fs.Mount(partition.Device, data.mountPointSecond, partition.Type, 0, ""); err != nil {
			t.Fatalf("Can't mount partition: %s", err)
		}

		firstDir := filepath.Join(data.mountPointFirst, "dir1")

		if err := os.MkdirAll(firstDir, 0o755); err != nil {
			t.Fatalf("Can't create directory: %s", err)
		}

		firstMountPoint, err := fs.GetMountPoint(firstDir)
		if err != nil {
			t.Fatalf("Can't get mount point: %s", err)
		}

		secondDir := filepath.Join(data.mountPointSecond, "dir2")

		if err := os.MkdirAll(secondDir, 0o755); err != nil {
			t.Fatalf("Can't create directory: %s", err)
		}

		secondMountPoint, err := fs.GetMountPoint(secondDir)
		if err != nil {
			t.Fatalf("Can't get mount point: %s", err)
		}

		if !data.compare(firstMountPoint, secondMountPoint) {
			t.Errorf("Incorrect mounts location")
		}

		if firstMountPoint == secondMountPoint {
			if err := fs.Umount(data.mountPointFirst); err != nil {
				t.Fatalf("Can't umount partition: %s", err)
			}
		} else {
			if err := fs.Umount(data.mountPointFirst); err != nil {
				t.Fatalf("Can't umount partition: %s", err)
			}

			if err := fs.Umount(data.mountPointSecond); err != nil {
				t.Fatalf("Can't umount partition: %s", err)
			}
		}
	}
}

func TestOverlayMount(t *testing.T) {
	content := []string{"file0", "file1", "file2", "file3", "file4", "file5", "file6"}
	lowerDirs := []string{
		filepath.Join(tmpDir, "lower0"), filepath.Join(tmpDir, "lower1"),
		filepath.Join(tmpDir, "lower2"),
	}

	// Create content

	if err := createDirContent(lowerDirs[0], content[:2]); err != nil {
		t.Fatalf("Can't create lower dir content: %s", err)
	}

	if err := createDirContent(lowerDirs[1], content[2:4]); err != nil {
		t.Fatalf("Can't create lower dir content: %s", err)
	}

	if err := createDirContent(lowerDirs[2], content[4:]); err != nil {
		t.Fatalf("Can't create lower dir content: %s", err)
	}

	workDir := filepath.Join(tmpDir, "workDir")

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("Can't create work dir: %s", err)
	}

	upperDir := filepath.Join(tmpDir, "upperDir")

	if err := os.MkdirAll(upperDir, 0o755); err != nil {
		t.Fatalf("Can't create upper dir: %s", err)
	}

	// Overlay mount

	if err := fs.OverlayMount(mountPoint, lowerDirs, workDir, upperDir); err != nil {
		t.Fatalf("Can't mount overlay dir: %s", err)
	}

	// Check content

	if err := checkContent(mountPoint, content); err != nil {
		t.Errorf("Overlay content mismatch: %s", err)
	}

	// Write some file

	newContent := []string{"newFile0", "newFile1", "newFile2"}

	if err := createDirContent(mountPoint, newContent); err != nil {
		t.Fatalf("Can't create new content: %s", err)
	}

	if err := fs.Umount(mountPoint); err != nil {
		t.Errorf("Can't unmount overlay dir: %s", err)
	}

	// New content should be in upper dir

	if err := checkContent(upperDir, newContent); err != nil {
		t.Errorf("Upper dir content mismatch: %s", err)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

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

func createDirContent(path string, content []string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return aoserrors.Wrap(err)
	}

	for _, fileName := range content {
		file, err := os.Create(filepath.Join(path, fileName))
		if err != nil {
			return aoserrors.Wrap(err)
		}

		file.Close()
	}

	return nil
}

func checkContent(path string, content []string) error {
	file, err := os.Open(path)
	if err != nil {
		return aoserrors.Wrap(err)
	}
	defer file.Close()

	dirContent, err := file.Readdir(0)
	if err != nil {
		return aoserrors.Wrap(err)
	}

	if len(dirContent) != len(content) {
		return aoserrors.Errorf("wrong files count: %d", len(dirContent))
	}

contentLoop:
	for _, fileName := range content {
		for _, item := range dirContent {
			if fileName == item.Name() {
				continue contentLoop
			}
		}

		return aoserrors.Errorf("file %s not found", fileName)
	}

	return nil
}

func getDirSize(path string) (int64, error) {
	out, err := exec.Command("du", "-s", "-B1", path).Output()
	if err != nil {
		return 0, aoserrors.Wrap(err)
	}

	fields := strings.Fields(string(out))

	if len(fields) < 2 {
		return 0, aoserrors.New("wrong output format")
	}

	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, aoserrors.Wrap(err)
	}

	return size, nil
}

func getAvailableSize(path string) (int64, error) {
	out, err := exec.Command("df", "-h", "-B1", path).Output()
	if err != nil {
		return 0, aoserrors.Wrap(err)
	}

	lines := strings.Split(string(out), "\n")

	if len(lines) < 2 {
		return 0, aoserrors.New("wrong output format")
	}

	fields := strings.Fields(lines[1])

	if len(fields) < 4 {
		return 0, aoserrors.New("wrong output format")
	}

	size, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0, aoserrors.Wrap(err)
	}

	return size, nil
}
