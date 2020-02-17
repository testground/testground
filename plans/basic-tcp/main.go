package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	gosync "sync"
	"time"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

var (
	MetricBytesSent     = &runtime.MetricDefinition{Name: "sent_bytes", Unit: "bytes", ImprovementDir: -1}
	MetricBytesReceived = &runtime.MetricDefinition{Name: "received_bytes", Unit: "bytes", ImprovementDir: -1}
	MetricTimeToSend    = &runtime.MetricDefinition{Name: "time_to_send", Unit: "ms", ImprovementDir: -1}
	MetricTimeToReceive = &runtime.MetricDefinition{Name: "time_to_receive", Unit: "ms", ImprovementDir: -1}
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	withShaping := runenv.TestCaseSeq == 1

	if withShaping && !runenv.TestSidecar {
		return fmt.Errorf("Need sidecar to shape traffic")
	}

	watcher, writer := sync.MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	runenv.RecordMessage("Waiting for network to be initialized")
	if err := sync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
		return err
	}
	runenv.RecordMessage("Network initialized")

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	config := sync.NetworkConfig{
		Network: "default",
		Enable:  true,
		State:   "network-configured",
	}

	if withShaping {
		bandwidth := runenv.SizeParam("bandwidth")
		runenv.RecordMessage("Bandwidth: %v", bandwidth)
		config.Default = sync.LinkShape{
			Bandwidth: bandwidth,
		}
	}

	_, err = writer.Write(ctx, sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}
	runenv.RecordMessage("Network configured")

	// Get a sequence number
	runenv.RecordMessage("get a sequence number")
	seq, err := writer.Write(ctx, &sync.Subtree{
		GroupKey:    "ip-allocation",
		PayloadType: reflect.TypeOf(""),
		KeyFunc: func(val interface{}) string {
			return val.(string)
		},
	}, hostname)
	if err != nil {
		return err
	}

	runenv.RecordMessage("I am %d", seq)

	if seq >= 1<<16 {
		return fmt.Errorf("test-case only supports 2**16 instances")
	}

	ipC := byte((seq >> 8) + 1)
	ipD := byte(seq)

	subnet := runenv.TestSubnet.IPNet
	config.IPv4 = &subnet
	config.IPv4.IP = append(config.IPv4.IP[0:2:2], ipC, ipD)
	config.State = "ip-changed"

	runenv.RecordMessage("before writing changed ip config to redis")
	_, err = writer.Write(ctx, sync.NetworkSubtree(hostname), &config)
	if err != nil {
		return err
	}

	runenv.RecordMessage("waiting for ip-changed")
	err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
	if err != nil {
		return err
	}
	runenv.RecordMessage("Test subnet: %v", runenv.TestSubnet)
	addrs, err := net.InterfaceAddrs()
	for _, addr := range addrs {
		runenv.RecordMessage("IP: %v", addr)
	}

	// States
	ready := sync.State("ready")
	received := sync.State("received")

	switch {
	case seq == 1: // receiver
		runenv.RecordMessage("Receiver: %v", config.IPv4.IP)

		quit := make(chan int)

		l, err := net.Listen("tcp", config.IPv4.IP.String()+":2000")
		if err != nil {
			return err
		}
		defer func() {
			close(quit)
			l.Close()
		}()

		// Signal we're ready
		_, err = writer.SignalEntry(ctx, ready)
		if err != nil {
			return err
		}
		runenv.Message("State: ready")

		var wg gosync.WaitGroup
		sendersCount := runenv.TestInstanceCount - 1
		wg.Add(sendersCount)
		runenv.Message(fmt.Sprintf("Waiting for connections: %v", sendersCount))

		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					select {
					case <-quit:
						// runenv.Message("Accepted, but quitting")
						return
					default:
						runenv.RecordFailure(err)
						return
					}
				}
				// runenv.Message("Accepted")
				incomingBufferSize := runenv.SizeParam("incoming-buffer-size")
				runenv.RecordMessage("Incoming Buffer Size: %v", incomingBufferSize)
				go func(c net.Conn) {
					defer c.Close()
					bytesRead := 0
					buf := make([]byte, incomingBufferSize)
					tstarted := time.Now()
					for {
						n, err := c.Read(buf)
						bytesRead += n
						// runenv.Message(fmt.Sprintf("Received %v", n))
						if err == io.EOF {
							// runenv.Message("EOF")
							break
						} else if err != nil {
							logging.S().Error(err)
							wg.Done()
							return
						}
					}
					runenv.EmitMetric(MetricBytesReceived, float64(bytesRead))
					runenv.EmitMetric(MetricTimeToReceive, float64(time.Now().Sub(tstarted)/time.Millisecond))
					wg.Done()
				}(conn)
			}
		}()

		wg.Wait()

		// Signal we've received all the data
		_, err = writer.SignalEntry(ctx, received)
		if err != nil {
			return err
		}
		runenv.RecordMessage("Received %v uploads", sendersCount)
		runenv.RecordSuccess()

	case seq >= 2: // sender
		runenv.RecordMessage("Sender: %v", seq)

		delayBetweenUploads := runenv.IntParam("delay-between-uploads")
		delay := time.Duration((int(seq)-2)*delayBetweenUploads) * time.Millisecond
		runenv.RecordMessage("Delaying for %v", delay)
		time.Sleep(delay)

		// Wait until ready state is signalled.
		// runenv.RecordMessage("Waiting for ready")
		err = <-watcher.Barrier(ctx, ready, 1)
		if err != nil {
			return err
		}
		// runenv.RecordMessage("State: ready")

		subnet := runenv.TestSubnet.IPNet
		receiverIPv4 := &subnet
		receiverIPv4.IP = append(receiverIPv4.IP[0:2:2], 1, 1)
		runenv.RecordMessage("Dialing %v", receiverIPv4.IP)
		conn, err := net.Dial("tcp", receiverIPv4.IP.String()+":2000")
		if err != nil {
			return err
		}
		size := runenv.SizeParam("size")
		chunkSize := runenv.SizeParam("chunk-size")
		runenv.RecordMessage("Size: %v", size)
		runenv.RecordMessage("Chunk Size: %v", chunkSize)
		buf := make([]byte, chunkSize)
		for i := 0; i < len(buf); i++ {
			buf[i] = byte(i)
		}
		var bytesWritten uint64 = 0
		tstarted := time.Now()
		for bytesWritten < size {
			if size-bytesWritten < uint64(len(buf)) {
				buf = buf[:size-bytesWritten]
			}
			n, err := conn.Write(buf)
			runenv.RecordMessage("Sent %v", n)
			bytesWritten += uint64(n)
			if err != nil {
				return err
			}
		}
		runenv.EmitMetric(MetricBytesSent, float64(bytesWritten))
		runenv.EmitMetric(MetricTimeToSend, float64(time.Now().Sub(tstarted)/time.Millisecond))
		conn.Close()

		// Wait until all data is received before shutting down
		// runenv.RecordMessage("Waiting for received state")
		err = <-watcher.Barrier(ctx, received, 1)
		if err != nil {
			return err
		}
		runenv.RecordSuccess()

	default:
		return fmt.Errorf("Unexpected seq: %v", seq)
	}

	return nil
}
