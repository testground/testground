package runner

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/rpc"
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

func gzipRunOutputs(ctx context.Context, basedir string, input *api.CollectionInput, ow *rpc.OutputWriter) error {
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

	gz := gzip.NewWriter(ow.BinaryWriter())
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	// validate path
	dir = filepath.Clean(dir)

	walker := func(file string, finfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(finfo, finfo.Name())
		if err != nil {
			return err
		}

		relFilePath := file
		if filepath.IsAbs(dir) {
			relFilePath, err = filepath.Rel(dir, file)
			if err != nil {
				return err
			}
		}

		hdr.Name = relFilePath

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if finfo.Mode().IsDir() {
			return nil
		}

		// add file to tar
		srcFile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		_, err = io.Copy(tw, srcFile)
		if err != nil {
			return err
		}
		return nil
	}

	if err := filepath.Walk(dir, walker); err != nil {
		return err
	}
	return nil
}

func reviewResources(group *api.RunGroup, ow *rpc.OutputWriter) {
	log := ow.With("group_id", group.ID)
	if group.Resources.CPU != "" || group.Resources.Memory != "" {
		log.Warnw("group has resources set. note that resources requirement and limits are ignored on the this runner.")
	}
}
