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

package retryhelper_test

import (
	"context"
	"testing"
	"time"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/utils/retryhelper"
)

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestRetryHelper(t *testing.T) {
	const (
		retryDelay = 1 * time.Second
		tolerance  = 0.1
	)

	// Success on first attempt

	callCount := 0
	timeStamps := []time.Time{}
	successIndex := 0

	testFunction := func() (err error) {
		callCount++

		timeStamps = append(timeStamps, time.Now())

		if callCount-1 == successIndex {
			return nil
		}

		return aoserrors.New("some error occurs")
	}

	if err := retryhelper.Retry(context.Background(), testFunction, nil, 3, retryDelay, 0); err != nil {
		t.Errorf("Retry error: %s", err)
	}

	if callCount != 1 {
		t.Errorf("Wrong call count: %d", callCount)
	}

	// Success on second attempt

	callCount = 0
	timeStamps = []time.Time{}
	successIndex = 1

	if err := retryhelper.Retry(context.Background(), testFunction, nil, 3, retryDelay, 0); err != nil {
		t.Errorf("Retry error: %s", err)
	}

	if callCount != 2 {
		t.Errorf("Wrong call count: %d", callCount)
	}

	if err := checkTimeStamps(timeStamps, retryDelay, tolerance); err != nil {
		t.Errorf("Wrong time stamp: %s", err)
	}

	// Fail

	callCount = 0
	timeStamps = []time.Time{}
	successIndex = 3

	if err := retryhelper.Retry(context.Background(), testFunction, nil, 3, retryDelay, 0); err == nil {
		t.Error("Error expected")
	}

	if callCount != 3 {
		t.Errorf("Wrong call count: %d", callCount)
	}

	if err := checkTimeStamps(timeStamps, retryDelay, tolerance); err != nil {
		t.Errorf("Wrong time stamp: %s", err)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func checkTimeStamps(timeStamps []time.Time, delay time.Duration, tolerance float64) (err error) {
	if len(timeStamps) < 2 {
		return nil
	}

	for i := 1; i < len(timeStamps); i++ {
		currentDelay := timeStamps[i].Sub(timeStamps[i-1])

		if float64(currentDelay) < float64(delay)*(1.0-tolerance) ||
			float64(currentDelay) > float64(delay)*(1.0+tolerance) {
			return aoserrors.Errorf("wrong time stamp: %d", i)
		}

		delay *= 2
	}

	return nil
}
