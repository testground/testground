//+build !linux

package sidecar

import (
	"errors"
)

func Run(_ string) error {
	return errors.New("the sidecar must be run from within a Linux host")
}
