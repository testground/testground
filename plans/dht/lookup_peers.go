package dht

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	levelds "github.com/ipfs/go-ds-leveldb"
	"github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
)

type lookupPeersTC struct {
	Count      int
	BucketSize int
}

//var _ dht.DHTTestCase = (*lookupPeersTC)(nil)

func main() {
	_ = os.Setenv("TEST_CASE_SEQ", "1")
	_ = os.Setenv("TEST_PLAN", "dht")
	_ = os.Setenv("TEST_BRANCH", "master")
	_ = os.Setenv("TEST_CASE", "lookup_peers")
	_ = os.Setenv("TEST_TAG", "")
	_ = os.Setenv("TEST_RUN", uuid.New().String())

	tc := &lookupPeersTC{}
	tc.Execute()
}

func (tc *lookupPeersTC) Name() string {
	return fmt.Sprintf("lookup_peers-%dpeers-%dsize", tc.Count, tc.BucketSize)
}

func (tc *lookupPeersTC) Execute() {
	dir, err := ioutil.TempDir("", "dht")
	if err != nil {
		panic(err)
	}

	ds, err := levelds.NewDatastore(dir, nil)
	if err != nil {
		panic(err)
	}

	host, err := libp2p.New(context.Background())
	if err != nil {
		panic(err)
	}

	_, err = kaddht.New(context.Background(), host /*dhtopts.BucketSize(tc.BucketSize),*/, dhtopts.Datastore(ds))
	if err != nil {
		panic(err)
	}

	consul, err := capi.NewClient(capi.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// 1. Publish my multiaddrs.
	// 2. Subscribe to all multiaddrs as they appear.
	// 3. Connect to all multiaddrs.
	// 4. Run test.
	prefix := fmt.Sprintf("run/%s/plan/%s/case/%s",
		os.Getenv("TEST_RUN"),
		os.Getenv("TEST_PLAN"),
		os.Getenv("TEST_CASE"))

	key := fmt.Sprintf("%s/nodes/%s/addrs",
		prefix,
		host.ID().Pretty())

	w, err := watch.Parse(map[string]interface{}{
		"type":   "keyprefix",
		"prefix": prefix,
	})

	if err != nil {
		panic(err)
	}

	w.Handler = func(i uint64, v interface{}) {
		fmt.Println(i)
		fmt.Println(v)
	}

	log := log.New(os.Stdout, "watch", log.LstdFlags)
	go w.RunWithClientAndLogger(consul, log)

	capi.DefaultConfig().GenerateEnv()

	go func() {
		var i int
		for {
			addrs, err := json.Marshal(host.Addrs())
			if err != nil {
				panic(err)
			}
			i++
			entry := capi.KVPair{Key: fmt.Sprintf("%s-%d", key, i), Value: addrs}
			if _, err := consul.KV().Put(&entry, nil); err != nil {
				panic(err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	fmt.Println("hello")
	time.Sleep(10 * time.Minute)

}
