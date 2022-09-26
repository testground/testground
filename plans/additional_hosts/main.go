package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"additional_hosts": run.InitializedTestCaseFn(additionalHosts),
	})
}

func additionalHosts(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := http.Get("http://http_server:8080")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("non 200 error code")
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if string(body) != "ok" {
		return errors.New("unexpected response")
	}
	return nil
}
