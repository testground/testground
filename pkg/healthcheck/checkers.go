package healthcheck

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/rpc"

	"github.com/docker/docker/client"
)

// CheckContainerStarted returns a Checker that succeeds if a container is
// started, and fails otherwise.
func CheckContainerStarted(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string) Checker {
	return func() (bool, string, error) {
		ci, err := docker.CheckContainer(ctx, ow, cli, name)
		if err != nil || ci == nil {
			return false, "container not found.", err
		}
		return ci.State.Running, fmt.Sprintf("container state: %s", ci.State.Status), nil
	}
}

// CheckNetwork returns a Checker that succeeds if the specified network exists,
// and fails otherwise.
func CheckNetwork(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, networkID string) Checker {
	return func() (bool, string, error) {
		networks, err := docker.CheckBridgeNetwork(ctx, ow, cli, networkID)
		if err != nil {
			return false, "error when checking for network", err
		}
		if len(networks) > 0 {
			return true, "network exists.", nil
		}
		return false, "network does not exist.", nil
	}
}

// DialableChecker returns a Checker that checks whether a remote endpoint is
// dialable. For TCP sockets, a failure could mean the network is unreachable,
// or that the remote TCP socket is closed. For UDP sockets, being
// connectionless, may return a false positive even if there is no listening
// process, but at least it will try to find a route to the host.
func DialableChecker(protocol string, address string) Checker {
	return func() (bool, string, error) {
		_, err := net.Dial(protocol, address)
		if err != nil {
			return false, "address not dialable.", err
		}
		return true, "address is already dialable.", err
	}
}

// CheckDirectoryExists returns a Checker that checks whether the specified
// directory exists. It succeeds if the directory exists, and fails if the Go
// runtime returns a ErrNotExist error. All other errors are propagated back,
// which presumably will mark this check as aborted.
func CheckDirectoryExists(path string) Checker {
	return func() (bool, string, error) {
		fi, err := os.Stat(path)
		if err != nil {
			// ErrExist is the error we expect to see (and handle with CreateDirectory)
			// Any other kind of error will be returned.
			if os.IsNotExist(err) {
				return false, "directory does not exist. can recreate.", nil
			}
			return false, "filesystem error. cannot recreate.", err
		}
		if fi.IsDir() {
			return true, "directory exists.", nil
		}
		return false, "expected directory. found regular file. please fix manually.", fmt.Errorf("not a directory")
	}
}

// Always is a checker which always fails. Use this checker when a Fixer should always be executed.
func Always() Checker {
	return func() (bool, string, error) {
		return false, "always fix", nil
	}
}

// All returns a Checker that succeeds when all provided Checkers succeed.
// If a Checker fails, it short-circuits and returns the first failure.
func All(checkers ...Checker) Checker {
	return func() (bool, string, error) {
		for _, ckr := range checkers {
			ok, msg, err := ckr()
			if err != nil {
				return ok, msg, err
			}
		}
		return true, "all checks passed.", nil
	}
}

// Any returns a Checker that succeeds if any of the provided Checkers succeed.
// If none do, it fails.
func Any(checkers ...Checker) Checker {
	return func() (bool, string, error) {
		for _, ckr := range checkers {
			ok, msg, err := ckr()
			if err == nil {
				return ok, msg, err
			}
		}
		return false, "all checks failed.", fmt.Errorf("all checks failed")
	}
}

// Not negates a Checker. To make it clear to the user that the check is
// negated, the message is re-formatted using "NOT(<msg>)" notation.
//
// Any error encountered when the check is executed gets included in the message
// value.
//
// This is useful if there is a checker which does exactly the opposite of what
// you want to check for i.e. you want to check that a directory does NOT exist,
// etc. or you want to check for combinations using Any() or All() with some
// negated predicates.
func Not(ckr Checker) Checker {
	return func() (bool, string, error) {
		ok, msg, err := ckr()
		notmsg := fmt.Sprintf("NOT(%s)", msg)
		if ok {
			return false, notmsg, err
		}
		return true, notmsg, err
	}
}
