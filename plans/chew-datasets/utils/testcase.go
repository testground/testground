package utils

import (
	"context"

	ipfsClient "github.com/ipfs/go-ipfs-api"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/testground/sdk/iptb"
	"github.com/ipfs/testground/sdk/runtime"
)

// TestCase represents a test case interface to be tested.
type TestCase interface {
	// AcceptFiles indicates if this test will accept files.
	AcceptFiles() bool

	// AcceptDirs indicates if this test will accept directories.
	AcceptDirs() bool

	// AddRepoOptions returns a function that modifies a repository
	// configuration to satisfy the test needs.
	AddRepoOptions() iptb.AddRepoOptions

	// Execute executes the test case with the given options.
	Execute(ctx context.Context, runenv *runtime.RunEnv, opts *TestCaseOptions) error
}

// TestCaseOptions are the options to pass to the test case execute function.
type TestCaseOptions struct {
	// Config is the test configuration.
	TestConfig

	// IpfsInstance is a connection to the in-process IPFS instance through Core API.
	IpfsInstance iface.CoreAPI

	// IpfsDaemon is a connection to a daemon instance of IPFS through IPTB.
	IpfsDaemon *ipfsClient.Shell
}
