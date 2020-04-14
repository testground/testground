package runner

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/rpc"
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

func zipRunOutputs(ctx context.Context, basedir string, input *api.CollectionInput, ow *rpc.OutputWriter) error {
	pattern := filepath.Join(basedir, "*", input.RunID)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	if len(matches) != 1 {
		return fmt.Errorf("run ID %s not found with runner %s", input.RunID, input.RunnerID)
	}

	dir := matches[0]

	if fi, err := os.Stat(dir); err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("internal error: not a directory when accessing run outputs")
	}

	wz := zip.NewWriter(ow.BinaryWriter())
	defer wz.Close()
	defer wz.Flush()

	base := filepath.Base(dir)
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.Join(base, strings.TrimPrefix(path, dir))
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := wz.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}

func reviewResources(group api.RunGroup, ow *rpc.OutputWriter) {
	if group.Resources.CPU != "" || group.Resources.Memory != "" {
		ow.Warn("group has resources set. note that resources requirement and limits are ignored on the this runner.")
	}
}
