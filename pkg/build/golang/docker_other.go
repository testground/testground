//+build !linux,!darwin,!freebsd

package golang

func getOwner(path string) (owner string, err error) {
	return "", nil
}
