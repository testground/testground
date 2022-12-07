package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

var testcases = map[string]interface{}{
	"issue-1349-silent-failure":                silentFailure,
	"issue-1493-success":                       run.InitializedTestCaseFn(success),
	"issue-1493-optional-failure":              run.InitializedTestCaseFn(optionalFailure),
	"issue-1488-latency-not-working-correctly": run.InitializedTestCaseFn(verifyRTT),
}

func main() {
	run.InvokeMap(testcases)
}

func silentFailure(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("This fails by NOT returning an error and NOT sending a test success status.")
	return nil
}

func success(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	runenv.RecordMessage("success!")
	return nil
}

func optionalFailure(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	shouldFail := runenv.BooleanParam("should_fail")
	runenv.RecordMessage("Test run with shouldFail: %s", shouldFail)

	if shouldFail {
		return errors.New("failing as requested")
	}

	return nil
}

func verifyRTT(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	latency := 25 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()


	client := initCtx.SyncClient
	netclient := initCtx.NetClient

	// Configure the network
	config := &network.Config{
		Network: "default",
		Enable:  true,
		Default: network.LinkShape{
			Latency:   latency,
		},
		CallbackState: sync.State("network-configured"),
		CallbackTarget: runenv.TestInstanceCount,
		// RoutingPolicy: network.DenyAll,
	}

	netclient.MustConfigureNetwork(ctx, config)

	// Find my IP address
	myIp, err := netclient.GetDataNetworkIP()
	if err != nil {
		return err
	}

	// Exhange IPs with the other instances
	peersTopic := sync.NewTopic("peers", new(net.IP))
	seq := client.MustPublish(ctx, peersTopic, myIp)

	peersCh := make(chan net.IP)
	peers := make([]net.IP, 0, runenv.TestInstanceCount)
	sub := initCtx.SyncClient.MustSubscribe(ctx, peersTopic, peersCh)

	// Wait for the other instance to publish their IP address
	for len(peers) < runenv.TestInstanceCount {
		select {
		case err := <-sub.Done():
			return err
		case ip := <-peersCh:
			peers = append(peers, ip)
		}
	}

	serverReadyState := sync.State("server-ready")
	clientDoneState := sync.State("client-done")

	if seq == 1 {
		// Setting up the server
		listener, err := net.ListenTCP("tcp4", &net.TCPAddr{Port: 1234})
		if err != nil {
			return err
		}
		defer listener.Close()

		go server(runenv, listener)

		runenv.RecordMessage("server is ready")
		client.MustSignalAndWait(ctx, serverReadyState, runenv.TestInstanceCount)
		runenv.RecordMessage("waiting for clients to be done")
		client.MustSignalAndWait(ctx, clientDoneState, runenv.TestInstanceCount)
		return nil
	} else {
		// Setting up the client
		serverIp := peers[0]

		runenv.RecordMessage("waiting for server to be ready")
		client.MustSignalAndWait(ctx, serverReadyState, runenv.TestInstanceCount)

		// Connect to the server
		runenv.RecordMessage("Attempting to connect to %s", serverIp)
		conn, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
			IP:   serverIp,
			Port: 1234,
		})
		if err != nil {
			return err
		}
		defer conn.Close()

		// disable Nagle's algorithm to measure latency.
		err = conn.SetNoDelay(true)
		if err != nil {
			return err
		}

		buf := make([]byte, 1)
		deltas := make([]time.Duration, 0, 3)

		// Send 3 ping to the server
		for i := 0; i < 10; i++ {
			start := time.Now()

			// Send the ping
			conn.Write([]byte{byte(i)})

			// Receive the pong
			_, err := conn.Read(buf)
			if err != nil {
				return err
			}

			end := time.Now()

			// append to the array of deltas
			deltas = append(deltas, end.Sub(start))
		}

		// Record rtts:
		runenv.RecordMessage("RTTs: %v", deltas)
		// Record min, max, avg RTTs
		min, max, avg := Summarize(deltas)
		runenv.RecordMessage("min: %v, max: %v, avg: %v", min, max, avg)

		client.MustSignalAndWait(ctx, clientDoneState, runenv.TestInstanceCount)

		if max > 25 * 2 * 1.1 * time.Millisecond {
			return fmt.Errorf("max RTT is invalid: %v", max)
		}
		if min < 25 * 2 * 0.90 * time.Millisecond {
			return fmt.Errorf("min RTT is invalid: %v", min)
		}
	}
	return nil
}

func handleConnection(conn *net.TCPConn) {
	defer conn.Close()
	// disable Nagle's algorithm to measure latency.
	err := conn.SetNoDelay(true)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 1)
	for {
		_, err := conn.Read(buf)
		if err == io.EOF {
			return
		}
		if err != nil {
			panic(err)
		}
		conn.Write(buf)
	}
}

func server(runenv *runtime.RunEnv, listener *net.TCPListener) {
	for {
		runenv.RecordMessage("accepting new connections")
		conn, err := listener.AcceptTCP()
		if err != nil {
			return
		}
		go handleConnection(conn)
	}
}

func Summarize(deltas []time.Duration) (min, max, avg time.Duration) {
	min = deltas[0]
	max = deltas[0]
	avg = deltas[0]

	for _, d := range deltas[1:] {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
		avg += d
	}

	avg /= time.Duration(len(deltas))
	return
}
