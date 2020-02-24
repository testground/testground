package main

import (
	"fmt"

	"github.com/ipfs/testground/sdk/runtime"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	return fmt.Errorf("Placeholder binary for lotus-base")
}
