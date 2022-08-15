package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func pingpong(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	runenv.RecordMessage("before sync.MustBoundClient")
	client := initCtx.SyncClient
	netclient := initCtx.NetClient

	oldAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	config := &network.Config{
		// Control the "default" network. At the moment, this is the only network.
		Network: "default",

		// Enable this network. Setting this to false will disconnect this test
		// instance from this network. You probably don't want to do that.
		Enable: true,
		Default: network.LinkShape{
			Latency:   100 * time.Millisecond,
			Bandwidth: 1 << 20, // 1Mib
		},
		CallbackState: "network-configured",
		RoutingPolicy: network.DenyAll,
	}

	runenv.RecordMessage("before netclient.MustConfigureNetwork")
	netclient.MustConfigureNetwork(ctx, config)

	seq := client.MustSignalAndWait(ctx, "ip-allocation", runenv.TestInstanceCount)

	// Make sure that the IP addresses don't change unless we request it.
	if newAddrs, err := net.InterfaceAddrs(); err != nil {
		return err
	} else if !sameAddrs(oldAddrs, newAddrs) {
		return fmt.Errorf("interfaces changed")
	}

	ipC := byte((seq >> 8) + 1)
	ipD := byte(seq)

	config.IPv4 = runenv.TestSubnet

	var newIp = append(config.IPv4.IP[0:2:2], ipC, ipD)

	runenv.RecordMessage("I am %d, and my desired IP is %s\n", seq, newIp)

	config.IPv4.IP = newIp
	config.IPv4.Mask = []byte{255, 255, 255, 0}
	config.CallbackState = "ip-changed"

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

	runenv.RecordMessage("before reconfiguring network")
	netclient.MustConfigureNetwork(ctx, config)

	getPodInfo()

	switch seq {
	case 1:
		fmt.Println("This container is listening!")
		conn, err = listener.AcceptTCP()
	case 2:
		var targetIp = append(config.IPv4.IP[:3:3], 1)
		fmt.Printf("Attempting to dial %s\n", targetIp)
		conn, err = net.DialTCP("tcp4", nil, &net.TCPAddr{
			IP:   targetIp,
			Port: 1234,
		})

		if err != nil {
			fmt.Printf("Received an error attempting to dial %s \n", targetIp)
			return err
		}
	default:
		return fmt.Errorf("expected at most two test instances")
	}
	if err != nil {
		return err
	}

	defer conn.Close()

	// trying to measure latency here.
	err = conn.SetNoDelay(true)
	if err != nil {
		return err
	}

	pingPong := func(test string, rttMin, rttMax time.Duration) error {
		buf := make([]byte, 1)

		runenv.RecordMessage("waiting until ready")

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

		// write sequence number.
		runenv.RecordMessage("writing my id")
		_, err = conn.Write([]byte{byte(seq)})
		if err != nil {
			return err
		}

		// pong other sequence number
		runenv.RecordMessage("reading their id")
		_, err = conn.Read(buf)
		if err != nil {
			return err
		}

		runenv.RecordMessage("returning their id")
		_, err = conn.Write(buf)
		if err != nil {
			return err
		}

		runenv.RecordMessage("reading my id")
		// read our sequence number
		_, err = conn.Read(buf)
		if err != nil {
			return err
		}

		runenv.RecordMessage("done")

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
		runenv.RecordMessage("ping RTT was %s [%s, %s]", rtt, rttMin, rttMax)

		// Don't reconfigure the network until we're done with the first test.
		state := sync.State("ping-pong-" + test)
		client.MustSignalAndWait(ctx, state, runenv.TestInstanceCount)

		return nil
	}
	err = pingPong("200", 0, 21500*time.Millisecond)
	if err != nil {
		return err
	}

	// config.Default.Latency = 10 * time.Millisecond
	// config.CallbackState = "latency-reduced"
	// netclient.MustConfigureNetwork(ctx, config)

	runenv.RecordMessage("ping pong")
	err = pingPong("10", 0, 3500000*time.Millisecond)
	if err != nil {
		return err
	}

	return nil
}

func getPodInfo() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		// get pods in all the namespaces by omitting namespace
		// Or specify namespace to get pods in particular namespace
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		// Examples for error handling:
		// - Use helper functions e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		pod, err := clientset.CoreV1().Pods("default").Get(context.TODO(), "example-xxxxx", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod example-xxxxx not found in default namespace\n")
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found example-xxxxx pod in default namespace\n")
			fmt.Printf("Pod name: %s  | Annotations: \n", pod.Name)
			for k, v := range pod.GetAnnotations() {
				fmt.Printf("%s->%s \n", k, v)
			}
		}

		time.Sleep(10 * time.Second)
	}
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
