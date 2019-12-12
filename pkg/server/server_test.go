package server

import (
	"context"
	"testing"

	"github.com/ipfs/testground/pkg/daemon/client"
)

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

	buildRes, err := client.ParseBuildResponse(resp)
	if err != nil {
		t.Fatal(err)
		return
	}

	resp, err = api.Run(ctx, &client.RunRequest{
		Plan:         "placebo",
		Case:         "ok",
		Runner:       "local:exec",
		Instances:    1,
		BuilderID:    "local:docker", // local:docker is incompatible with local:exec
		ArtifactPath: buildRes.ArtifactPath,
	})
	t.Log(err)
	defer resp.Close()

	err = client.ParseRunResponse(resp)
	if err == nil {
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

	buildRes, err := client.ParseBuildResponse(resp)
	if err != nil {
		t.Fatal(err)
		return
	}

	resp, err = api.Run(ctx, &client.RunRequest{
		Plan:         "placebo",
		Case:         "ok",
		Runner:       "local:exec",
		Instances:    1,
		BuilderID:    buildRes.BuilderID,
		ArtifactPath: buildRes.ArtifactPath,
	})
	t.Log(err)
	defer resp.Close()

	err = client.ParseRunResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
}
