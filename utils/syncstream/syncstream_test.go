package syncstream_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/aosedge/aos_common/utils/syncstream"
)

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestSyncStream(t *testing.T) {
	type testType1 struct {
		i int
	}

	type testType2 struct {
		s string
	}

	type testType3 struct {
		f float64
	}

	testData := []interface{}{
		&testType1{i: 42},
		&testType2{s: "test"},
		&testType3{f: 3.14},
	}

	testChannel := make(chan interface{}, 1)

	stream := syncstream.New()

	go func() {
		for {
			response, ok := <-testChannel
			if !ok {
				return
			}

			stream.ProcessMessages(response)
		}
	}()

	for _, testItem := range testData {
		response, err := stream.Send(context.Background(), func() error {
			testChannel <- testItem

			return nil
		}, reflect.TypeOf(testItem))
		if err != nil {
			t.Errorf("Send stream error: %v", err)
		}

		if response != testItem {
			t.Errorf("Wrong response received: %v", response)
		}
	}

	close(testChannel)
}
