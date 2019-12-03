package tgwriter

import (
	"encoding/json"
	"io"
)

func New(w io.Writer) *TgWriter {
	return &TgWriter{
		output: w,
	}
}

type TgWriter struct {
	io.Writer
	output io.Writer
}

// Msg defines a protocol message struct sent from the Testground daemon to the Testground client.
// For a given request, clients should expect between 1 and `n` `progress` messages, and
// exactly 1 `result` message.
type Msg struct {
	Type    string      `json:"type"` // progress or result or error
	Payload interface{} `json:"payload,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Message string `json:"message"`
}

func (tgw *TgWriter) Write(p []byte) (n int, err error) {
	pld := Msg{
		Type:    "progress",
		Payload: p,
	}

	json, err := json.Marshal(pld)
	if err != nil {
		return 0, err
	}

	return tgw.output.Write(json)
}

func (tgw *TgWriter) WriteResult(res interface{}) error {
	pld := Msg{
		Type:    "result",
		Payload: res,
	}

	json, err := json.Marshal(pld)
	if err != nil {
		return err
	}

	_, err = tgw.output.Write(json)

	return err
}

func (tgw *TgWriter) WriteError(message string) error {
	pld := Msg{
		Type: "error",
		Error: &Error{
			Message: message,
		},
	}

	json, err := json.Marshal(pld)
	if err != nil {
		return err
	}

	_, err = tgw.output.Write(json)

	return err
}
