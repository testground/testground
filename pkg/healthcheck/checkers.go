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

// Checker is a function that checks whether a precondition is met. It returns
// whether the check succeeded, an optional message to present to the user, and
// error in case the check logic itself failed.
//
//   (true, *, nil) => HealthcheckStatusOK
//   (false, *, nil) => HealthcheckStatusFailed
//   (false, *, not-nil) => HealthcheckStatusAborted
//   checker doesn't run => HealthcheckStatusOmitted (e.g. dependent checks where the upstream failed)
type Checker func() (ok bool, msg string, err error)

// DefaultContainerChecker returns a Checker, a method which when executed will check for the
// existance of the container. This should be considered a sensible default for checking whether
// docker containers are started.
func DockerContainerChecker(ctx context.Context, ow *rpc.OutputWriter, cli *client.Client, name string) Checker {
	return func() (bool, string, error) {
		ci, err := docker.CheckContainer(ctx, ow, cli, name)
		if err != nil || ci == nil {
			return false, "container not found.", err
		}
		return ci.State.Running, fmt.Sprintf("container state %s", ci.State.Status), nil
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
}

// All returns a Checker. This method takes checkers for its parameters, When the returned Checkeri
// executed, it will return a successful result only when all parameter checkers return successfully.
// It returns with a failure when the first failure is encountered. Use this when there are multiple
// prerequisites which should be checked as a group and fixed with a single Fixer.
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

// Any returns a Checker. This method takes checkers for its parameters. When the returned Checker is
// executed, it will and return with a successful result so long as any of the passed checkers are
// successful. This method returns a successfully when the first success is encountered. Use when
// there are multiple satisfactory conditions such that a fixer should not be executed so long as
// one of them is working.
func Any(checkers ...Checker) Checker {
	return func() (bool, string, error) {
		for _, ckr := range checkers {
			ok, msg, err := ckr()
			if err == nil {
				return ok, msg, err
			}
		}
		return false, "all checks failed.", fmt.Errorf("all checks failed.")
	}
}

// Not returns a checker which flips the boolean value of the passed Checker. To make it clear to
// the user that the check is negated, the message is re-formatted using "NOT(<msg>)" notation.
// Any error encountered when the check is executed gets included in the return value.
// This is useful if there is a checker which does exactly the opposite of what you want to check
// for i.e. you want to check that a directory does NOT exist, etc. or you want to check for
// combinations using Any() or All() with some negated predicates.
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
