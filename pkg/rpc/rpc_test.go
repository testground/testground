package rpc_test

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/rpc/rpctest"
)

type testcase struct {
	f        string          // The name of the method to be tested
	cts      []rpc.ChunkType // Chunk types to be expected in the result
	param    interface{}     // parameter to be passed to `f`
	expected interface{}     // expected chunk value
}

// testBody decodes the rpc body received, which is a json stream of rpc.Chunks
// Ensure received body conforms to the expectations defined in the testcase.
// Each method tested should produce a chunk of certain "ChunkType" and should
// carry the payload expected.
func testBody(t *testing.T, test *testcase, rdr io.Reader) {
	dec := json.NewDecoder(rdr)
	for dec.More() {
		var ch rpc.Chunk
		err := dec.Decode(&ch)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(t.Name()+test.f, func(t *testing.T) {
			// Check chunk type
			var isAllowed bool
			for _, tp := range test.cts {
				if ch.Type == tp {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				t.Log("incorrect ChunkType", string(ch.Type))
				t.Fail()
			}
			switch ch.Type {
			// base64-encoded strings that look like this when decoded:
			// May 19 09:51:43.022537	INFO	test	{"req_id": "TestLogging-Infof"}
			// Check the data after the timestamp matches expected.
			case rpc.ChunkTypeProgress:
				t.Log(ch.Type, ch.Payload)
				t.Log("expected", test.expected)
				if !strings.Contains(ch.Payload.(string), test.expected.(string)[30:]) {
					t.Log("ChunkTypeProgress data did not match what was expected.", ch.Payload)
					t.Fail()
				}

			// string or []uint8
			case rpc.ChunkTypeResult, rpc.ChunkTypeBinary:
				if ch.Payload != test.expected {
					t.Log("ChunkTypeResult data did not match what was expected.", ch.Payload)
					t.Fail()
				}

			case rpc.ChunkTypeError:
				if ch.Payload != nil {
					t.Log("ChunkTypeError", ch.Payload)
					t.Fail()
				}
			}
		})
	}
}

// Test methods directly attached to the outputWriter
func TestLogging(t *testing.T) {

	// Info and Warn send.
	// Debug does not send to the client
	// Fatal causes a crash. Just check the ones that send to the client.

	tcs := []testcase{
		{"Info", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDIxIDA2OjQwOjM3Ljg5NTE0OQkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmdJbmZvIn0K"},
		{"Infof", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDIxIDA2OjQwOjM3Ljg5NTM0MgkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmdJbmZvZiJ9Cg=="},
		{"Infow", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDIxIDA2OjQ2OjAxLjMzNDA4OQkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmdJbmZvdyJ9Cg=="},
		{"Error", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDIxIDA2OjQ2OjMzLjc5MTgyNgkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nRXJyb3IifQo="},
		{"Errorf", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDIxIDA2OjQ2OjUxLjI4NTUwNgkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nRXJyb3JmIn0K"},
		{"Errorw", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "WF5IDIxIDA2OjQ3OjA3LjE1ODkyMgkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nRXJyb3J3In0K"},
		{"WriteError", []rpc.ChunkType{rpc.ChunkTypeProgress, rpc.ChunkTypeError}, "test", "TWF5IDIxIDA2OjQ3OjI2LjU1MzUzNQkbWzMzbVdBUk4bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmdXcml0ZUVycm9yIn0K"},
		{"WriteResult", []rpc.ChunkType{rpc.ChunkTypeResult}, "test", "test"},
		{"WriteBinary", []rpc.ChunkType{rpc.ChunkTypeBinary}, []uint8{1, 2, 3, 4}, "AQIDBA=="},
	}

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + test.f)
		// Call method with data. i.e. ow.Infof("test")
		reflect.ValueOf(ow).MethodByName(test.f).Call([]reflect.Value{reflect.ValueOf(test.param)})
		ow.Flush()

		res := rec.Result()

		testBody(t, &test, res.Body)
	}
}

// test writing directly to the writers.
func TestWriters(t *testing.T) {
	data := []byte("test")
	tcs := []testcase{
		{"BinaryWriter", []rpc.ChunkType{rpc.ChunkTypeBinary}, data, "dGVzdA=="},
		{"InfoWriter", []rpc.ChunkType{rpc.ChunkTypeProgress}, data, "TWF5IDIxIDA2OjMzOjIyLjEzNDI3NgkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdFdyaXRlcnNJbmZvV3JpdGVyIn0K"},
	}

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + test.f)
		// Get the io.Writer by name
		wtr := reflect.ValueOf(ow).MethodByName(test.f).Call([]reflect.Value{})[0]
		// Call its "Write()" method
		wtr.MethodByName("Write").Call([]reflect.Value{reflect.ValueOf(test.param)})
		ow.Flush()

		res := rec.Result()

		testBody(t, &test, res.Body)
	}
}
