package docker

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
)

// PipeOutput pipes a reader that spits out jsonmessage structs into a writer,
// usually stdout. It returns normally when the reader is exhausted, or in error
// if one occurs.
func PipeOutput(r io.ReadCloser, w io.Writer) error {
	var msg jsonmessage.JSONMessage
Loop:
	for dec := json.NewDecoder(r); ; {
		switch err := dec.Decode(&msg); err {
		case nil:
			_ = msg.Display(w, false)
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
