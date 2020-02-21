//+build linux darwin freebsd

package golang

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func getOwner(path string) (owner string, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	stat := info.Sys().(unix.Stat_t)
	return fmt.Sprintf("%d:%d", stat.Uid, stat.Gid), nil
}
