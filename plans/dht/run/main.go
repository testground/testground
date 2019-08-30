package main

import (
	"os"

	"github.com/google/uuid"
	"github.com/ipfs/testground/plans/dht"
)

func main() {
	_ = os.Setenv("TEST_CASE_SEQ", "1")
	_ = os.Setenv("TEST_PLAN", "dht")
	_ = os.Setenv("TEST_BRANCH", "master")
	_ = os.Setenv("TEST_CASE", "lookup_peers")
	_ = os.Setenv("TEST_TAG", "")
	_ = os.Setenv("TEST_RUN", uuid.New().String())

	tc := &dht.LookupPeersTC{}
	tc.Execute()
}
