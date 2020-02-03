//+build !linux

package sidecar

import (
	"errors"
)

func GetRunners() []string {
	return nil
}

func Run(_, _ string) error {
	return errors.New("the sidecar must be run from within a Linux host")
}
