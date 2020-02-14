package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/tgwriter"

	"github.com/mitchellh/mapstructure"
)

// Client is the API client that performs all operations
// against a Testground server.
type Client struct {
	// client used to send and receive http requests.
	client   *http.Client
	endpoint string
}

// New initializes a new API client
func New(endpoint string) *Client {
	logging.S().Infow("testground client initialized", "addr", endpoint)

	return &Client{
		client:   &http.Client{},
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

// List sends `list` request to the daemon.
// The Body in the response implement an io.ReadCloser and it's up to the caller to
// close it.
func (c *Client) List(ctx context.Context) (io.ReadCloser, error) {
	return c.request(ctx, "GET", "/list", nil)
}

// Describe sends `describe` request to the daemon.
// The Body in the response implement an io.ReadCloser and it's up to the caller to
// close it.
// The response is a stream of `Msg` protocol messages. See `ParseDescribeResponse()` for specifics.
func (c *Client) Describe(ctx context.Context, r *DescribeRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "GET", "/describe", bytes.NewReader(body.Bytes()))
}

// Build sends `build` request to the daemon.
// The Body in the response implement an io.ReadCloser and it's up to the caller to
// close it.
// The response is a stream of `Msg` protocol messages. See `ParseBuildResponse()` for specifics.
func (c *Client) Build(ctx context.Context, r *BuildRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/build", bytes.NewReader(body.Bytes()))
}

// Run sends `run` request to the daemon.
// The Body in the response implement an io.ReadCloser and it's up to the caller to
// close it.
// The response is a stream of `Msg` protocol messages.
func (c *Client) Run(ctx context.Context, r *RunRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/run", bytes.NewReader(body.Bytes()))
}

// CollectOutputs sends a `collectOutputs` request to the daemon.
//
// The Body in the response implement an io.ReadCloser and it's up to the caller
// to close it.
func (c *Client) CollectOutputs(ctx context.Context, r *OutputsRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/outputs", bytes.NewReader(body.Bytes()))
}

// Terminate sends a `terminate` request to the daemon.
func (c *Client) Terminate(ctx context.Context, r *TerminateRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/terminate", bytes.NewReader(body.Bytes()))
}

// Healthcheck sends a `healthcheck` request to the daemon.
func (c *Client) Healthcheck(ctx context.Context, r *HealthcheckRequest) (io.ReadCloser, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(r)
	if err != nil {
		return nil, err
	}

	return c.request(ctx, "POST", "/healthcheck", bytes.NewReader(body.Bytes()))
}

func parseGeneric(r io.ReadCloser, fnProgress, fnResult func(interface{}) error) error {
	var msg tgwriter.Msg

	for dec := json.NewDecoder(r); ; {
		err := dec.Decode(&msg)
		if err != nil {
			return err
		}

		switch msg.Type {
		case "progress":
			err = fnProgress(msg.Payload)
			if err != nil {
				return err
			}

		case "error":
			return errors.New(msg.Error.Message)

		case "result":
			return fnResult(msg.Payload)

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

// ParseRunResponse parses a response from a `run` call
func ParseRunResponse(r io.ReadCloser) (RunResponse, error) {
	var resp RunResponse
	err := parseGeneric(
		r,
		printProgress,
		func(result interface{}) error {
			return mapstructure.Decode(result, &resp)
		},
	)
	return resp, err
}

// ParseListResponse parses a response from a `list` call
func ParseListResponse(r io.ReadCloser) error {
	return parseGeneric(
		r,
		printProgress,
		func(result interface{}) error {
			return nil
		},
	)
}

// ParseBuildResponse parses a response from a `build` call
func ParseBuildResponse(r io.ReadCloser) (BuildResponse, error) {
	var resp BuildResponse
	err := parseGeneric(
		r,
		printProgress,
		func(result interface{}) error {
			return mapstructure.Decode(result, &resp)
		},
	)
	return resp, err
}

// ParseDescribeResponse parses a response from a `describe` call
func ParseDescribeResponse(r io.ReadCloser) error {
	return parseGeneric(
		r,
		printProgress,
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
		func(result interface{}) error {
			return nil
		},
	)
}

// ParseHealthcheckResponse parses a response from a 'healthcheck' call
func ParseHealthcheckResponse(r io.ReadCloser) error {
	return parseGeneric(
		r,
		printProgress,
		func(result interface{}) error {
			return nil
		},
	)
}

func (c *Client) request(ctx context.Context, method string, path string, body io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequest(method, "http://"+c.endpoint+path, body)
	req = req.WithContext(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
