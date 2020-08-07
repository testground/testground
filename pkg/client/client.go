package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/testground/testground/pkg/task"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"

	"github.com/testground/testground/pkg/api"
	"github.com/testground/testground/pkg/config"
	"github.com/testground/testground/pkg/logging"
	"github.com/testground/testground/pkg/rpc"

	"github.com/mholt/archiver"
	"github.com/mitchellh/mapstructure"
)

// Client is the API client that performs all operations
// against a Testground server.
type Client struct {
	// client used to send and receive http requests.
	client   *http.Client
	cfg      *config.EnvConfig
	endpoint string
}

// New initializes a new API client
func New(cfg *config.EnvConfig) *Client {
	endpoint := cfg.Client.Endpoint

	logging.S().Infow("testground client initialized", "addr", endpoint)

	return &Client{
		client:   &http.Client{},
		cfg:      cfg,
		endpoint: endpoint,
	}
}

// Close the transport used by the client
func (c *Client) Close() error {
	if t, ok := c.client.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

// Build sends a multipart `build` request to the daemon.
//
// A build request comprises the following parts:
//
//  * Part 1 (Content-Type: application/json): composition json.
//  * Part 2 (Content-Type: application/zip): test plan source.
//  * Part 3 (optional, Content-Type: application/zip): linked sdk.
//
// The Body in the response implements an io.ReadCloser and it's up to the
// caller to close it.
//
// The response is a stream of `Msg` protocol messages. See
// `ParseBuildResponse()` for specifics.
func (c *Client) Build(ctx context.Context, r *api.BuildRequest, plandir string, sdkdir string, extraSrcs []string) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	var (
		rd, wr = io.Pipe()
		mp     = multipart.NewWriter(wr)
	)

	go func() error {
		var (
			hcomp  = make(textproto.MIMEHeader) // composition
			hplan  = make(textproto.MIMEHeader) // plan source
			hsdk   = make(textproto.MIMEHeader) // optional sdk
			hextra = make(textproto.MIMEHeader) // optional extra dirs
		)

		hcomp.Set("Content-Type", "application/json")
		hcomp.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "composition.json"}))

		hplan.Set("Content-Type", "application/zip")
		hplan.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "plan.zip"}))

		hsdk.Set("Content-Type", "application/zip")
		hsdk.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "sdk.zip"}))

		hextra.Set("Content-Type", "application/zip")
		hextra.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "extra.zip"}))

		// Part 1: composition json.
		w, err := mp.CreatePart(hcomp)
		if err != nil {
			return wr.CloseWithError(err)
		}

		if err := json.NewEncoder(w).Encode(r); err != nil {
			return wr.CloseWithError(err)
		}

		// Part 2: plan source directory.
		w, err = mp.CreatePart(hplan)
		if err != nil {
			return wr.CloseWithError(err)
		}
		if err = writeZippedDirs(w, false, plandir); err != nil {
			return wr.CloseWithError(err)
		}

		// Optional part 3: sdk source directory.
		if sdkdir != "" {
			w, err = mp.CreatePart(hsdk)
			if err != nil {
				return wr.CloseWithError(err)
			}
			if err = writeZippedDirs(w, false, sdkdir); err != nil {
				return wr.CloseWithError(err)
			}
		}

		if len(extraSrcs) != 0 {
			w, err = mp.CreatePart(hextra)
			if err != nil {
				return wr.CloseWithError(err)
			}
			if err = writeZippedDirs(w, true, extraSrcs...); err != nil {
				return wr.CloseWithError(err)
			}
		}

		if err := mp.Close(); err != nil {
			return wr.CloseWithError(err)
		}
		return wr.Close()
	}() //nolint:errcheck

	contentType := "multipart/related; boundary=" + mp.Boundary()
	return c.request(ctx, "POST", "/build", rd, "Content-Type", contentType)
}

func (c *Client) Run(ctx context.Context, r *api.RunRequest, plandir string, sdkdir string, extraSrcs []string) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	var (
		rd, wr = io.Pipe()
		mp     = multipart.NewWriter(wr)
	)

	go func() error {
		var (
			hcomp  = make(textproto.MIMEHeader) // composition
			hplan  = make(textproto.MIMEHeader) // plan source
			hsdk   = make(textproto.MIMEHeader) // optional sdk
			hextra = make(textproto.MIMEHeader) // optional extra dirs
		)

		hcomp.Set("Content-Type", "application/json")
		hcomp.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "composition.json"}))

		hplan.Set("Content-Type", "application/zip")
		hplan.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "plan.zip"}))

		hsdk.Set("Content-Type", "application/zip")
		hsdk.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "sdk.zip"}))

		hextra.Set("Content-Type", "application/zip")
		hextra.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "extra.zip"}))

		// Part 1: composition json.
		w, err := mp.CreatePart(hcomp)
		if err != nil {
			return wr.CloseWithError(err)
		}

		if err := json.NewEncoder(w).Encode(r); err != nil {
			return wr.CloseWithError(err)
		}

		// Optional part 2: plan source directory.
		if plandir != "" {
			w, err = mp.CreatePart(hplan)
			if err != nil {
				return wr.CloseWithError(err)
			}
			if err = writeZippedDirs(w, false, plandir); err != nil {
				return wr.CloseWithError(err)
			}
		}

		// Optional part 3: sdk source directory.
		if sdkdir != "" {
			w, err = mp.CreatePart(hsdk)
			if err != nil {
				return wr.CloseWithError(err)
			}
			if err = writeZippedDirs(w, false, sdkdir); err != nil {
				return wr.CloseWithError(err)
			}
		}

		if len(extraSrcs) != 0 {
			w, err = mp.CreatePart(hextra)
			if err != nil {
				return wr.CloseWithError(err)
			}
			if err = writeZippedDirs(w, true, extraSrcs...); err != nil {
				return wr.CloseWithError(err)
			}
		}

		if err := mp.Close(); err != nil {
			return wr.CloseWithError(err)
		}
		return wr.Close()
	}() //nolint:errcheck

	contentType := "multipart/related; boundary=" + mp.Boundary()
	return c.request(ctx, "POST", "/run", rd, "Content-Type", contentType)
}

// CollectOutputs sends a `collectOutputs` request to the daemon.
//
// The Body in the response implement an io.ReadCloser and it's up to the caller
// to close it.
func (c *Client) CollectOutputs(ctx context.Context, r *api.OutputsRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/outputs", bytes.NewReader(body.Bytes()))
}

// Terminate sends a `terminate` request to the daemon.
func (c *Client) Terminate(ctx context.Context, r *api.TerminateRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/terminate", bytes.NewReader(body.Bytes()))
}

// Healthcheck sends a `healthcheck` request to the daemon.
func (c *Client) Healthcheck(ctx context.Context, r *api.HealthcheckRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/healthcheck", bytes.NewReader(body.Bytes()))
}

// BuildPurge sends a `build/purge` request to the daemon.
func (c *Client) BuildPurge(ctx context.Context, r *api.BuildPurgeRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/build/purge", bytes.NewReader(body.Bytes()))
}

func (c *Client) TaskInfo(ctx context.Context, id string) (io.ReadCloser, error) {
	return c.request(ctx, "GET", "/task/"+id, nil)
}

func parseGeneric(r io.ReadCloser, fnProgress, fnBinary, fnResult func(interface{}) error) error {
	var chunk rpc.Chunk
	var once sync.Once

	for dec := json.NewDecoder(r); ; {
		err := dec.Decode(&chunk)
		if err != nil {
			return err
		}

		switch chunk.Type {
		case rpc.ChunkTypeProgress:
			once.Do(func() {
				fmt.Println(aurora.Bold(aurora.Cyan("\n>>> Server output:\n")))
			})

			err = fnProgress(chunk.Payload)
			if err != nil {
				return err
			}

		case rpc.ChunkTypeError:
			fmt.Println(aurora.Bold(aurora.BrightRed("\n>>> Error:\n")))
			return errors.New(chunk.Error.Msg)

		case rpc.ChunkTypeResult:
			fmt.Println(aurora.Bold(aurora.BrightGreen("\n>>> Result:\n")))
			return fnResult(chunk.Payload)

		case rpc.ChunkTypeBinary:
			err := fnBinary(chunk.Payload)
			if err != nil {
				return err
			}

		default:
			return errors.New("unknown message type")
		}
	}
}

func printProgress(progress interface{}) error {
	m, err := base64.StdEncoding.DecodeString(progress.(string))
	if err != nil {
		return err
	}

	fmt.Print(string(m))
	return nil
}

// ParseCollectResponse parses a response from a `collect` call
func ParseCollectResponse(r io.ReadCloser, file io.Writer) (api.CollectResponse, error) {
	var resp api.CollectResponse
	err := parseGeneric(
		r,
		printProgress,
		func(payload interface{}) error {
			m, err := base64.StdEncoding.DecodeString(payload.(string))
			if err != nil {
				return err
			}

			_, err = file.Write(m)
			return err
		},
		func(result interface{}) error {
			resp.Exists = result.(bool)
			return nil
		},
	)
	return resp, err
}

// ParseRunResponse parses a response from a `run` call
func ParseRunResponse(r io.ReadCloser) (string, error) {
	var resp string
	err := parseGeneric(
		r,
		printProgress,
		nil,
		func(result interface{}) error {
			return mapstructure.Decode(result, &resp)
		},
	)
	return resp, err
}

// ParseBuildResponse parses a response from a `build` call
func ParseBuildResponse(r io.ReadCloser) (string, error) {
	var resp string
	err := parseGeneric(
		r,
		printProgress,
		nil,
		func(result interface{}) error {
			return mapstructure.Decode(result, &resp)
		},
	)
	return resp, err
}

// ParseBuildPurgeResponse parses a response from 'build/purge' call.
func ParseBuildPurgeResponse(r io.ReadCloser) error {
	return parseGeneric(
		r,
		printProgress,
		nil,
		func(result interface{}) error {
			return nil
		},
	)
}

// ParseTerminateRequest parses a response from a 'terminate' call
func ParseTerminateRequest(r io.ReadCloser) error {
	return parseGeneric(
		r,
		printProgress,
		nil,
		func(result interface{}) error {
			return nil
		},
	)
}

// ParseHealthcheckResponse parses a response from a 'healthcheck' call
func ParseHealthcheckResponse(r io.ReadCloser) (api.HealthcheckResponse, error) {
	var resp api.HealthcheckResponse

	err := parseGeneric(
		r,
		printProgress,
		nil,
		func(result interface{}) error {
			return mapstructure.Decode(result, &resp)
		},
	)

	return resp, err
}

func ParseTaskInfoResponse(r io.ReadCloser) (task.Task, error) {
	var resp task.Task

	err := parseGeneric(
		r,
		printProgress,
		nil,
		func(result interface{}) error {
			fmt.Println(result)
			return Decode(result, &resp)
		},
	)

	return resp, err
}

func (c *Client) request(ctx context.Context, method string, path string, body io.Reader, headers ...string) (io.ReadCloser, error) {
	if len(headers)%2 != 0 {
		return nil, fmt.Errorf("headers must be tuples: key1, value1, key2, value2")
	}
	req, err := http.NewRequest(method, c.endpoint+path, body)
	req = req.WithContext(ctx)

	token := strings.TrimSpace(c.cfg.Client.Token)
	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	for i := 0; i < len(headers); i = i + 2 {
		req.Header.Add(headers[i], headers[i+1])
	}

	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// see https://github.com/mitchellh/mapstructure/issues/159#issuecomment-482201507
func ToTimeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(time.Time{}) {
			return data, nil
		}

		switch f.Kind() {
		case reflect.String:
			return time.Parse(time.RFC3339, data.(string))
		case reflect.Float64:
			return time.Unix(0, int64(data.(float64))*int64(time.Millisecond)), nil
		case reflect.Int64:
			return time.Unix(0, data.(int64)*int64(time.Millisecond)), nil
		default:
			return data, nil
		}
		// Convert it by parsing
	}
}

func Decode(input interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: nil,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			ToTimeHookFunc()),
		Result: result,
	})
	if err != nil {
		return err
	}

	if err := decoder.Decode(input); err != nil {
		return err
	}
	return err
}

// writeZippedDirs a list of directories, zips them into a single zip file, and writes it on w.
// if toplevel=true, it will retain the toplevel directories, so if /abc, /def are passed, the resulting
// zip archive will contain /abc and /def.
// if toplevel=false, it will omit the toplevel directories and will place the contents of each
// at the root of the zip, with overwrite=true. So /abc and /def are placed as /abc/* and /def/* at the root.
func writeZippedDirs(w io.Writer, toplevel bool, dirs ...string) error {
	// A temporary .zip file to deflate the directory into.
	// archiver doesn't support archiving direcly into an io.Writer, so we
	// need a file as a buffer.
	tmp, err := ioutil.TempFile("", "*.zip")
	if err != nil {
		return err
	}

	// Make sure to clean up the tmp zip file after we're done.
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	for _, dir := range dirs {
		if fi, err := os.Stat(dir); err != nil {
			return err
		} else if !fi.IsDir() {
			return fmt.Errorf("file %s is not a directory", dir)
		}
	}

	var files []string
	if toplevel {
		files = dirs
	} else {
		for _, dir := range dirs {
			// Fetch all files in the dir to pass them to archiver; otherwise we'll
			// end up with a top-level directory inside the zip.
			fis, err := ioutil.ReadDir(dir)
			if err != nil {
				return err
			}
			for _, fi := range fis {
				files = append(files, filepath.Join(dir, fi.Name()))
			}
		}
	}

	// Deflate the directory into a zip archive, allowing it to overwrite
	// the tmp file that we created above.
	z := archiver.NewZip()
	z.OverwriteExisting = true
	if err = z.Archive(files, tmp.Name()); err != nil {
		return err
	}

	// Write out the tmp file into the supplied io.Writer.
	_, err = io.Copy(w, tmp)
	return err
}
