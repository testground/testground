package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/testground/pkg/api"
)

// Use consistent IP address ranges for both the data and the control subnet.
// This range was selected as it's specifically set aside for testing and
// shouldn't conflict with any real networks.
var (
	controlSubnet  = "192.18.0.0/16"
	controlGateway = "192.18.0.1"
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

func getWorkDir(input *api.RunInput) (string, error) {
	path := filepath.Join(input.EnvConfig.WorkDir(), "results")
	err := os.MkdirAll(path, 0777)
	return path, err
}

func getRunDir(input *api.RunInput) (string, error) {
	workDir, err := getWorkDir(input)
	if err != nil {
		return "", err
	}
	path := filepath.Join(workDir, input.TestPlan.Name, input.RunID)
	err = os.MkdirAll(path, 0777)
	return path, err
}
