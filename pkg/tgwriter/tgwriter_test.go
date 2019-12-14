package tgwriter

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestProgress(t *testing.T) {
	w := httptest.NewRecorder()
	log := zap.NewNop().Sugar()
	tgw := New(w, log)

	_, err := tgw.Write([]byte("testground"))
	if err != nil {
		t.Fatal(err)
	}

	// make sure we wrote the message
	tgw.Flush()

	buf := new(bytes.Buffer)
	res := w.Result()
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	msg := &Msg{}
	err = json.Unmarshal(buf.Bytes(), msg)
	if err != nil {
		t.Fatal(err)
	}

	if msg.Type != "progress" {
		t.Fatal(errors.New("message type must be 'progress'"))
	}

	txt, ok := msg.Payload.(string)
	if !ok {
		t.Fatal(errors.New("message not ok"))
	}

	m, err := base64.StdEncoding.DecodeString(txt)
	if err != nil {
		t.Fatal(err)
	}

	if string(m) != "testground" {
		t.Fatal(errors.New("message not ok"))
	}
}

func TestResult(t *testing.T) {
	w := httptest.NewRecorder()
	log := zap.NewNop().Sugar()
	tgw := New(w, log)

	tgw.WriteResult([]byte("testground"))

	// make sure we wrote the message
	tgw.Flush()

	buf := new(bytes.Buffer)
	res := w.Result()
	_, err := buf.ReadFrom(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	msg := &Msg{}
	err = json.Unmarshal(buf.Bytes(), msg)
	if err != nil {
		t.Fatal(err)
	}

	if msg.Type != "result" {
		t.Fatal(errors.New("message type must be 'result'"))
	}

	txt, ok := msg.Payload.(string)
	if !ok {
		t.Fatal(errors.New("message not ok"))
	}

	m, err := base64.StdEncoding.DecodeString(txt)
	if err != nil {
		t.Fatal(err)
	}

	if string(m) != "testground" {
		t.Fatal(errors.New("message not ok"))
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	log := zap.NewNop().Sugar()
	tgw := New(w, log)

	tgw.WriteError("testground")

	// make sure we wrote the message
	tgw.Flush()

	buf := new(bytes.Buffer)
	res := w.Result()
	_, err := buf.ReadFrom(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	msg := &Msg{}
	err = json.Unmarshal(buf.Bytes(), msg)
	if err != nil {
		t.Fatal(err)
	}

	if msg.Type != "error" {
		t.Fatal(errors.New("message type must be 'error'"))
	}

	if msg.Error.Message != "testground" {
		t.Fatal(errors.New("message not ok"))
	}
}
