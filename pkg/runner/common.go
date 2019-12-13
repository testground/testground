package runner

import (
	"errors"
	"fmt"
)

// Use consistent IP address ranges for both the data and the control subnet.
// _which_
var (
	controlSubnet  = "192.168.0.0/16"
	controlGateway = "192.168.0.1"
)

func nextDataNetwork(lenNetworks int) (string, string, error) {
	if lenNetworks > 4095 {
		return "", "", errors.New("space exhausted")
	}
	a := 16 + lenNetworks/256
	b := 0 + lenNetworks%256

	subnet := fmt.Sprintf("%d.%d.0.0/16", a, b)
	gateway := fmt.Sprintf("%d.%d.0.1", a, b)

	return subnet, gateway, nil
}
