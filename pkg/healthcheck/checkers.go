package healthcheck

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
)


// DefaultContainerChecker returns a Checker, a method which when executed will check for the
// existance of the container. This should be considered a sensible default for checking whether
// docker containers are started.
func DefaultContainerChecker(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string) Checker {
	return func() (bool, string, error) {
		ci, err := docker.CheckContainer(ctx, ow, cli, name)
		if err != nil || ci == nil {
			return false, "container not running.", err
		}
		return ci.State.Running, "container already running.", nil
	}
}

// DockerNetworkChecker returns a Checker, a method which when executed will verify a docker network
// exists with the passed networkID as its name.
func DockerNetworkChecker(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string) Checker {
	return func() (bool, string, error) {
		networks, err := docker.CheckBridgeNetwork(ctx, ow, cli, networkID)
		if err != nil {
			return false, "error when checking for network", err
		}
		if len(networks) > 0 {
			return true, "network already exists.", nil
		}
		return false, "network does not exist.", nil
	}
}

// DialableChecker returns a Checker, a method which when executed will tell us whether a
// port is dialable. For TCP sockets, a false return could mean the network is unreachable,
// or that a TCP socket is closed. For UDP sockets, being connectionless, may return a false
// positive if the network is reachable.
func DialableChecker(protocol string, address string) Checker {
	return func() (bool, string, error) {
		_, err := net.Dial(protocol, address)
		if err != nil {
			return false, "address not dialable.", err
		}
		return true, "address is already dialable.", err
	}
}

// DirExistsChecker returns a Checker, a method which when executed will check whether a director
// exists. A true value means the directory exists. A false value means it does not exist, or
// that the path does not point to a directory. Aside from ErrNotExist, which is the error we expect
// to handle, any file permission or I/O errors will will be returned to the caller.
func DirExistsChecker(path string) Checker {
	return func() (bool, string, error) {
		fi, err := os.Stat(path)
		if err != nil {
			// ErrExist is the error we expect to see (and handle with DirExistsFixer)
			// Any other kind of error will be returned.
			if os.IsNotExist(err) {
				return false, "directory does not exist. can recreate.", nil
			}
			return false, "filesystem error. cannot recreate.", err
		}
		if fi.IsDir() {
			return true, "directory already exists.", nil
		}
		return false, "expected directory. found regular file. please fix manually.", fmt.Errorf("not a directory")
	}