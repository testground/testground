package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/testground/sdk-go/network"
)

func main() {
	kkk := `{
		"Network": "default",
		"Enable": true,
		"Default": {
				"Latency": 100000000,
				"Bandwidth": 1048576
		},
		"State": "ip-changed",
		"RoutingPolicy": "deny-all",
		"IPv4": {
			"IP": "16.0.0.2",
			"Mask": "255.255.0.0"
		}
		
}`

	config := network.Config{}

	// err := json.NewDecoder(strings.NewReader(kkk)).Decode(&config)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	byts, _ := json.MarshalIndent(&net.IPNet{
		IP:   []byte{16, 0, 0, 2},
		Mask: []byte{255, 255, 0, 0},
	}, "", "  ")

	fmt.Println(config, kkk, string(byts))
}
