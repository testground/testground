package runner

import (
	"testing"
)

func TestGetIPAMParams(t *testing.T) {
	var tests = []struct {
		lenNetworks int
		subnet      string
		gateway     string
		hasError    bool
	}{
		{0, "16.0.0.0/16", "16.0.0.1", false},
		{1, "16.1.0.0/16", "16.1.0.1", false},
		{2, "16.2.0.0/16", "16.2.0.1", false},
		{255, "16.255.0.0/16", "16.255.0.1", false},
		{256, "17.0.0.0/16", "17.0.0.1", false},
		{4095, "31.255.0.0/16", "31.255.0.1", false},
		{4096, "", "", true},
	}

	for _, tt := range tests {
		subnet, gateway, err := getIPAMParams(tt.lenNetworks)
		if subnet != tt.subnet || gateway != tt.gateway || (err != nil && !tt.hasError) {
			t.Errorf("got subnet %s gateway %s, want %s and %s", subnet, gateway, tt.subnet, tt.gateway)
		}
	}
}
