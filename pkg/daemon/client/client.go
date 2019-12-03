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
// The response is a stream of `Msg` protocol messages. See `ProcessDescribeResponse()` for specifics.
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
// The response is a stream of `Msg` protocol messages. See `ProcessBuildResponse()` for specifics.
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

// ProcessBuildResponse parses a response from a `build` call
func ProcessBuildResponse(r io.ReadCloser) (string, error) {
	var msg tgwriter.Msg

	for dec := json.NewDecoder(r); ; {
		err := dec.Decode(&msg)
		if err != nil {
			return "", err
		}

		switch msg.Type {
		case "progress":
			m, err := base64.StdEncoding.DecodeString(msg.Payload.(string))
			if err != nil {
				return "", err
			}

			fmt.Print(string(m))

		case "error":
			return "", errors.New(msg.Error.Message)

		case "result":
			var resp BuildResponse
			err := mapstructure.Decode(msg.Payload, &resp)
			if err != nil {
				return "", err
			}

			return resp.ArtifactPath, nil

		default:
			return "", errors.New("unknown message type")
		}
	}
}

// ProcessDescribeResponse parses a response from a `describe` call
func ProcessDescribeResponse(r io.ReadCloser) error {
	var msg tgwriter.Msg

	for dec := json.NewDecoder(r); ; {
		err := dec.Decode(&msg)
		if err != nil {
			return err
		}

		switch msg.Type {
		case "progress":
			m, err := base64.StdEncoding.DecodeString(msg.Payload.(string))
			if err != nil {
				return err
			}

			fmt.Print(string(m))

		case "error":
			return errors.New(msg.Error.Message)

		case "result":
			return nil

		default:
			return errors.New("unknown message type")
		}
	}
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
