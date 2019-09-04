package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ipfs/testground/plans/dht"
	"github.com/logrusorgru/aurora"
)

func main() {
	_ = os.Setenv("TEST_CASE_SEQ", "1")
	_ = os.Setenv("TEST_PLAN", "dht")
	_ = os.Setenv("TEST_BRANCH", "master")
	_ = os.Setenv("TEST_CASE", "lookup_peers")
	_ = os.Setenv("TEST_TAG", "")
	_ = os.Setenv("TEST_RUN", "1")

	var tcs []*dht.LookupPeersTC
	for i := 0; i < 100; i++ {
		fmt.Println(aurora.Sprintf(aurora.Blue(aurora.Bold("starting instance: %d")), i))
		tc := &dht.LookupPeersTC{Instance: i}
		tcs = append(tcs, tc)
		go tc.Execute()
	}

	for range time.Tick(5 * time.Second) {
		for _, tc := range tcs {
			fmt.Println(aurora.Sprintf(aurora.Yellow(aurora.Italic("instance %d: received %d events")), tc.Instance, tc.EventsReceived))
		}
		fmt.Println("-----------")
	}

}
