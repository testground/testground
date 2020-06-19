package docker

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
)

// PipeOutput pipes a reader that spits out jsonmessage structs into a writer,
// usually stdout. It returns normally when the reader is exhausted, or in error
// if one occurs. In both cases, it will return the accumulated string
// representation of the output.
func PipeOutput(r io.ReadCloser, w io.Writer) (output string, err error) {
	var (
		msg   jsonmessage.JSONMessage
		buf   = new(bytes.Buffer)
		multi = io.MultiWriter(buf, w)
	)

Loop:
	for dec := json.NewDecoder(r); ; {
		switch err := dec.Decode(&msg); err {
		case nil:
			_ = msg.Display(multi, false)
			if msg.Error != nil {
				return buf.String(), msg.Error
			}
		case io.EOF:
			break Loop
		default:
			return buf.String(), err
		}
	}
	return buf.String(), nil
}
