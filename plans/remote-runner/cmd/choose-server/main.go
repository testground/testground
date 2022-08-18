package main

import (
	"context"
	"fmt"
	"os"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

type RemoteRunnerConfig struct {
	Name string
	Arch string // Can be omitted if name is localhost
}

func main() {
	runenv := runtime.CurrentRunEnv()
	ctx := context.Background()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	chooseServerState := sync.State("choose_server")
	seq := int(client.MustSignalEntry(ctx, chooseServerState)) - 1 // We want a 0 indexed seq.

	var remoteRunners []RemoteRunnerConfig
	runenv.JSONParam("remote-runners", &remoteRunners)

	if len(remoteRunners) == 0 {
		panic("No remote runners config available")
	}

	if seq > len(remoteRunners) {
		fmt.Fprintf(os.Stderr, "More runners than remote runner configs. Defaulting the rest of the to the runners to use the first remote runner config\n")
		seq = 0

	}

	fmt.Printf("%s,%s\n", remoteRunners[seq].Name, remoteRunners[seq].Arch)
}
