package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func routingPolicyTest(policy network.RoutingPolicyType) run.InitializedTestCaseFn {
	return func(env *runtime.RunEnv, initCtx *run.InitContext) error {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		netclient := initCtx.NetClient

		config := &network.Config{
			Network:       "default",
			Enable:        true,
			CallbackState: "network-configured-with-policy",
			RoutingPolicy: policy,
		}

		env.RecordMessage("configuring network with network policy: %s", policy)
		netclient.MustConfigureNetwork(ctx, config)

		const (
			url     = "https://bafybeicg2rebjoofv4kbyovkw7af3rpiitvnl6i7ckcywaq6xjcxnc2mby.ipfs.dweb.link/"
			content = "hello world\n"
		)

		tr := &http.Transport{
			// Skipping TLS checks because the container might not have CA certificates.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		httpClient := &http.Client{Transport: tr}
		resp, err := httpClient.Get(url)

		if policy == network.DenyAll {
			if err == nil {
				return fmt.Errorf("http request must not work with traffic blocked")
			}

			env.RecordMessage("connection failed as expected: %s", err)
			return nil
		}

		if err != nil {
			return err
		}
		defer resp.Body.Close()

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if string(bytes) != content {
			return fmt.Errorf("received %s, expected %s", string(bytes), content)
		}

		env.RecordMessage("received message: %s", content)
		return nil
	}
}
