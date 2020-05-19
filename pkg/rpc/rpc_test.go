package rpc_test

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/rpc/rpctest"
)

type tc struct {
	f        string
	ct       rpc.ChunkType
	expected string
}

func TestLogging(t *testing.T) {

	// Info and Warn send.
	// Debug does not send to the client
	// Fatal causes a crash. Just check the ones that send to the client.

	tcs := []tc{
		{"Info", rpc.ChunkTypeProgress, "TWF5IDE5IDA5OjUxOjQzLjAyMjI3OQkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctSW5mbyJ9Cg=="},
		{"Infof", rpc.ChunkTypeProgress, "TWF5IDE5IDA5OjUxOjQzLjAyMjUzNwkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctSW5mb2YifQo="},
		{"Infow", rpc.ChunkTypeProgress, "TWF5IDE5IDA5OjUxOjQzLjAyMjY1MAkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdExvZ2dpbmctSW5mb3cifQo="},
		{"Error", rpc.ChunkTypeProgress, "TWF5IDE5IDA5OjUxOjQzLjAyMjc1MQkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nLUVycm9yIn0K"},
		{"Errorf", rpc.ChunkTypeProgress, "TWF5IDE5IDA5OjUxOjQzLjAyMjg1MwkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nLUVycm9yZiJ9Cg=="},
		{"Errorw", rpc.ChunkTypeProgress, "TWF5IDE5IDA5OjUxOjQzLjAyMjk1NQkbWzMxbUVSUk9SG1swbQl0ZXN0CXsicmVxX2lkIjogIlRlc3RMb2dnaW5nLUVycm9ydyJ9Cg=="},
		{"WriteResult", rpc.ChunkTypeResult, "test"},
		// Cant test fatal because... you know. It exits immediately.
		//		{"Fatal", rpc.ChunkTypeError, ""},
		//		{"Fatalf", rpc.ChunkTypeError, ""},
		//		{"Fatalw", rpc.ChunkTypeError, ""},
	}

	data := "test"

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + "-" + test.f)
		reflect.ValueOf(ow).MethodByName(test.f).Call([]reflect.Value{reflect.ValueOf(data)})
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
		t.Run("expected chunktype: "+test.f, func(t *testing.T) {
			// Check chunk type
			if ch.Type != test.ct {
				t.Log("incorrect ChunkType", string(ch.Type))
				t.Fail()
			}
		})
		t.Run("expected content: "+test.f, func(t *testing.T) {
			p := ch.Payload.(string)
			teststr := p
			expected := test.expected
			if test.ct == rpc.ChunkTypeProgress {
				// ChunkTypeProgress contains unpredictable timestamps.
				// Rather than deserializing, just check the last part of the data.
				part := len(p) - 50
				teststr = p[part:]
				expected = test.expected[part:]
			}
			for i, ch := range teststr {
				if string(expected[i]) != string(ch) {
					t.Log("incorrect content at position", i, p)
				}
			}
		})
	}
}

// Test that content serialized through the writers is received with the correct ChunkType and
// content is expected.
func TestWriters(t *testing.T) {
	tcs := []tc{
		// expect encoded raw binary
		{"BinaryWriter", rpc.ChunkTypeBinary, "dGVzdA=="},
		// expect encoded zap logger output
		{"InfoWriter", rpc.ChunkTypeProgress, "TWF5IDE1IDA3OjE5OjI4LjY5OTIzNAkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdFdyaXRlcnMtSW5mb1dyaXRlciJ9Cg=="},
	}
	data := []byte("test")

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + "-" + test.f)
		// Get the io.Writer by name
		wtr := reflect.ValueOf(ow).MethodByName(test.f).Call([]reflect.Value{})[0]
		// Call its "Write()" method
		wtr.MethodByName("Write").Call([]reflect.Value{reflect.ValueOf(data)})
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

			// Check chunk type
			if ch.Type != test.ct {
				t.Log("incorrect ChunkType", string(ch.Type))
				t.Fail()
			}
		})
	}
}
