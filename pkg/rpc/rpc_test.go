package rpc_test

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/rpc/rpctest"
)

type testcase struct {
	f        string
	ct       []rpc.ChunkType
	param    interface{}
	expected interface{}
}

func allowedType(ch rpc.Chunk, cts []rpc.ChunkType) bool {
	for _, tp := range cts {
		if ch.Type == tp {
			return true
		}
	}
	return false
}

func verifyExpected(t *testing.T, ch rpc.Chunk, expected interface{}) {
	switch ch.Type {
	// base64-encoded strings that look like this when decoded:
	// May 19 09:51:43.022537	INFO	test	{"req_id": "TestLogging-Infof"}
	// Check the data after the timestamp matches expected.
	case rpc.ChunkTypeProgress:
		t.Log(ch.Type, ch.Payload)
		if !strings.Contains(ch.Payload.(string), expected.(string)[30:]) {
			t.Log("ChunkTypeProgress data did not match what was expected.", ch.Payload)
			t.Fail()
		}

	// string or []uint8
	case rpc.ChunkTypeResult, rpc.ChunkTypeBinary:
		if ch.Payload != expected {
			t.Log("data did not match what was expected.", ch.Payload)
			t.Fail()
		}

	case rpc.ChunkTypeError:
		t.Log("", ch.Payload)
	}
}

func TestLogging(t *testing.T) {

	// Info and Warn send.
	// Debug does not send to the client
	// Fatal causes a crash. Just check the ones that send to the client.

	tcs := []testcase{
		{"Info", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDE5IDA5OjUxOjQzLjAyMjI3OQkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctSW5mbyJ9Cg=="},
		{"Infof", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDE5IDA5OjUxOjQzLjAyMjUzNwkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctSW5mb2YifQo="},
		{"Infow", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDE5IDA5OjUxOjQzLjAyMjY1MAkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctSW5mb3cifQo="},
		{"Error", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDE5IDA5OjUxOjQzLjAyMjc1MQkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nLUVycm9yIn0K"},
		{"Errorf", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDE5IDA5OjUxOjQzLjAyMjg1MwkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nLUVycm9yZiJ9Cg=="},
		{"Errorw", []rpc.ChunkType{rpc.ChunkTypeProgress}, "test", "TWF5IDE5IDA5OjUxOjQzLjAyMjk1NQkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nLUVycm9ydyJ9Cg=="},
		{"WriteError", []rpc.ChunkType{rpc.ChunkTypeProgress, rpc.ChunkTypeError}, "test", "TWF5IDIwIDIzOjQ1OjQxLjM4NjY2OQkbWzMzbVdBUk4bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctV3JpdGVFcnJvciJ9Cg=="},
		{"WriteResult", []rpc.ChunkType{rpc.ChunkTypeResult}, "test", "test"},
		{"WriteBinary", []rpc.ChunkType{rpc.ChunkTypeBinary}, []uint8{1, 2, 3, 4}, "AQIDBA=="},
	}

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + "-" + test.f)
		reflect.ValueOf(ow).MethodByName(test.f).Call([]reflect.Value{reflect.ValueOf(test.param)})
		ow.Flush()

		res := rec.Result()

		dec := json.NewDecoder(res.Body)
		for dec.More() {
			var ch rpc.Chunk
			err := dec.Decode(&ch)
			if err != nil {
				t.Fatal(err)
			}
			t.Run("Test Logging: "+test.f, func(t *testing.T) {
				// Check chunk type
				if !allowedType(ch, test.ct) {
					t.Log("incorrect ChunkType", string(ch.Type))
					t.Fail()
				}

				verifyExpected(t, ch, test.expected)
			})
		}
	}
}

// Test that content serialized through the writers is received with the correct ChunkType and
// content is expected.
func TestWriters(t *testing.T) {
	data := []byte("test")
	tcs := []testcase{
		// expect encoded raw binary
		{"BinaryWriter", []rpc.ChunkType{rpc.ChunkTypeBinary}, data, "dGVzdA=="},
		// expect encoded zap logger output
		{"InfoWriter", []rpc.ChunkType{rpc.ChunkTypeProgress}, data, "TWF5IDE1IDA3OjE5OjI4LjY5OTIzNAkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdFdyaXRlcnMtSW5mb1dyaXRlciJ9Cg=="},
	}

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + "-" + test.f)
		// Get the io.Writer by name
		wtr := reflect.ValueOf(ow).MethodByName(test.f).Call([]reflect.Value{})[0]
		// Call its "Write()" method
		wtr.MethodByName("Write").Call([]reflect.Value{reflect.ValueOf(test.param)})
		ow.Flush()
		res := rec.Result()

		// Unmarshal body
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		var ch rpc.Chunk
		err = json.Unmarshal(body, &ch)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		t.Run("Writer-"+test.f, func(t *testing.T) {
			if !allowedType(ch, test.ct) {
				t.Log("incorrect ChunkType", string(ch.Type))
				t.Fail()
			}
			verifyExpected(t, ch, test.expected)
		})
	}
}
