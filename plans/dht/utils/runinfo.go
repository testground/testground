package utils

import (
	"github.com/testground/testground/sdk/runtime"
	"github.com/testground/testground/sdk/sync"
)

type RunInfo struct {
	RunEnv *runtime.RunEnv
	Client *sync.Client

	Groups          []string
	GroupProperties map[string]*GroupInfo
}
