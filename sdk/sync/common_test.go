package sync

import (
	"context"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ipfs/testground/sdk/runtime"
)

func TestMain(m *testing.M) {
	// Avoid collisions in Redis keys over test runs.
	rand.Seed(time.Now().UnixNano())

	// _ = os.Setenv("LOG_LEVEL", "debug")

	// Set fail-fast options.
	DefaultRedisOpts.PoolTimeout = 500 * time.Millisecond
	DefaultRedisOpts.MaxRetries = 0

	closeFn, err := ensureRedis()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	v := m.Run()

	_ = closeFn()
	os.Exit(v)
}

// Check if there's a running instance of redis, or start it otherwise. If we
// start an ad-hoc instance, the close function will terminate it.
func ensureRedis() (func() error, error) {
	// Try to obtain a client; if this fails, we'll attempt to start a redis
	// instance.
	client, err := redisClient(context.Background(), zap.S())
	if err == nil {
		_ = client.Close()
		return func() error { return nil }, err
	}

	cmd := exec.Command("redis-server", "-")
	if err := cmd.Start(); err != nil {
		return func() error { return nil }, fmt.Errorf("failed to start redis: %w", err)
	}

	time.Sleep(1 * time.Second)

	// Try to obtain a client again.
	if client, err = redisClient(context.Background(), zap.S()); err != nil {
		return func() error { return nil }, fmt.Errorf("failed to obtain redis client despite starting instance: %v", err)
	}
	defer client.Close()

	return func() error {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed while stopping test-scoped redis: %s", err)
		}
		return nil
	}, nil
}

// randomRunEnv generates a random RunEnv for testing purposes.
func randomRunEnv() *runtime.RunEnv {
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	_, subnet, _ := net.ParseCIDR("127.1.0.1/16")

	return runtime.NewRunEnv(runtime.RunParams{
		TestPlan:           fmt.Sprintf("testplan-%d", rand.Uint32()),
		TestSidecar:        false,
		TestCase:           fmt.Sprintf("testcase-%d", rand.Uint32()),
		TestRun:            fmt.Sprintf("testrun-%d", rand.Uint32()),
		TestCaseSeq:        int(rand.Uint32()),
		TestRepo:           "github.com/ipfs/go-ipfs",
		TestSubnet:         &runtime.IPNet{IPNet: *subnet},
		TestCommit:         fmt.Sprintf("%x", sha1.Sum(b)),
		TestInstanceCount:  int(1 + (rand.Uint32() % 999)),
		TestInstanceRole:   "",
		TestInstanceParams: make(map[string]string),
	})
}
