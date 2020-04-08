package utils

import (
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

type RunInfo struct {
	RunEnv  *runtime.RunEnv
	Watcher *sync.Watcher
	Writer  *sync.Writer

	Groups          []string
	GroupProperties map[string]*GroupInfo
}
