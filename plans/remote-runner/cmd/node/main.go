package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

func main() {
	run.InvokeMap(testcases)
}

var testcases = map[string]interface{}{
	"server-client": run.InitializedTestCaseFn(ServerClientTestCase),
}

func ServerClientTestCase(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	ctx := context.Background()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	libp2pTest := runenv.BooleanParam("libp2p_test")
	enrolledState := sync.State("enrolled")

	seq := client.MustSignalEntry(ctx, enrolledState)

	if libp2pTest {
		fmt.Println("Testing libp2p")
		done := sync.State("done")
		err := libp2pPing(ctx, client, int(seq))
		if err != nil {
			runenv.RecordFailure(err)
		} else {
			runenv.RecordSuccess()
		}
		client.MustSignalEntry(ctx, done)
		<-client.MustBarrier(ctx, done, runenv.TestInstanceCount).C
		return nil
	} else {
		fmt.Println("Testing http")
		serverReady := sync.State("serverReady")
		respRecv := sync.State("responseReceived")

		switch seq {
		case 1:
			// First node is the server
			var server http.Server
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("pong"))
			})
			server.Addr = ":8188"
			go func() {
				time.Sleep(time.Second)
				client.MustSignalEntry(ctx, serverReady)
				<-client.MustBarrier(ctx, respRecv, runenv.TestInstanceCount-1).C
				server.Close()
			}()
			server.ListenAndServe()

		default:
			// every other node is a client
			<-client.MustBarrier(ctx, serverReady, 1).C
			var httpClient http.Client
			remoteIP := runenv.StringParam("server_ip")
			resp, err := httpClient.Get("http://" + remoteIP + ":8188/")
			if err != nil {
				return err
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			client.MustSignalEntry(ctx, respRecv)

			if string(body) != "pong" {
				return fmt.Errorf("didn't read pong")
			}
		}
		runenv.RecordSuccess()
		return nil
	}
}

type syncClient interface {
	Subscribe(ctx context.Context, topic *sync.Topic, ch interface{}) (sub *sync.Subscription, err error)
	Publish(ctx context.Context, topic *sync.Topic, payload interface{}) (int64, error)
}

func libp2pPing(ctx context.Context, client syncClient, enrolledNumber int) error {
	st := sync.NewTopic("transfer-key", peer.AddrInfo{})

	switch enrolledNumber {
	case 1:
		// All nodes will interact with this node
		h, err := libp2p.New()
		if err != nil {
			return err
		}
		ai := peer.AddrInfo{
			ID:    h.ID(),
			Addrs: h.Addrs(),
		}
		client.Publish(ctx, st, ai)
	default:
		h, err := libp2p.New()
		if err != nil {
			return err
		}

		aiCh := make(chan peer.AddrInfo)
		_, err = client.Subscribe(ctx, st, aiCh)
		if err != nil {
			return err
		}

		node1ai := <-aiCh

		err = h.Connect(ctx, node1ai)
		if err != nil {
			return err
		}
		res := <-ping.Ping(ctx, h, node1ai.ID)
		if res.Error != nil {
			return res.Error
		}
	}
	return nil
}
