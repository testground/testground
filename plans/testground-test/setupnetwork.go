package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

var (
	STATE_ST = sync.Subtree{
		GroupKey:    "setupnetwork",
		PayloadType: reflect.TypeOf(""),
		KeyFunc:     func(val interface{}) string { return val.(string) },
	}

	HOST_ST = sync.Subtree{
		GroupKey:    "serverhostname",
		PayloadType: reflect.TypeOf(&net.IP{}),
		KeyFunc:     func(val interface{}) string { return string(val.(net.IP)) },
	}
)

func setupNetworkTest(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	err := sync.WaitNetworkInitialized(ctx, runenv, watcher)
	if err != nil {
		return err
	}

	seq, err := writer.Write(ctx, &STATE_ST, "setup")
	if err != nil {
		return err
	}

	if seq == 1 { // I am a server
		return TestServer(ctx, writer, runenv)
	}

	return TestClient(ctx, watcher, runenv)
}

// Silly TCP server, accepts one connection at a time.
// Copies from accepted connection into a buffer
func TestServer(ctx context.Context, writer *sync.Writer, runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Server mode.")
	// Find out what our given IP address is.
	ipnet := &runenv.TestSubnet.IPNet
	addrs, _ := net.InterfaceAddrs()
	var listenIP net.IP
	for _, addr := range addrs {
		ip, _, _ := net.ParseCIDR(addr.String())
		if ipnet.Contains(ip) {
			listenIP = ip
		}
	}
	runenv.RecordMessage("Passing this IP to the client: %s", listenIP.String())
	_, err := writer.Write(ctx, &HOST_ST, &listenIP)
	if err != nil {
		return err
	}
	lstring := listenIP.String() + ":8000"
	l, err := net.Listen("tcp", lstring)
	if err != nil {
		return err
	}
	runenv.RecordMessage("Listening on " + lstring)
	writer.SignalEntry(ctx, sync.State("serverready"))
	buf := make([]byte, 1024*1024*1024) // Read up to 1MB
	for i := 0; i <= 5; i++ {
		writer.SignalEntry(ctx, sync.State(fmt.Sprintf("readytoaccept-%d", i)))
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		runenv.RecordMessage("Accepted connection.")
		_, err = conn.Read(buf)
		conn.Close()
	}

	return nil
}

// Connect, transfer, and record metrics.
func TestClient(ctx context.Context, watcher *sync.Watcher, runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Client mode.")
	transferMetric := runtime.MetricDefinition{
		Name:           "copy time",
		Unit:           "nanoseconds",
		ImprovementDir: -1,
	}
	latencyMetric := runtime.MetricDefinition{
		Name:           "network latency",
		Unit:           "nanoseconds",
		ImprovementDir: -1,
	}
	<-watcher.Barrier(ctx, sync.State("serverready"), 1)
	runenv.RecordMessage("Server is ready, trying to connect")
	c := make(chan *net.IP, 2)
	watcher.Subscribe(ctx, &HOST_ST, c)
	serverIp := <-c
	lstring := serverIp.String() + ":8000"
	for i := 0; i <= 5; i++ {
		connectionStart := time.Now().UnixNano()
		conn, err := net.Dial("tcp", lstring)
		connectionEnd := time.Now().UnixNano()
		if err != nil {
			runenv.RecordMessage("e", err)
			return err
		}
		buf := make([]byte, 1024)
		rand.Read(buf)
		<-watcher.Barrier(ctx, sync.State(fmt.Sprintf("readytoaccept-%d", i)), 1)
		transferStart := time.Now().UnixNano()
		var transferred int
		for j := 0; j <= 1024; j++ {
			n, err := conn.Write(buf)
			if err != nil {
				return err
			}
			transferred += n
		}
		transferEnd := time.Now().UnixNano()
		runenv.RecordMessage("Wrote %d bytes.", transferred)
		conn.Close()
		runenv.RecordMetric(&latencyMetric, float64(connectionEnd-connectionStart))
		runenv.RecordMetric(&transferMetric, float64(transferEnd-transferStart))
		time.Sleep(10 * time.Second)
	}
	return nil
}
