package server

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/ipfs/testground/pkg/daemon/client"
)

func readerToString(r io.Reader) (string, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestIncompatibleBuilder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := ListenAndServe(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	api := client.New(addr)

	req := &client.BuildRequest{
		Plan:    "placebo",
		Builder: "exec:go",
	}

	resp, err := api.Build(ctx, req)
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

	resp, err = api.Run(ctx, &client.RunRequest{
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := ListenAndServe(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	api := client.New(addr)
	req := &client.BuildRequest{
		Plan:    "placebo",
		Builder: "exec:go",
	}

	resp, err := api.Build(ctx, req)
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

	resp, err = api.Run(ctx, &client.RunRequest{
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
