package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	syncc "sync"

	"github.com/pkg/errors"
	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/ptypes"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

var size int

var buffersize = 4 * 1024 // 4kb

type ListenAddrs struct {
	Addrs []string
}

var PeerTopic = sync.NewTopic("peers", &ListenAddrs{})

func Storm(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Second)
	defer cancel()

	runenv.RecordStart()

	connCount := runenv.IntParam("conn_count")
	connDelayMs := runenv.IntParam("conn_delay_ms")
	connDial := runenv.IntParam("concurrent_dials")
	outgoing := runenv.IntParam("conn_outgoing")
	size = runenv.IntParam("data_size_kb")

	runenv.RecordMessage("running with data_size_kb: %d", size)
	runenv.RecordMessage("running with conn_outgoing: %d", outgoing)
	runenv.RecordMessage("running with conn_count: %d", connCount)
	runenv.RecordMessage("running with conn_delay_ms: %d", connDelayMs)
	runenv.RecordMessage("running with conncurrent_dials: %d", connDial)

	size = size * 1024 // convert kb to bytes

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	if !runenv.TestSidecar {
		return nil
	}

	netclient := network.NewClient(client, runenv)
	netclient.MustWaitNetworkInitialized(ctx)

	tcpAddr, err := getSubnetAddr(runenv.TestSubnet)
	if err != nil {
		return err
	}

	mynode := &ListenAddrs{}
	mine := map[string]struct{}{}

	for i := 0; i < connCount; i++ {
		l, err := net.Listen("tcp", tcpAddr.IP.String()+":0")
		if err != nil {
			runenv.D().Counter("listens.err").Inc(1)
			runenv.RecordMessage("error listening: %s", err.Error())
			return err
		}
		defer l.Close()

		runenv.RecordMessage("listening on %s", l.Addr())
		runenv.D().Counter("listens.ok").Inc(1)

		mynode.Addrs = append(mynode.Addrs, l.Addr().String())
		mine[l.Addr().String()] = struct{}{}

		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}

				go handleRequest(runenv, conn)
			}
		}()
	}

	runenv.RecordMessage("my node info: %s", mynode.Addrs)

	_ = client.MustSignalAndWait(ctx, sync.State("listening"), runenv.TestInstanceCount)

	allAddrs, err := shareAddresses(ctx, client, runenv, mynode)
	if err != nil {
		return err
	}

	otherAddrs := []string{}
	for _, addr := range allAddrs {
		if _, ok := mine[addr]; ok {
			continue
		}
		otherAddrs = append(otherAddrs, addr)
	}

	_ = client.MustSignalAndWait(ctx, sync.State("got-other-addrs"), runenv.TestInstanceCount)

	runenv.D().Counter("other.addrs").Inc(int64(len(otherAddrs)))

	sem := make(chan struct{}, connDial)      // limit the number of concurrent net.Dials
	writesem := make(chan struct{}, connDial) // limit the number of concurrent conn.write

	var wg syncc.WaitGroup
	wg.Add(outgoing)

	alloutgoing := outgoing

	for outgoing > 0 {
		randomaddrIdx := rand.Intn(len(otherAddrs))

		addr := otherAddrs[randomaddrIdx]

		outgoing--

		sz := size

		go func() {
			defer wg.Done()

			delay := time.Duration(rand.Intn(connDelayMs)) * time.Millisecond
			runenv.RecordMessage("sleeping for: %s", delay)
			<-time.After(delay)

			sem <- struct{}{}

			t := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
			if err != nil {
				runenv.RecordFailure(fmt.Errorf("couldnt dial: %s %w", addr, err))
				runenv.D().ResettingHistogram("dial.fail").Update(int64(time.Since(t)))

				<-sem
				return
			}
			<-sem

			runenv.D().ResettingHistogram("dial.ok").Update(int64(time.Since(t)))

			_ = client.MustSignalAndWait(ctx, sync.State("outgoing-dials-done"), runenv.TestInstanceCount*alloutgoing)

			for sz > 0 {
				func() {
					writesem <- struct{}{}
					defer func() { <-writesem }()

					var data []byte
					if sz <= buffersize {
						data = make([]byte, sz)
						sz = 0
					} else {
						data = make([]byte, buffersize)
						sz -= buffersize
					}
					rand.Read(data)

					timewrite := time.Now()
					n, err := conn.Write(data)
					if err != nil {
						runenv.D().ResettingHistogram("conn.write.err").Update(int64(time.Since(timewrite)))
						runenv.RecordFailure(fmt.Errorf("couldnt write to conn: %s %w", addr, err))
						return
					}
					runenv.D().ResettingHistogram("conn.write.ok").Update(int64(time.Since(timewrite)))
					runenv.D().Counter("bytes.sent").Inc(int64(n))
				}()
			}
		}()
	}

	wg.Wait()

	runenv.RecordMessage("done writing")
	_ = client.MustSignalAndWait(ctx, sync.State("done writing"), runenv.TestInstanceCount)
	runenv.RecordMessage("done writing after barrier")

	time.Sleep(10 * time.Second) // wait for the last set of metrics to be emitted

	runenv.RecordMessage("Done")
	return nil
}

func handleRequest(runenv *runtime.RunEnv, conn net.Conn) {
	n := -1
	for n != 0 {
		buf := make([]byte, buffersize)
		var err error
		n, err = conn.Read(buf)
		if err != nil && err != io.EOF {
			fmt.Println("Error reading:", err.Error())
		}
		runenv.D().Counter("bytes.read").Inc(int64(n))
	}

	conn.Close()
}

func getSubnetAddr(subnet *ptypes.IPNet) (*net.TCPAddr, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok {
			if subnet.Contains(ip.IP) {
				tcpAddr := &net.TCPAddr{IP: ip.IP}
				return tcpAddr, nil
			}
		} else {
			panic(fmt.Sprintf("%T", addr))
		}
	}
	return nil, fmt.Errorf("no network interface found. Addrs: %v", addrs)
}

func shareAddresses(ctx context.Context, client sync.Client, runenv *runtime.RunEnv, mynodeInfo *ListenAddrs) ([]string, error) {
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan *ListenAddrs)
	if _, _, err := client.PublishSubscribe(subCtx, PeerTopic, mynodeInfo, ch); err != nil {
		return nil, errors.Wrap(err, "publish/subscribe failure")
	}

	res := []string{}

	for i := 0; i < runenv.TestInstanceCount; i++ {
		select {
		case info := <-ch:
			runenv.RecordMessage("got info: %d: %s", i, info.Addrs)
			res = append(res, info.Addrs...)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		runenv.D().Counter("got.info").Inc(1)
	}

	return res, nil
}
