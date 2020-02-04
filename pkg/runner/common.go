package runner

import (
	"errors"
	"fmt"
	"net"
)

// Use consistent IP address ranges for both the data and the control subnet.
// This range was selected as it's specifically set aside for testing and
// shouldn't conflict with any real networks.
var (
	controlSubnet  = "192.18.0.0/16"
	controlGateway = "192.18.0.1"
)

func nextDataNetwork(lenNetworks int) (*net.IPNet, string, error) {
	if lenNetworks > 4095 {
		return nil, "", errors.New("space exhausted")
	}
	a := 16 + lenNetworks/256
	b := 0 + lenNetworks%256

	sn := fmt.Sprintf("%d.%d.0.0/16", a, b)
	gw := fmt.Sprintf("%d.%d.0.1", a, b)

	_, subnet, err := net.ParseCIDR(sn)
	return subnet, gw, err
}
