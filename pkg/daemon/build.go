package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"
)

func (d *Daemon) buildHandler(engine api.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ruid := r.Header.Get("X-Request-ID")
		log := logging.S().With("req_id", ruid)

		log.Infow("handle request", "command", "build")
		defer log.Infow("request handled", "command", "build")

		tgw := rpc.NewOutputWriter(w, r)

		// Create a packing directory under the workdir.
		dir := filepath.Join(engine.EnvConfig().Dirs().Work(), "requests", ruid)
		if err := os.MkdirAll(dir, 0755); err != nil {
			tgw.WriteError("failed to create temp directory to unpack request", "err", err)
			return
		}

		req, plan, sdk, err := consumeBuildRequest(r, dir)
		if err != nil {
			tgw.WriteError("failed to consume request", "err", err)
			return
		}

		out, err := engine.DoBuild(r.Context(), &req.Composition, dir, plan, sdk, tgw)
		if err != nil {
			tgw.WriteError(fmt.Sprintf("engine build error: %s", err))
			return
		}

		tgw.WriteResult(out)
	}
}

func consumeBuildRequest(r *http.Request, dir string) (*api.BuildRequest, string, string, error) {
	var (
		req *api.BuildRequest
		p   *multipart.Part
		err error

		plan string
		sdk  string
	)

	if r.Body == nil {
		return nil, "", "", fmt.Errorf("failed to parse request: nil body")
	}

	defer r.Body.Close()

	// Validate the incoming multipart request.
	ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to parse request: %w", err)
	}

	if !strings.HasPrefix(ct, "multipart/") {
		ct := r.Header.Get("Content-Type")
		return nil, "", "", fmt.Errorf("not a multipart request; Content-Type: %s", ct)
	}

	rd := multipart.NewReader(r.Body, params["boundary"])

	// 1. Read and decode the request payload.
	if p, err = rd.NextPart(); err != nil {
		return nil, "", "", fmt.Errorf("unexpected error when reading composition: %w", err)
	}
	if err = json.NewDecoder(p).Decode(&req); err != nil {
		return nil, "", "", fmt.Errorf("failed to json decode request body: %w", err)
	}

	var (
		planzip *os.File
		sdkzip  *os.File
	)

	// 2a. Read test plan source archive.
	if planzip, err = os.Create(filepath.Join(dir, "plan.zip")); err != nil {
		return nil, "", "", fmt.Errorf("failed to create file for test plan: %w", err)
	}
	if p, err = rd.NextPart(); err != nil {
		return nil, "", "", fmt.Errorf("unexpected error when reading test plan: %w", err)
	}
	if _, err = io.Copy(planzip, p); err != nil {
		return nil, "", "", fmt.Errorf("unexpected error when copying test plan: %w", err)
	}

	// 2b. Inflate the test plan source archive.
	plan = filepath.Join(dir, "plan")
	if err := os.Mkdir(plan, 0755); err != nil {
		return nil, "", "", fmt.Errorf("failed to create directory for test plan: %w", err)
	}
	if err := archiver.NewZip().Unarchive(planzip.Name(), plan); err != nil {
		return nil, "", "", fmt.Errorf("failed to decompress test plan: %w", err)
	}

	// 3. Read the optional sdk archive.
	switch p, err = rd.NextPart(); err {
	case io.EOF:
		// this is ok; we have no sdk to link.
	case nil:
		// 3a. Read sdk source archive.
		if sdkzip, err = os.Create(filepath.Join(dir, "sdk.zip")); err != nil {
			return nil, "", "", fmt.Errorf("failed to create file for sdk: %w", err)
		}
		if _, err = io.Copy(sdkzip, p); err != nil {
			return nil, "", "", fmt.Errorf("unexpected error when copying sdk: %w", err)
		}

		// 3b. Inflate the sdk source archive.
		sdk = filepath.Join(dir, "sdk")
		if err := os.Mkdir(sdk, 0755); err != nil {
			return nil, "", "", fmt.Errorf("failed to create directory for sdk: %w", err)
		}
		if err := archiver.NewZip().Unarchive(sdkzip.Name(), sdk); err != nil {
			return nil, "", "", fmt.Errorf("failed to decompress sdk: %w", err)
		}
	default:
		// an error occurred.
		return nil, "", "", fmt.Errorf("unexpected error when reading sdk: %w", err)
	}

	return req, plan, sdk, nil
}
