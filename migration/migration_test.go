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

package migration_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	_ "github.com/mattn/go-sqlite3" //ignore lint
	log "github.com/sirupsen/logrus"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/migration"
)

/*******************************************************************************
 * Consts
 ******************************************************************************/

const (
	busyTimeout = 60000
	journalMode = "WAL"
	syncMode    = "NORMAL"
)

/*******************************************************************************
 * Public
 ******************************************************************************/

func TestSetCurrentVersion(t *testing.T) {
	if err := os.MkdirAll("tmp/migration", 0755); err != nil {
		t.Fatalf("Error creating directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll("tmp/migration"); err != nil {
			t.Fatalf("Error cleaning up: %s", err)
		}
	}()

	checkDbVersion(t, 1, "tmp/migration/test.db")
	checkDbVersion(t, 25, "tmp/migration/test.db")
	checkDbVersion(t, 222229925, "tmp/migration/test.db")
	checkDbVersion(t, 0, "tmp/migration/test.db")
}

func TestDbMigrationUp(t *testing.T) {
	testMigration(t, uint(1), uint(25))
}

func TestDbMigrationDown(t *testing.T) {
	testMigration(t, uint(25), uint(1))
}

func TestDbMigrationSameVer(t *testing.T) {
	testMigration(t, uint(1), uint(1))
}

func TestMigrationFail(t *testing.T) {
	if err := os.MkdirAll("tmp/migration", 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll("tmp/migration"); err != nil {
			t.Fatalf("Error cleaning up: %s", err)
		}
	}()

	var currentVersion uint = 1
	var nextVersion uint = 24

	if err := createTestDb("tmp/migration/test.db", currentVersion); err != nil {
		t.Errorf("Error preparing test db, err %s", err)
	}

	if err := generateMigrationFiles(currentVersion, "tmp/migration/migrations1"); err != nil {
		t.Errorf("Can't generate migration files for ver %d", currentVersion)
	}

	if err := generateMigrationFiles(nextVersion, "tmp/migration/migrations25"); err != nil {
		t.Errorf("Can't generate migration files for ver %d", nextVersion)
	}

	if err := breakMigrationVersion(13, "tmp/migration/migrations25"); err != nil {
		t.Errorf("Can't break migration files for ver %d", nextVersion)
	}

	dbLocal, err := startMigrationRoutine("tmp/migration/test.db", "tmp/migration/migrations1",
		"tmp/migration/mergedMigration", currentVersion)
	if err != nil {
		t.Fatalf("Can't create database: %s", err)
	}

	dbLocal.Close()

	if err = compareDbVersions(currentVersion, "tmp/migration/test.db"); err != nil {
		t.Errorf("Compare error %s", err)
	}

	dbLocal, err = startMigrationRoutine("tmp/migration/test.db", "tmp/migration/migrations25",
		"tmp/migration/mergedMigration", uint(nextVersion))
	if err == nil {
		t.Fatalf("Database is expected to be failed")
	}

	dbLocal.Close()
}

func TestInitialMigration(t *testing.T) {
	if err := os.MkdirAll("tmp/migration", 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll("tmp/migration"); err != nil {
			t.Fatalf("Error cleaning up: %s", err)
		}
	}()

	var initialVersion uint = 0
	var nextVersion uint = 25

	if err := createTestDb("tmp/migration/test.db", initialVersion); err != nil {
		t.Errorf("Error preparing test db, err %s", err)
	}

	if err := generateMigrationFiles(nextVersion, "tmp/migration/migrations25"); err != nil {
		t.Errorf("Can't generate migration files for ver %d", nextVersion)
	}

	dbLocal, err := startMigrationRoutine("tmp/migration/test.db", "tmp/migration/",
		"tmp/migration/mergedMigration", initialVersion)
	if err != nil {
		t.Errorf("Can't create database: %s", err)
	}

	//Removing schema_migrations from test db
	if err = removeMigrationDataFromDb(dbLocal); err != nil {
		t.Errorf("Unable to remove migration data")
	}

	dbLocal.Close()

	dbLocal, err = startMigrationRoutine("tmp/migration/test.db", "tmp/migration/migrations25",
		"tmp/migration/mergedMigration", uint(nextVersion))
	if err != nil {
		t.Errorf("Error during database creation: %s", err)
	}

	dbLocal.Close()

	if err = compareDbVersions(nextVersion, "tmp/migration/test.db"); err != nil {
		t.Error("Db has wrong version")
	}
}

func TestSetDatabaseVersion(t *testing.T) {
	if err := os.MkdirAll("tmp/migration", 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll("tmp/migration"); err != nil {
			t.Fatalf("Error cleaning up: %s", err)
		}
	}()

	currentVersion := uint(12)
	name := "tmp/migration/test.db"

	sqlite, err := getSQLConnection(name)
	if err != nil {
		t.Fatalf("Can't create database connection")
	}

	if err = migration.SetDatabaseVersion(sqlite, "tmp/migration", currentVersion); err != nil {
		t.Fatalf("Can't set database version")
	}

	sqlite.Close()

	if err = compareDbVersions(currentVersion, name); err != nil {
		t.Errorf("Compare error : %s", err)
	}
}

func TestMergeMigrationFiles(t *testing.T) {
	if err := os.MkdirAll("tmp/migration", 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll("tmp/migration"); err != nil {
			t.Fatalf("Error cleaning up: %s", err)
		}
	}()

	srcDir := "tmp/migration/srcDir"
	destDir := "tmp/migration/destDir"

	files := []string{"file1", "file2", "file3"}

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}

	if err := createEmptyFile(filepath.Join(srcDir, files[0])); err != nil {
		t.Errorf("Can't create empty file %s", err)
	}

	if err := createEmptyFile(filepath.Join(srcDir, files[1])); err != nil {
		t.Errorf("Can't create empty file %s", err)
	}

	if err := createEmptyFile(filepath.Join(destDir, files[1])); err != nil {
		t.Errorf("Can't create empty file %s", err)
	}

	if err := createEmptyFile(filepath.Join(destDir, files[2])); err != nil {
		t.Fatalf("Can't create empty file %s", err)
	}

	if err := migration.MergeMigrationFiles(srcDir, destDir); err != nil {
		t.Fatalf("Can't merge migration files %s", err)
	}

	destFiles, err := ioutil.ReadDir(destDir)
	if err != nil {
		t.Fatalf("Can't read destination directory")
	}

	for _, f := range destFiles {
		if sort.SearchStrings(files, f.Name()) == len(files) {
			t.Fatalf("Error, can't find file %s in merged path", f)
		}
	}
}

/*******************************************************************************
 * Private
 ******************************************************************************/
func createEmptyFile(path string) (err error) {
	srcFile, err := os.Create(path)
	if err != nil {
		return err
	}
	srcFile.Close()

	return nil
}

func compareDbVersions(currentVersion uint, name string) (err error) {
	//Check database version
	dbVersion, dirty, err := getCurrentDbVersion(name)
	if err != nil {
		return fmt.Errorf("Unable to get version from db. err: %s", err)
	}

	if dirty == true || dbVersion != currentVersion {
		return aoserrors.New("DB versions are different")
	}

	return nil
}

func createTestDb(dbName string, version uint) (err error) {
	conn := fmt.Sprintf("%s?_busy_timeout=%d&_journal_mode=%s&_sync=%s",
		dbName, busyTimeout, journalMode, syncMode)

	sql, err := sql.Open("sqlite3", conn)
	if err != nil {
		return err
	}
	defer sql.Close()

	// DB preparation
	if _, err = sql.Exec(
		`CREATE TABLE testing (
			version INTEGER)`); err != nil {
		return err
	}

	if _, err = sql.Exec(
		`INSERT INTO testing (
			version) values (?)`, version); err != nil {
		return err
	}

	if _, err := getOperationVersion(sql); err != nil {
		return err
	}

	return nil
}

func breakMigrationVersion(ver uint, path string) (err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	upScript := fmt.Sprintf("UPDATE broken_base SET version = %d;", ver)
	upPath := filepath.Join(abs, fmt.Sprintf("%d_update.up.sql", ver))
	if err = writeToFile(upPath, upScript); err != nil {
		return err
	}
	return nil
}

func generateMigrationFiles(verTo uint, path string) (err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(abs, 0755); err != nil {
		return err
	}

	var i uint

	for i = 0; i <= verTo; i++ {
		upScript := fmt.Sprintf("UPDATE testing SET version = %d;", i)
		upPath := filepath.Join(abs, fmt.Sprintf("%d_update.up.sql", i))
		downPath := filepath.Join(abs, fmt.Sprintf("%d_update.down.sql", i-1))
		downScript := fmt.Sprintf("UPDATE testing SET version = %d;", i-1)
		if err = writeToFile(upPath, upScript); err != nil {
			return err
		}
		if err = writeToFile(downPath, downScript); err != nil {
			return err
		}
	}
	return nil
}

func removeMigrationDataFromDb(sqlite *sql.DB) (err error) {
	_, err = sqlite.Exec("DROP TABLE IF EXISTS schema_migrations")

	return err
}

func getCurrentDbVersion(name string) (version uint, dirty bool, err error) {
	sql, err := sql.Open("sqlite3", fmt.Sprintf("%s?_busy_timeout=%d&_journal_mode=%s&_sync=%s",
		name, busyTimeout, journalMode, syncMode))
	if err != nil {
		return 0, false, err
	}
	defer sql.Close()

	stmt, err := sql.Prepare("SELECT version, dirty FROM schema_migrations LIMIT 1")
	if err != nil {
		return 0, false, err
	}
	defer stmt.Close()

	err = stmt.QueryRow().Scan(&version, &dirty)
	if err != nil {
		return 0, false, err
	}

	log.Debugf("version: %d, dirty: %v", version, dirty)

	return version, dirty, nil
}

func getOperationVersion(sql *sql.DB) (version int, err error) {
	stmt, err := sql.Prepare("SELECT version FROM testing")
	if err != nil {
		return version, err
	}
	defer stmt.Close()

	err = stmt.QueryRow().Scan(&version)
	if err != nil {
		return version, err
	}

	return version, nil
}

func writeToFile(path string, data string) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	file.WriteString(data)

	file.Close()

	return nil
}

func checkDbVersion(t *testing.T, currentVersion uint, name string) {
	if err := os.RemoveAll(name); err != nil {
		t.Fatalf("Error cleaning up: %s", err)
	}

	dbLocal, err := startMigrationRoutine(name, "tmp/migration", "tmp/migration", currentVersion)
	if err != nil {
		t.Errorf("Can't create database: %s", err)
	}

	dbLocal.Close()

	if err = compareDbVersions(currentVersion, name); err != nil {
		t.Errorf("Compare error : %s", err)
	}
}

func testMigration(t *testing.T, currentVersion uint, nextVersion uint) {
	if err := os.MkdirAll("tmp/migration", 0755); err != nil {
		t.Errorf("Error creating directory: %s", err)
	}
	defer func() {
		if err := os.RemoveAll("tmp/migration"); err != nil {
			t.Fatalf("Error cleaning up: %s", err)
		}
	}()

	if err := createTestDb("tmp/migration/test.db", currentVersion); err != nil {
		t.Errorf("Error preparing test db, err %s", err)
	}

	if err := generateMigrationFiles(currentVersion, "tmp/migration/migrations1"); err != nil {
		t.Errorf("Can't generate migration files for ver %d", currentVersion)
	}

	if err := generateMigrationFiles(nextVersion, "tmp/migration/migrations25"); err != nil {
		t.Errorf("Can't generate migration files for ver %d", nextVersion)
	}

	dbLocal, err := startMigrationRoutine("tmp/migration/test.db", "tmp/migration/migrations1",
		"tmp/migration/mergedMigration", currentVersion)
	if err != nil {
		t.Errorf("Can't create database: %s", err)
	}

	dbLocal.Close()

	if err = compareDbVersions(currentVersion, "tmp/migration/test.db"); err != nil {
		t.Errorf("Compare error %s", err)
	}

	dbLocal, err = startMigrationRoutine("tmp/migration/test.db", "tmp/migration/migrations25",
		"tmp/migration/mergedMigration", uint(nextVersion))
	if err != nil {
		t.Errorf("Can't create database: %s", err)
	}

	dbLocal.Close()

	if err = compareDbVersions(nextVersion, "tmp/migration/test.db"); err != nil {
		t.Errorf("Compare error %s", err)
	}
}

func startMigrationRoutine(name string, migrationPath string, mergedMigrationPath string,
	version uint) (sqlite *sql.DB, err error) {
	// Check and create db
	if _, err = os.Stat(filepath.Dir(name)); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return nil, err
		}
	}

	exists := true
	if _, err := os.Stat(name); os.IsNotExist(err) {
		exists = false
	}

	sqlite, err = getSQLConnection(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			sqlite.Close()
		}
	}()

	if err = migration.MergeMigrationFiles(migrationPath, mergedMigrationPath); err != nil {
		return sqlite, err
	}

	if !exists {
		// Set database version if database not exist
		if err = migration.SetDatabaseVersion(sqlite, migrationPath, version); err != nil {
			return sqlite, err
		}
	} else {
		if err = migration.DoMigrate(sqlite, mergedMigrationPath, version); err != nil {
			return sqlite, err
		}
	}

	return sqlite, nil
}

func getSQLConnection(name string) (sqlite *sql.DB, err error) {
	return sql.Open("sqlite3", fmt.Sprintf("%s?_busy_timeout=%d&_journal_mode=%s&_sync=%s",
		name, busyTimeout, journalMode, syncMode))
}
