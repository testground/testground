package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	if runenv.TestCaseSeq < 0 {
		panic("test case sequence number not set")
	}

	if runenv.TestCaseSeq != 0 {
		return fmt.Errorf("aborting")
	}

	runenv.Message("before sync.MustWatcherWriter")
	watcher, writer := sync.MustWatcherWriter(runenv)
	defer watcher.Close()
	defer writer.Close()

	if !runenv.TestSidecar {
		return nil
	}

	runenv.Message("before sync.WaitNetworkInitialized")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	oldAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	config := sync.NetworkConfig{
		// Control the "default" network. At the moment, this is the only network.
		Network: "default",

		// Enable this network. Setting this to false will disconnect this test
		// instance from this network. You probably don't want to do that.
		Enable: true,
		Default: sync.LinkShape{
			Latency:   100 * time.Millisecond,
			Bandwidth: 1 << 20, // 1Mib
		},
		State: "network-configured",
	}

	runenv.Message("before writer config")
	_, err = writer.Write(sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	runenv.Message("before barrier")
	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}

	// Make sure that the IP addresses don't change unless we request it.

	newAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	if !sameAddrs(oldAddrs, newAddrs) {
		return fmt.Errorf("interfaces changed")
	}

	// Get a sequence number
	runenv.Message("get a sequence number")
	seq, err := writer.Write(&sync.Subtree{
		GroupKey:    "ip-allocation",
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		},
	}, hostname)
	if err != nil {
		return err
	}

	runenv.Message("I am %d", seq)

	if seq >= 1<<16 {
		return fmt.Errorf("test-case only supports 2**16 instances")
	}

	ipC := byte((seq >> 8) + 1)
	ipD := byte(seq)

	config.IPv4 = &*runenv.TestSubnet
	config.IPv4.IP = append(config.IPv4.IP[0:2:2], ipC, ipD)
	config.State = "ip-changed"

	var (
		listener *net.TCPListener
		conn     *net.TCPConn
	)
	if seq == 1 {
		listener, err = net.ListenTCP("tcp4", &net.TCPAddr{Port: 1234})
		if err != nil {
			return err
		}
		defer listener.Close()
	}

	logging.S().Debug("before writing changed ip config to redis")
	_, err = writer.Write(sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	logging.S().Debug("waiting for barrier")
	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}

	switch seq {
	case 1:
		conn, err = listener.AcceptTCP()
	case 2:
		conn, err = net.DialTCP("tcp4", nil, &net.TCPAddr{
			IP:   append(config.IPv4.IP[:3:3], 1),
			Port: 1234,
		})
	default:
		return fmt.Errorf("expected at most two test instances")
	}
	if err != nil {
		return err
	}

	defer conn.Close()

	// trying to measure latency here.
	conn.SetNoDelay(true)

	pingPong := func(test string, rttMin, rttMax time.Duration) error {
		buf := make([]byte, 1)

		runenv.Message("waiting until ready")
		// wait till both sides are ready
		_, err = conn.Write([]byte{0})
		if err != nil {
			return err
		}
		_, err = conn.Read(buf)
		if err != nil {
			return err
		}

		start := time.Now()

		runenv.Message("writing my id")
		// write sequence number.
		_, err = conn.Write([]byte{byte(seq)})
		if err != nil {
			return err
		}

		runenv.Message("reading their id")
		// pong other sequence number
		_, err = conn.Read(buf)
		if err != nil {
			return err
		}
		runenv.Message("returning their id")
		_, err = conn.Write(buf)
		if err != nil {
			return err
		}

		runenv.Message("reading my id")
		// read our sequence number
		_, err = conn.Read(buf)
		if err != nil {
			return err
		}

		runenv.Message("done")

		// stop
		end := time.Now()

		// check the sequence number.
		if buf[0] != byte(seq) {
			return fmt.Errorf("read unexpected value")
		}

		// check the RTT
		rtt := end.Sub(start)
		if rtt < rttMin || rtt > rttMax {
			return fmt.Errorf("expected an RTT between %s and %s, got %s", rttMin, rttMax, rtt)
		}
		runenv.Message("ping RTT was %s [%s, %s]", rtt, rttMin, rttMax)

		state := sync.State("ping-pong-" + test)

		// Don't reconfigure the network until we're done with the first test.
		writer.SignalEntry(state)
		err = <-watcher.Barrier(ctx, state, int64(runenv.TestInstanceCount))
		if err != nil {
			return err
		}
		return nil
	}
	err = pingPong("200", 200*time.Millisecond, 205*time.Millisecond)
	if err != nil {
		return err
	}

	config.Default.Latency = 10 * time.Millisecond
	config.State = "latency-reduced"

	logging.S().Debug("writing new config with latency reduced")
	_, err = writer.Write(sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	logging.S().Debug("waiting at barrier")
	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}

	logging.S().Debug("ping pong")
	err = pingPong("10", 20*time.Millisecond, 30*time.Millisecond)
	if err != nil {
		return err
	}

	return nil
}

func sameAddrs(a, b []net.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	aset := make(map[string]bool, len(a))
	for _, addr := range a {
		aset[addr.String()] = true
	}
	for _, addr := range b {
		if !aset[addr.String()] {
			return false
		}
	}
	return true
}
