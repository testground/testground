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

	// InstanceOptions returns the options needed to build a runtime instance
	// of IPFS through the Core API.
	InstanceOptions() *IpfsInstanceOptions

	// DaemonOptions returns the options (ensemble) needed to build a daemon instance
	// of IPFS through IPTB.
	DaemonOptions() *iptb.TestEnsembleSpec

	// Execute executes the test case with the given options.
	Execute(ctx context.Context, runenv *runtime.RunEnv, opts *TestCaseOptions)
}

// TestCaseOptions are the options to pass to the test case execute function.
type TestCaseOptions struct {
	// IpfsInstance is a connection to the in-process IPFS instance through Core API.
	IpfsInstance iface.CoreAPI

	// IpfsDaemon is a connection to a daemon instance of IPFS through IPTB.
	IpfsDaemon *ipfsClient.Shell

	// Config is the test configuration.
	Config TestConfig
}
