package utils

import (
	"bytes"
	"io"
	"math/rand"
)

var randReader *rand.Rand

func RandReader(len int) io.Reader {
	if randReader == nil {
		randReader = rand.New(rand.NewSource(2))
	}
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}
