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

package image_test

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/aostypes"
	"github.com/aoscloud/aos_common/image"
	"github.com/aoscloud/aos_common/utils/testtools"
	"github.com/opencontainers/go-digest"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const (
	filePerm       = 0o600
	dirSymlinkSize = int64(4 * 1024)
)

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var workDir string

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

	if workDir, err = os.MkdirTemp("", "aos_"); err != nil {
		log.Fatalf("Error create work dir: %s", err)
	}

	ret := m.Run()

	if err = os.RemoveAll(workDir); err != nil {
		log.Fatalf("Error removing tmp dir: %s", err)
	}

	os.Exit(ret)
}

func TestUntarGZArchive(t *testing.T) {
	// test destination does not exist
	if err := image.UntarGZArchive(context.Background(),
		path.Join(workDir, "no_tar"), path.Join(workDir, "no_dest")); err == nil {
		t.Error("UntarGZArchive should failed:  destination does not exist")
	}

	if err := os.MkdirAll(path.Join(workDir, "outfolder"), 0o755); err != nil {
		t.Fatalf("Error creating tmp dir %s", err)
	}

	if err := image.UntarGZArchive(context.Background(),
		path.Join(workDir, "no_tar"), path.Join(workDir, "outfolder")); err == nil {
		t.Error("UntarGZArchive should failed:  no such file or directory")
	}

	// test invalid archive
	if err := os.WriteFile(path.Join(workDir, "testArchive.tar.gz"),
		[]byte("This is test file"), 0o600); err != nil {
		t.Fatalf("Can't write test file: %s", err)
	}

	if err := image.UntarGZArchive(context.Background(),
		path.Join(workDir, "testArchive.tar.gz"), ""); err == nil {
		t.Error("UntarGZArchive should failed: invalid header")
	}

	// prepare source folder and create archive
	if err := os.MkdirAll(path.Join(workDir, "archive_folder"), 0o755); err != nil {
		t.Fatalf("Error creating tmp dir %s", err)
	}

	if err := os.MkdirAll(path.Join(workDir, "archive_folder", "dir1"), 0o755); err != nil {
		t.Fatalf("Error creating tmp dir %s", err)
	}

	if err := os.WriteFile(path.Join(workDir, "archive_folder", "file.txt"),
		[]byte("This is test file"), 0o600); err != nil {
		t.Fatalf("Can't write test file: %s", err)
	}

	if err := os.WriteFile(path.Join(workDir, "archive_folder", "dir1", "file2.txt"),
		[]byte("This is test file2"), 0o600); err != nil {
		t.Fatalf("Can't write test file: %s", err)
	}

	command := exec.Command("tar", "-czf", path.Join(workDir, "test_archive.tar.gz"), "-C",
		path.Join(workDir, "archive_folder"), ".")
	if err := command.Run(); err != nil {
		t.Fatalf("Can't run tar: %s", err)
	}

	if err := image.UntarGZArchive(context.Background(),
		path.Join(workDir, "test_archive.tar.gz"), path.Join(workDir, "outfolder")); err != nil {
		t.Fatalf("Untar error: %s", err)
	}

	// compare source dir and untarred dir
	command = exec.Command("git", "diff", "--no-index", path.Join(workDir, "archive_folder"),
		path.Join(workDir, "outfolder"))

	if out, _ := command.Output(); string(out) != "" {
		t.Errorf("Untar content not identical")
	}
}

func TestDownload(t *testing.T) {
	if _, err := image.Download(context.Background(), workDir, "https://gobyexample.com/maps"); err != nil {
		t.Errorf("File can not be downloaded: %s", err)
	}

	if _, err := image.Download(context.Background(), workDir, "fake_url"); err == nil {
		t.Errorf("Expect error because we use a fake URL: %s", err)
	}
}

func TestCreateFileInfo(t *testing.T) {
	fileNamePath := path.Join(workDir, "file")

	if err := os.WriteFile(fileNamePath, []byte("Hello"), filePerm); err != nil {
		t.Fatalf("Error create a new file: %s", err)
	}

	info, err := image.CreateFileInfo(context.Background(), fileNamePath)
	if err != nil {
		t.Errorf("Error creating file info: %s", err)
	}

	out, err := exec.Command("du", "-b", fileNamePath).Output()
	if err != nil {
		t.Fatalf("du returns error result: %s", err)
	}

	fileSize, err := strconv.ParseUint(strings.Fields(string(out))[0], 10, 64)
	if err != nil {
		t.Fatalf("Bad conversion str to int: %s", err)
	}

	if fileSize != info.Size {
		t.Errorf("Size of file mismatch. Expect: %d, actual: %d", fileSize, info.Size)
	}

	out, err = exec.Command("openssl", "dgst", "-sha3-256", fileNamePath).Output()
	if err != nil {
		t.Fatalf("openssl dgst -sha3-256 returns error result: %s", err)
	}

	shaStr := strings.Fields(string(out))
	actualCheckSum := hex.EncodeToString(info.Sha256)

	if shaStr[1] != actualCheckSum {
		t.Errorf("sha256 not equals. Expected: %s, actual: %s", shaStr[1], actualCheckSum)
	}
}

func TestCheckFileInfo(t *testing.T) {
	fileNamePath := path.Join(workDir, "file")

	if err := os.WriteFile(fileNamePath, []byte("Hello"), filePerm); err != nil {
		t.Fatalf("Error create a new file: %s", fileNamePath)
	}

	info, err := image.CreateFileInfo(context.Background(), fileNamePath)
	if err != nil {
		t.Errorf("Can't create file info: %s", err)
	}

	if err = image.CheckFileInfo(context.Background(), fileNamePath, info); err != nil {
		t.Errorf("File info mismatch: %s", err)
	}

	// --- Negative cases
	// Bad file size case
	tmpFileSize := info.Size
	info.Size++

	if err = image.CheckFileInfo(context.Background(), fileNamePath, info); err == nil {
		t.Error("File size should not be matched")
	}

	info.Size = tmpFileSize

	// Bad sha256sum case
	tmpSha256 := info.Sha256[0]
	info.Sha256[0]--

	if err = image.CheckFileInfo(context.Background(), fileNamePath, info); err == nil {
		t.Error("sha256 should not be matched")
	}

	info.Sha256[0] = tmpSha256
}

func TestUncompressedContentSize(t *testing.T) {
	contentSize := int64(2 * 1024)

	dir, err := createTestDir("testDir1", contentSize)
	if err != nil {
		t.Fatalf("Can't create test dir: %v", err)
	}

	defer os.Remove(dir)

	tarFile1 := filepath.Join(workDir, "archive1.tar")

	if output, err := exec.Command("tar", "-C", dir, "-cf", tarFile1, "./").CombinedOutput(); err != nil {
		t.Fatalf("Can't create tar archive: message: %s, err: %v", string(output), err)
	}
	defer os.Remove(tarFile1)

	size, err := image.GetUncompressedTarContentSize(tarFile1)
	if err != nil {
		t.Fatalf("Can't get tar content size: %v", err)
	}

	if contentSize+dirSymlinkSize+dirSymlinkSize != size {
		t.Errorf("Unexpected tar content size: %v", size)
	}

	tarFile2 := filepath.Join(workDir, "archive2.tar.gz")

	if output, err := exec.Command("tar", "-C", dir, "-czf", tarFile2, "./").CombinedOutput(); err != nil {
		t.Fatalf("Can't create tar archive: message: %s, err: %v", string(output), err)
	}
	defer os.Remove(tarFile2)

	if size, err = image.GetUncompressedTarContentSize(tarFile2); err != nil {
		t.Fatalf("Can't get tar content size: %v", err)
	}

	if contentSize+dirSymlinkSize+dirSymlinkSize != size {
		t.Errorf("Unexpected tar content size: %v", size)
	}
}

func TestCopyImage(t *testing.T) {
	fileSize := int64(1500000)
	srcFile := filepath.Join(workDir, "src.dat")
	dstFile := filepath.Join(workDir, "dst.dat")

	if err := generateRandomFile(srcFile, fileSize); err != nil {
		t.Fatalf("Can't generate random file: %v", err)
	}

	copied, err := image.Copy(dstFile, srcFile)
	if err != nil {
		t.Fatalf("Can't copy image: %v", err)
	}

	if copied != fileSize {
		t.Fatalf("Wrong copied size: %d", copied)
	}

	if err = testtools.CompareFiles(dstFile, srcFile); err != nil {
		t.Errorf("Compare error: %s", err)
	}
}

func TestCopyImageFromGzipArchive(t *testing.T) {
	fileSize := int64(1500000)
	srcFile := filepath.Join(workDir, "src.dat")
	dstFile := filepath.Join(workDir, "dst.dat")
	archiveFile := filepath.Join(workDir, "dst.gz")

	if err := generateRandomFile(srcFile, fileSize); err != nil {
		t.Fatalf("Can't generate random file: %v", err)
	}

	if err := gzipFile(archiveFile, srcFile); err != nil {
		t.Fatalf("Can't generate random file: %v", err)
	}

	copied, err := image.CopyFromGzipArchive(dstFile, archiveFile)
	if err != nil {
		t.Fatalf("Can't copy image: %v", err)
	}

	if copied != fileSize {
		t.Fatalf("Wrong copied size: %d", copied)
	}

	if err = testtools.CompareFiles(dstFile, srcFile); err != nil {
		t.Errorf("Compare error: %s", err)
	}
}

func TestCopyImageToDevice(t *testing.T) {
	fileSize := int64(1500000)
	srcFile := filepath.Join(workDir, "src.dat")
	dstFile := filepath.Join(workDir, "dst.dat")

	if err := generateRandomFile(srcFile, fileSize); err != nil {
		t.Fatalf("Can't generate random file: %v", err)
	}

	if err := os.RemoveAll(dstFile); err != nil {
		t.Fatalf("Can't remove destination file: %v", err)
	}

	if _, err := image.CopyToDevice(dstFile, srcFile, false); err == nil {
		t.Error("Error expected as device doesn't exist")
	}

	file, err := os.Create(dstFile)
	if err != nil {
		t.Fatalf("Can't create destination file: %v", err)
	}

	file.Close()

	copied, err := image.CopyToDevice(dstFile, srcFile, false)
	if err != nil {
		t.Fatalf("Can't copy image: %v", err)
	}

	if copied != fileSize {
		t.Fatalf("Wrong copied size: %d", copied)
	}

	if err = testtools.CompareFiles(dstFile, srcFile); err != nil {
		t.Errorf("Compare error: %s", err)
	}
}

func TestCopyImageFromGzipArchiveToDevice(t *testing.T) {
	fileSize := int64(1500000)
	srcFile := filepath.Join(workDir, "src.dat")
	dstFile := filepath.Join(workDir, "dst.dat")
	archiveFile := filepath.Join(workDir, "dst.gz")

	if err := generateRandomFile(srcFile, fileSize); err != nil {
		t.Fatalf("Can't generate random file: %v", err)
	}

	if err := gzipFile(archiveFile, srcFile); err != nil {
		t.Fatalf("Can't generate random file: %v", err)
	}

	if err := os.RemoveAll(dstFile); err != nil {
		t.Fatalf("Can't remove destination file: %v", err)
	}

	if _, err := image.CopyFromGzipArchiveToDevice(dstFile, srcFile, false); err == nil {
		t.Error("Error expected as device doesn't exist")
	}

	file, err := os.Create(dstFile)
	if err != nil {
		t.Fatalf("Can't create destination file: %v", err)
	}

	file.Close()

	copied, err := image.CopyFromGzipArchiveToDevice(dstFile, archiveFile, false)
	if err != nil {
		t.Fatalf("Can't copy image: %v", err)
	}

	if copied != fileSize {
		t.Fatalf("Wrong copied size: %d", copied)
	}

	if err = testtools.CompareFiles(dstFile, srcFile); err != nil {
		t.Errorf("Compare error: %s", err)
	}
}

func TestImageManifest(t *testing.T) {
	fileName := path.Join(workDir, "manifest.json")

	imgConfigDigest, err := generateAndSaveDigest(filepath.Join(workDir, "blobs"), []byte("{}"))
	if err != nil {
		t.Fatalf("Can't generate and save digest: %v", err)
	}

	rootDigest, err := generateAndSaveDigest(workDir, []byte("{}"))
	if err != nil {
		t.Fatalf("Can't generate and save digest: %v", err)
	}

	layerDigest2, err := generateAndSaveDigest(workDir, []byte("{}"))
	if err != nil {
		t.Fatalf("Can't generate and save digest: %v", err)
	}

	manifest := aostypes.ServiceManifest{
		Manifest: imagespec.Manifest{
			Config: imagespec.Descriptor{
				MediaType: "application/vnd.oci.image.config.v1+json",
				Digest:    imgConfigDigest,
			},
			Layers: []imagespec.Descriptor{
				{
					MediaType: "application/vnd.aos.image.layer.v1.tar",
					Digest:    rootDigest,
				},
				{
					MediaType: "application/vnd.aos.image.layer.v1.tar",
					Digest:    layerDigest2,
				},
			},
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Can't get image manifest: %v", err)
	}

	if err := os.WriteFile(fileName, data, 0o600); err != nil {
		t.Fatalf("Can't save manifest file %v", err)
	}

	serviceManifest, err := image.GetImageManifest(workDir)
	if err != nil {
		t.Fatalf("Can't get image manifest: %v", err)
	}

	if !reflect.DeepEqual(manifest, *serviceManifest) {
		t.Fatalf("Unexpected manifest data")
	}

	layers := image.GetLayersFromManifest(serviceManifest)
	if len(layers) != 1 {
		t.Fatalf("Unexpected layers count")
	}

	if layers[0] != layerDigest2.String() {
		t.Error("Unexpected layer digest")
	}

	if err = image.ValidateDigest(workDir, manifest.Config.Digest); err != nil {
		t.Errorf("Can't validate digest")
	}
}

/*******************************************************************************
 * Private
 ******************************************************************************/

func generateAndSaveDigest(folder string, data []byte) (retDigest digest.Digest, err error) {
	fullPath := filepath.Join(folder, "sha256")
	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		return retDigest, aoserrors.Wrap(err)
	}

	h := sha256.New()
	h.Write(data)
	retDigest = digest.NewDigest("sha256", h)

	file, err := os.Create(filepath.Join(fullPath, retDigest.Hex()))
	if err != nil {
		return retDigest, aoserrors.Wrap(err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return retDigest, aoserrors.Wrap(err)
	}

	return retDigest, nil
}

func createTestDir(dirName string, sizeContent int64) (string, error) {
	tmpFolder := filepath.Join(workDir, dirName)

	if err := os.MkdirAll(tmpFolder, 0o755); err != nil {
		return "", aoserrors.Wrap(err)
	}

	file, err := os.Create(filepath.Join(tmpFolder, "testFile.txt"))
	if err != nil {
		return "", aoserrors.Wrap(err)
	}
	defer file.Close()

	if err := file.Truncate(sizeContent); err != nil {
		return "", aoserrors.Wrap(err)
	}

	if err := os.Symlink(file.Name(), filepath.Join(tmpFolder, "symlink")); err != nil {
		return "", aoserrors.Wrap(err)
	}

	return tmpFolder, nil
}

func generateRandomFile(filePath string, size int64) error {
	file, err := os.Create(filePath)
	if err != nil {
		return aoserrors.Wrap(err)
	}
	defer file.Close()

	if _, err := io.CopyN(file, rand.Reader, size); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}

func gzipFile(dst, src string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return aoserrors.Wrap(err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return aoserrors.Wrap(err)
	}
	defer dstFile.Close()

	gzipWriter := gzip.NewWriter(dstFile)
	defer gzipWriter.Close()

	if _, err := io.Copy(gzipWriter, srcFile); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}
