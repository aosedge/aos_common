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

package aoserrors_test

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/aosedge/aos_common/aoserrors"
)

var errTestError = errors.New("test error")

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestAosError(t *testing.T) {
	err := aoserrors.Wrap(errTestError)

	if !errors.Is(err, errTestError) {
		t.Error("Wrapped error should be errTestError")
	}

	pc, _, line, _ := runtime.Caller(0)
	line -= 6

	f := runtime.FuncForPC(pc)
	if f == nil {
		t.Fatal("Can't get func for PC")
	}

	if err.Error() != fmt.Sprintf("%s [%s:%d]", errTestError.Error(), f.Name(), line) {
		t.Errorf("Wrong error message: %s", err.Error())
	}

	err = aoserrors.Wrap(err)

	if !errors.Is(err, errTestError) {
		t.Error("Wrapped error should be errTestError")
	}

	if err.Error() != fmt.Sprintf("%s [%s:%d]", errTestError.Error(), f.Name(), line) {
		t.Errorf("Wrong error message: %s", err.Error())
	}
}
