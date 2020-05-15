package rpc_test

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/rpctest"
)

func TestProgress(t *testing.T) {

	// Info and Warn send.
	// Debug does not send to the client
	// Fatal causes a crash. Just check the ones that send to the client.

	tcs := []string{"Info", "Infof", "Warn", "Warnf"}
	data := "test test test"

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + "-" + test)
		reflect.ValueOf(ow).MethodByName(test).Call([]reflect.Value{reflect.ValueOf(data)})
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

		// Check chunk type
		if ch.Type != rpc.ChunkTypeProgress {
			t.Log("incorrect ChunkType")
			t.Fail()
		}
	}
}

// Test that content serialized through the writers is received with the correct ChunkType and
// content is expected.
func TestWriters(t *testing.T) {
	type tc struct {
		w        string
		ct       rpc.ChunkType
		expected string
	}
	tcs := []tc{
		{"BinaryWriter", rpc.ChunkTypeBinary, "dGVzdA=="},
		{"InfoWriter", rpc.ChunkTypeProgress, "TWF5IDE1IDA3OjE5OjI4LjY5OTIzNAkbWzM0bUlORk8bWzBtCXRlc3QJeyJyZXFfaWQiOiAiVGVzdFdyaXRlcnMtSW5mb1dyaXRlciJ9Cg=="},
	}
	data := []byte("test")

	for _, test := range tcs {
		rec, ow := rpctest.NewRecordedOutputWriter(t.Name() + "-" + test.w)
		// Get the io.Writer by name
		wtr := reflect.ValueOf(ow).MethodByName(test.w).Call([]reflect.Value{})[0]
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

		// Check chunk type
		if ch.Type != test.ct {
			t.Log("incorrect ChunkType")
			t.Fail()
		}
		p := ch.Payload.(string)
		if test.expected != p {
			t.Log("incorrect content")
			t.Log(p)
			t.Fail()
		}
	}
}
