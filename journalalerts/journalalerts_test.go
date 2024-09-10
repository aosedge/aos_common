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

package journalalerts_test

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aosedge/aos_common/aoserrors"
	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"
	"github.com/aosedge/aos_common/journalalerts"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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
 * Types
 **********************************************************************************************************************/

type instanceInfo struct {
	instanceIdent  aostypes.InstanceIdent
	serviceVersion string
}

type testInstanceProvider struct {
	instancesInfo map[string]instanceInfo
}

type testCursorStorage struct {
	cursor string
}

type testSystemdJournal struct {
	sync.RWMutex
	messages       []*sdjournal.JournalEntry
	currentMessage int
	systemdMatches []string
}

type testSender struct {
	alertsChannel chan cloudprotocol.AlertItem
}

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var (
	errTimeout       = errors.New("timeout")
	errIncorrectType = errors.New("incorrect alert type")
)

var (
	instanceProvider = testInstanceProvider{instancesInfo: make(map[string]instanceInfo)}
	cursorStorage    testCursorStorage
)

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestGetSystemError(t *testing.T) {
	testJournal := testSystemdJournal{}
	testSender := newTestSender()
	journalalerts.SDJournal = &testJournal

	const numMessages = 5

	alertsHandler, err := journalalerts.New(journalalerts.Config{
		ServiceAlertPriority: 4,
		SystemAlertPriority:  3,
	},
		&instanceProvider, &cursorStorage, testSender)
	if err != nil {
		t.Fatalf("Can't create alerts: %s", err)
	}
	defer alertsHandler.Close()

	// Check crit message received

	messages := make([]string, numMessages)

	for i := 0; i < numMessages; i++ {
		messages[i] = uuid.New().String()

		testJournal.addMessage(messages[i], "someSystemService", "", "3")
	}

	if err = waitAlerts(testSender.alertsChannel, 5*time.Second,
		cloudprotocol.AlertTagSystemError, aostypes.InstanceIdent{}, "1.0.0", messages); err != nil {
		t.Errorf("Result failed: %s", err)
	}
}

func TestGetServiceError(t *testing.T) {
	testJournal := testSystemdJournal{}
	testSender := newTestSender()
	journalalerts.SDJournal = &testJournal

	alertsHandler, err := journalalerts.New(journalalerts.Config{
		ServiceAlertPriority: 4,
		SystemAlertPriority:  3,
	},
		&instanceProvider, &cursorStorage, testSender)
	if err != nil {
		t.Fatalf("Can't create alerts: %s", err)
	}
	defer alertsHandler.Close()

	instanceInfo := instanceInfo{
		instanceIdent: aostypes.InstanceIdent{
			ServiceID: "alertservice0",
			SubjectID: "subject0",
			Instance:  0,
		},
		serviceVersion: "1.0.0",
	}

	instanceID := fmt.Sprintf("%s_%s_%s", instanceInfo.instanceIdent.ServiceID, instanceInfo.instanceIdent.SubjectID,
		strconv.FormatUint(instanceInfo.instanceIdent.Instance, 10))

	unitName := "aos-service@" + instanceID + ".service"

	instanceProvider.instancesInfo[instanceID] = instanceInfo

	messages := []string{}

	// msg 1
	testJournal.addMessage("starting", "init.scope", "", "4")

	// msg 2
	message := unitName + ": Main process exited, code=dumped, status=11/SEGV"

	testJournal.addMessage(message, unitName, "", "3")

	messages = append(messages, message)

	// msg 3
	message = unitName + ": Failed with result 'core-dump'."

	testJournal.addMessage(message, "", "/system.slice/system-aos@service.slice/"+unitName, "3")

	messages = append(messages, message)

	if err = waitAlerts(testSender.alertsChannel, 5*time.Second,
		cloudprotocol.AlertTagServiceInstance, instanceInfo.instanceIdent, "1.0.0", messages); err != nil {
		t.Errorf("Result failed: %s", err)
	}
}

func TestGetServiceManagerAlerts(t *testing.T) {
	testJournal := testSystemdJournal{}
	testSender := newTestSender()
	journalalerts.SDJournal = &testJournal

	alertsHandler, err := journalalerts.New(journalalerts.Config{
		ServiceAlertPriority: 4,
		SystemAlertPriority:  3,
	},
		&instanceProvider, &cursorStorage, testSender)
	if err != nil {
		t.Fatalf("Can't create alerts: %s", err)
	}
	defer alertsHandler.Close()

	const numMessages = 5

	messages := make([]string, 0, numMessages)

	for i := 0; i < numMessages; i++ {
		messages = append(messages, uuid.New().String())

		testJournal.addMessage(messages[i], "aos-servicemanager.service", "", "3")
	}

	if err = waitAlerts(testSender.alertsChannel, 5*time.Second, cloudprotocol.AlertTagAosCore,
		aostypes.InstanceIdent{}, "1.0.0", messages); err != nil {
		t.Errorf("Result failed: %s", err)
	}
}

func TestMessageFilter(t *testing.T) {
	testJournal := testSystemdJournal{}
	testSender := newTestSender()
	journalalerts.SDJournal = &testJournal

	const numMessages = 3

	filter := []string{"test", "regexp"}

	alertsHandler, err := journalalerts.New(journalalerts.Config{
		ServiceAlertPriority: 4,
		SystemAlertPriority:  3,
		Filter:               filter,
	},
		&instanceProvider, &cursorStorage, testSender)
	if err != nil {
		t.Fatalf("Can't create alerts: %s", err)
	}
	defer alertsHandler.Close()

	validMessage := "message should not be filterout"
	messages := []string{"test mesage to filterout", validMessage, "regexp mesage to filterout"}

	for i := range messages {
		testJournal.addMessage(messages[i], "test.service", "", "3")
	}

	foundCount := 0

	for i := 0; i < numMessages; i++ {
		err := waitResult(testSender.alertsChannel, 1*time.Second,
			func(alert cloudprotocol.AlertItem) (success bool, err error) {
				if alert.Tag != cloudprotocol.AlertTagSystemError {
					return false, aoserrors.New("wrong alert type")
				}

				systemAlert, ok := alert.Payload.(cloudprotocol.SystemAlert)
				if !ok {
					return false, aoserrors.New("wrong alert type content")
				}

				if systemAlert.Message != validMessage {
					return false, aoserrors.New("receive unexpected alert mesage")
				}

				return true, nil
			})

		if err == nil {
			foundCount++

			continue
		}

		if !errors.Is(err, errTimeout) {
			t.Errorf("Result failed: %s", err)
		}
	}

	if foundCount != 1 {
		t.Errorf("Incorrect count of received alerts count = %d", foundCount)
	}
}

func TestWrongFilter(t *testing.T) {
	testJournal := testSystemdJournal{}
	testSender := newTestSender()
	journalalerts.SDJournal = &testJournal

	alertsHandler, err := journalalerts.New(journalalerts.Config{
		Filter:               []string{"", "*(test)^"},
		ServiceAlertPriority: 4,
		SystemAlertPriority:  3,
	},
		&instanceProvider, &cursorStorage, testSender)
	if err != nil {
		t.Fatalf("Can't create alerts: %s", err)
	}
	defer alertsHandler.Close()
}

func TestJournalSetup(t *testing.T) {
	testJournal := testSystemdJournal{}
	journalalerts.SDJournal = &testJournal

	alertsConfig := journalalerts.Config{
		ServiceAlertPriority: 4,
		SystemAlertPriority:  3,
	}

	etalonMatches := []string{"_SYSTEMD_UNIT=init.scope"}

	for priorityLevel := 0; priorityLevel <= alertsConfig.SystemAlertPriority; priorityLevel++ {
		etalonMatches = append(etalonMatches, fmt.Sprintf("PRIORITY=%d", alertsConfig.SystemAlertPriority))
	}

	_ = cursorStorage.SetJournalCursor("somecursor")

	alertsHandler, err := journalalerts.New(alertsConfig, &instanceProvider, &cursorStorage, nil)
	if err != nil {
		t.Fatalf("Can't create alerts: %s", err)
	}
	defer alertsHandler.Close()

matchLoop:
	for _, etalonMatch := range etalonMatches {
		for _, journalMatch := range testJournal.systemdMatches {
			if etalonMatch == journalMatch {
				continue matchLoop
			}
		}

		t.Errorf("Journal filter doesn't contains: %s", etalonMatch)
	}
}

/***********************************************************************************************************************
 * Interfaces
 **********************************************************************************************************************/

func (instanceProvider *testInstanceProvider) GetInstanceInfoByID(
	id string,
) (ident aostypes.InstanceIdent, version string, err error) {
	instance, ok := instanceProvider.instancesInfo[id]
	if !ok {
		return ident, version, aoserrors.New("Instance does not exist")
	}

	return instance.instanceIdent, instance.serviceVersion, nil
}

func (cursorStorage *testCursorStorage) SetJournalCursor(cursor string) (err error) {
	cursorStorage.cursor = cursor

	return nil
}

func (cursorStorage *testCursorStorage) GetJournalCursor() (cursor string, err error) {
	return cursorStorage.cursor, nil
}

func (journal *testSystemdJournal) Next() (uint64, error) {
	journal.Lock()
	defer journal.Unlock()

	if len(journal.messages) == 0 {
		return uint64(sdjournal.SD_JOURNAL_NOP), nil
	}

	if journal.currentMessage >= len(journal.messages)-1 {
		return uint64(sdjournal.SD_JOURNAL_NOP), nil
	}

	journal.currentMessage++

	return uint64(sdjournal.SD_JOURNAL_APPEND), nil
}

func (journal *testSystemdJournal) GetEntry() (entry *sdjournal.JournalEntry, err error) {
	journal.RLock()
	defer journal.RUnlock()

	entry = journal.messages[journal.currentMessage]

	return entry, nil
}

func (journal *testSystemdJournal) Wait(timeout time.Duration) int {
	time.Sleep(timeout)

	journal.RLock()
	defer journal.RUnlock()

	if len(journal.messages) == 0 {
		return sdjournal.SD_JOURNAL_NOP
	}

	if journal.currentMessage >= len(journal.messages)-1 {
		return sdjournal.SD_JOURNAL_NOP
	}

	return sdjournal.SD_JOURNAL_APPEND
}

func (journal *testSystemdJournal) Close() error { return nil }

func (journal *testSystemdJournal) AddMatch(match string) error {
	journal.systemdMatches = append(journal.systemdMatches, match)

	return nil
}

func (journal *testSystemdJournal) AddDisjunction() error { return nil }

func (journal *testSystemdJournal) SeekTail() error { return nil }

func (journal *testSystemdJournal) Previous() (uint64, error) {
	journal.Lock()
	defer journal.Unlock()

	journal.currentMessage = -1

	return uint64(sdjournal.SD_JOURNAL_NOP), nil
}

func (journal *testSystemdJournal) SeekCursor(cursor string) error { return nil }

func (journal *testSystemdJournal) GetCursor() (string, error) { return "", nil }

func (journal *testSystemdJournal) addMessage(message, systemdUnit, cgroupUnit, priority string) {
	journal.Lock()
	defer journal.Unlock()

	journalEntry := sdjournal.JournalEntry{Fields: make(map[string]string)}

	journalEntry.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE] = message
	journalEntry.Fields[sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT] = systemdUnit
	journalEntry.Fields[sdjournal.SD_JOURNAL_FIELD_SYSTEMD_CGROUP] = cgroupUnit
	journalEntry.Fields[sdjournal.SD_JOURNAL_FIELD_PRIORITY] = priority

	journal.messages = append(journal.messages, &journalEntry)
}

func (sender *testSender) SendAlert(alert cloudprotocol.AlertItem) {
	sender.alertsChannel <- alert
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func newTestSender() (sender *testSender) {
	sender = &testSender{
		alertsChannel: make(chan cloudprotocol.AlertItem, 1),
	}

	return sender
}

func waitResult(alertsChannel <-chan cloudprotocol.AlertItem, timeout time.Duration,
	checkAlert func(alert cloudprotocol.AlertItem) (success bool, err error),
) (err error) {
	for {
		select {
		case alert := <-alertsChannel:
			success, err := checkAlert(alert)
			if err != nil {
				return aoserrors.Wrap(err)
			}

			if success {
				return nil
			}

		case <-time.After(timeout):
			return errTimeout
		}
	}
}

func waitAlerts(alertsChannel <-chan cloudprotocol.AlertItem, timeout time.Duration,
	tag string, instance aostypes.InstanceIdent, serviceVersion string, data []string,
) (err error) {
	return waitResult(alertsChannel, timeout, func(alert cloudprotocol.AlertItem) (success bool, err error) {
		if alert.Tag != tag {
			return false, nil
		}

		for i, message := range data {
			var alertMsg string

			switch alert.Tag {
			case cloudprotocol.AlertTagAosCore:
				castedAlert, ok := alert.Payload.(cloudprotocol.CoreAlert)
				if !ok {
					return false, errIncorrectType
				}

				alertMsg = castedAlert.Message

			case cloudprotocol.AlertTagServiceInstance:
				castedAlert, ok := alert.Payload.(cloudprotocol.ServiceInstanceAlert)
				if !ok {
					return false, errIncorrectType
				}

				if castedAlert.InstanceIdent != instance || serviceVersion != castedAlert.ServiceVersion {
					continue
				}

				alertMsg = castedAlert.Message

			case cloudprotocol.AlertTagSystemError:
				castedAlert, ok := alert.Payload.(cloudprotocol.SystemAlert)
				if !ok {
					return false, errIncorrectType
				}

				alertMsg = castedAlert.Message
			}

			if alertMsg == message {
				data = append(data[:i], data[i+1:]...)

				if len(data) == 0 {
					return true, nil
				}

				return false, nil
			}
		}

		return false, nil
	})
}
