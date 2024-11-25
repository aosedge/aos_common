// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2024 Renesas Electronics Corporation.
// Copyright (C) 2024 EPAM Systems, Inc.
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

package semverutils_test

import (
	"fmt"
	"testing"

	"github.com/aosedge/aos_common/utils/semverutils"
)

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestLessThan(t *testing.T) {
	successCases := []struct {
		version1       string
		version2       string
		expectedResult bool
	}{
		{"1.9", "1.10", true},
		{"1.5", "1.3", false},
		{"1.3", "1.3", false},
	}

	for _, testcase := range successCases {
		testName := fmt.Sprintf("semverutils.LessThan(%s, %s)", testcase.version1, testcase.version2)

		res, err := semverutils.LessThan(testcase.version1, testcase.version2)
		checkSuccess(t, res, testcase.expectedResult, err, testName)
	}

	errorCases := []struct {
		version1 string
		version2 string
	}{
		{"1.x", "1.3"},
		{"1.y", "1.x"},
	}

	for _, testcase := range errorCases {
		testName := fmt.Sprintf("semverutils.LessThan(%s, %s)", testcase.version1, testcase.version2)

		_, err := semverutils.LessThan(testcase.version1, testcase.version2)
		checkFailure(t, err, testName)
	}
}

func TestGreaterThan(t *testing.T) {
	successCases := []struct {
		version1       string
		version2       string
		expectedResult bool
	}{
		{"1.9", "1.10", false},
		{"1.5", "1.3", true},
		{"1.3", "1.3", false},
	}

	for _, testcase := range successCases {
		testName := fmt.Sprintf("semverutils.GreaterThan(%s, %s)", testcase.version1, testcase.version2)

		res, err := semverutils.GreaterThan(testcase.version1, testcase.version2)
		checkSuccess(t, res, testcase.expectedResult, err, testName)
	}

	errorCases := []struct {
		version1 string
		version2 string
	}{
		{"1.x", "1.3"},
		{"1.y", "1.x"},
	}

	for _, testcase := range errorCases {
		testName := fmt.Sprintf("semverutils.GreaterThan(%s, %s)", testcase.version1, testcase.version2)

		_, err := semverutils.GreaterThan(testcase.version1, testcase.version2)
		checkFailure(t, err, testName)
	}
}

func TestCompare(t *testing.T) {
	successCases := []struct {
		version1       string
		version2       string
		expectedResult int
	}{
		{"1.9", "1.10", -1},
		{"1.5", "1.3", 1},
		{"1.3", "1.3", 0},
	}

	for _, testcase := range successCases {
		testName := fmt.Sprintf("semverutils.Compare(%s, %s)", testcase.version1, testcase.version2)

		res, err := semverutils.Compare(testcase.version1, testcase.version2)
		checkSuccess(t, res, testcase.expectedResult, err, testName)
	}

	errorCases := []struct {
		version1 string
		version2 string
	}{
		{"1.x", "1.3"},
		{"1.y", "1.x"},
	}

	for _, testcase := range errorCases {
		testName := fmt.Sprintf("semverutils.GreaterThan(%s, %s)", testcase.version1, testcase.version2)

		_, err := semverutils.GreaterThan(testcase.version1, testcase.version2)
		checkFailure(t, err, testName)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func checkSuccess(t *testing.T, actual, expected interface{}, err error, name string) {
	t.Helper()

	if actual != expected {
		t.Errorf("Wrong result for %s: actual=%v expected=%v", name, actual, expected)
	}

	if err != nil {
		t.Errorf("Unexpected error: %v for %s", err, name)
	}
}

func checkFailure(t *testing.T, err error, message string) {
	t.Helper()

	if err == nil {
		t.Errorf("Error expected for %s", message)
	}
}
