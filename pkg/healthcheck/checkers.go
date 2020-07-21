package healthcheck

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/testground/testground/pkg/docker"
	"github.com/testground/testground/pkg/rpc"

	"github.com/docker/docker/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

// CheckCommandStatus returns a checker which executes a command and returns successfully or
// unsuccessfully depending on the exit status of the command.
func CheckCommandStatus(ctx context.Context, cmd string, args ...string) Checker {
	return func() (bool, string, error) {
		cmd := exec.CommandContext(ctx, cmd, args...)
		err := cmd.Start()
		if err != nil {
			// for example, command does not exist. This is an error with the check.
			return false, fmt.Sprintf("failed to start command. `%s` (%v)", cmd, err), err
		}
		err = cmd.Wait()
		if err != nil {
			// command returns with a non-zero exit code. Return nil error, and false for the check.
			return false, fmt.Sprintf("command failed `%s` (%v)", cmd, err), nil
		}
		// command returns with a zero exit code.
		return true, fmt.Sprintf("command completed successfully `%s`", cmd), nil
	}
}

// CheckK8sPods returns a checker which verifies the number of pods found matches the number
// expected. If Listing the pods returns an error, the error is returned. The boolean value returned
// by the check follows whether the number of pods observed in the list matches the expected count.
func CheckK8sPods(ctx context.Context, client *kubernetes.Clientset, label string, namespace string, count int) Checker {
	return func() (bool, string, error) {
		listOpts := metav1.ListOptions{LabelSelector: label}
		pods, err := client.CoreV1().Pods(namespace).List(listOpts)
		if err != nil {
			return false, fmt.Sprintf("failed to list pods %s", label), err
		}
		found := len(pods.Items)
		msg := fmt.Sprintf("expected %d pods; found %d", count, found)
		if found != count {
			return false, msg, nil
		}
		return true, msg, nil
	}
}

// CheckRedisPort returns a checker which verifies if the default port of redis (6379) is already binded
// on localhost. If it is, it fails. If not, it succeeds.
func CheckRedisPort() Checker {
	return func() (bool, string, error) {
		ln, err := net.Listen("tcp", "localhost:6379")
		if err != nil {
			return false, "local port 6379 is already occupied; please stop any local Redis instances first.", nil
		}
		ln.Close()
		return true, "local port 6379 is free.", nil
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
