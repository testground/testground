package rpc_test

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/testground/testground/pkg/rpc"
	"github.com/testground/testground/pkg/rpctest"
)

func TestInfo(t *testing.T) {
	rec, ow := rpctest.NewRecordedOutputWriter(t.Name())

	// Record Info
	ow.Info("test test test")
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

	// Check chunk contents.
	if ch.Type != rpc.ChunkTypeProgress {
		t.Log("incorrect ChunkType")
		t.Fail()
	}
	if ch.Payload == nil || len(ch.Payload.(string)) <= 0 {
		t.Log("incorrect Payload")
		t.Fail()
	}

}

func TestBinary(t *testing.T) {
	rec, ow := rpctest.NewRecordedOutputWriter(t.Name())
	payload := []byte("test")

	// Send binary data
	_, err := ow.BinaryWriter().Write(payload)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
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

	// Check chunk contents
	if ch.Type != rpc.ChunkTypeBinary {
		t.Log("incorrect ChunkType")
		t.Fail()
	}
	if ch.Payload == nil || ch.Payload.(string) != "dGVzdA==" {
		t.Log("nil Payload")
		t.Fail()
	}
}
