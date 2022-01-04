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

package action_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/utils/action"
)

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestExecute(t *testing.T) {
	testData := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	mutex := sync.Mutex{}

	actionHandler := action.New(len(testData) / 2)

	resultMap := make(map[string]int)

	for _, id := range testData {
		actionHandler.Execute(id, func(id string) (err error) {
			mutex.Lock()
			defer mutex.Unlock()

			if _, ok := resultMap[id]; ok {
				t.Errorf("Action is already executed: %s", id)

				return nil
			}

			resultMap[id] = 0

			return nil
		})
	}

	actionHandler.Wait()

	if len(resultMap) != len(testData) {
		t.Errorf("Wrong result len: %d", len(resultMap))
	}
}

func TestExecuteChannel(t *testing.T) {
	actionError := aoserrors.New("this is error")
	testData := map[string]error{
		"0": nil, "1": actionError, "2": nil, "3": nil, "4": actionError,
		"5": nil, "6": nil, "7": actionError, "8": actionError, "9": nil,
	}
	mutex := sync.Mutex{}

	actionHandler := action.New(len(testData))

	var wg sync.WaitGroup

	for id := range testData {
		wg.Add(1)

		go func(id string) {
			defer wg.Done()

			channel := actionHandler.Execute(id, func(id string) (err error) {
				mutex.Lock()
				defer mutex.Unlock()

				return testData[id]
			})

			actionError := <-channel

			if !errors.Is(testData[id], actionError) {
				t.Errorf("Wrong action %s error: %v", id, actionError)
			}
		}(id)
	}

	wg.Wait()
}
