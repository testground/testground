package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/ipfs/testground/pkg/daemon/client"
)

var srv *Server

func getServer() string {
	if srv == nil {
		srv = New(":51000")

		go func() {
			err := srv.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	return ":51000"
}

func readerToString(r io.Reader) (string, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestIncompatibleBuilder(t *testing.T) {
	api := client.New(getServer())

	req := &client.BuildRequest{
		Plan:    "do-nothing",
		Builder: "exec:go",
	}

	resp, err := api.Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer resp.Close()

	buildRes, err := client.ProcessBuildResponse(resp)
	if err != nil {
		t.Fatal(err)
		return
	}

	resp, err = api.Run(context.Background(), &client.RunRequest{
		Plan:         "smlbench",
		Case:         "simple-add",
		Runner:       "local:exec",
		Instances:    1,
		BuilderID:    "local:docker", // local:docker is incompatible with local:exec
		ArtifactPath: buildRes.ArtifactPath,
	})
	t.Log(err)
	defer resp.Close()

	txt, err := readerToString(resp)
	if err != nil {
		t.Fatal(err)
	}

	// Empty response means failures.
	if txt != "" {
		t.Fail()
	}
}

func TestCompatibleBuilder(t *testing.T) {
	api := client.New(getServer())

	req := &client.BuildRequest{
		Plan:    "do-nothing",
		Builder: "exec:go",
	}

	resp, err := api.Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer resp.Close()

	buildRes, err := client.ProcessBuildResponse(resp)
	if err != nil {
		t.Fatal(err)
		return
	}

	resp, err = api.Run(context.Background(), &client.RunRequest{
		Plan:         "smlbench",
		Case:         "simple-add",
		Runner:       "local:exec",
		Instances:    1,
		BuilderID:    buildRes.BuilderID,
		ArtifactPath: buildRes.ArtifactPath,
	})
	t.Log(err)
	defer resp.Close()

	txt, err := readerToString(resp)
	if err != nil {
		t.Fatal(err)
	}

	// Empty response means failures.
	if txt == "" {
		t.Fail()
	}
}
