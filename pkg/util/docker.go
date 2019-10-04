package util

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
)

func PipeDockerOutput(r io.ReadCloser, w io.Writer) error {
	var msg jsonmessage.JSONMessage
Loop:
	for dec := json.NewDecoder(r); ; {
		switch err := dec.Decode(&msg); err {
		case nil:
			msg.Display(w, true)
			if msg.Error != nil {
				return msg.Error
			}
		case io.EOF:
			break Loop
		default:
			return err
		}
	}
	return nil
}
