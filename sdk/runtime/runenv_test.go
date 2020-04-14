package runtime

import (
	"reflect"
	"testing"
)

func TestParseKeyValues(t *testing.T) {
	type args struct {
		in []string
	}
	tests := []struct {
		name    string
		args    args
		wantRes map[string]string
		wantErr bool
	}{
		{
			name: "empty, int, string, bool",
			args: args{
				[]string{
					"TEST_INSTANCE_ROLE=",
					"TEST_ARTIFACTS=/artifacts",
					"TEST_SIDECAR=true",
				},
			},
			wantErr: false,
			wantRes: map[string]string{
				"TEST_INSTANCE_ROLE": "",
				"TEST_ARTIFACTS":     "/artifacts",
				"TEST_SIDECAR":       "true",
			},
		},
		{
			name: "empty, string, int, complex",
			args: args{
				[]string{
					"TEST_BRANCH=",
					"TEST_RUN=e765696a-bdf2-408e-8b39-aeb0e90c0ff6",
					"TEST_GROUP_INSTANCE_COUNT=200",
					"TEST_GROUP_ID=single",
					"TEST_INSTANCE_PARAMS=bucket_size=2|n_find_peers=1|timeout_secs=300|auto_refresh=true|random_walk=false|n_bootstrap=1",
					"TEST_SUBNET=30.38.0.0/16",
				},
			},
			wantErr: false,
			wantRes: map[string]string{
				"TEST_BRANCH":               "",
				"TEST_RUN":                  "e765696a-bdf2-408e-8b39-aeb0e90c0ff6",
				"TEST_GROUP_INSTANCE_COUNT": "200",
				"TEST_GROUP_ID":             "single",
				"TEST_INSTANCE_PARAMS":      "bucket_size=2|n_find_peers=1|timeout_secs=300|auto_refresh=true|random_walk=false|n_bootstrap=1",
				"TEST_SUBNET":               "30.38.0.0/16",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := ParseKeyValues(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeyValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("ParseKeyValues() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
