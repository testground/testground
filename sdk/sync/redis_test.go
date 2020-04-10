package sync

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
)

func TestRedisHost(t *testing.T) {
	realRedisHost := os.Getenv(EnvRedisHost)
	defer os.Setenv(EnvRedisHost, realRedisHost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = os.Setenv(EnvRedisHost, "redis-does-not-exist.example.com")
	client, err := redisClient(ctx, zap.S())
	if err == nil {
		_ = client.Close()
		t.Error("should not have found redis host")
	}

	_ = os.Setenv(EnvRedisHost, "redis-does-not-exist.example.com")
	client, err = redisClient(ctx, zap.S())
	if err == nil {
		_ = client.Close()
		t.Error("should not have found redis host")
	}

	realHost := realRedisHost
	if realHost == "" {
		realHost = "localhost"
	}
	_ = os.Setenv(EnvRedisHost, realHost)
	client, err = redisClient(ctx, zap.S())
	if err != nil {
		t.Errorf("should have found the redis host, failed with: %s", err)
	}
	addr := client.Options().Addr
	_ = client.Close()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	hostIP := net.ParseIP(host)
	if hostIP == nil {
		t.Fatal("expected host to be an IP")
	}
	addrs, err := net.LookupIP(realHost)
	if err != nil {
		t.Fatal("failed to resolve redis host")
	}
	for _, a := range addrs {
		if a.Equal(hostIP) {
			// Success!
			return
		}
	}
	t.Fatal("redis address not found in list of addresses")
}

func TestConnUnblock(t *testing.T) {
	closeFn := ensureRedis(t)
	defer closeFn()

	client := redis.NewClient(&redis.Options{})
	c := client.Conn()
	id, _ := c.ClientID().Result()

	ch := make(chan struct{})
	go func() {
		timer := time.AfterFunc(1*time.Second, func() { close(ch) })
		_, _ = c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result()
		timer.Stop()
	}()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("XREAD unexpectedly returned early")
	}

	unblocked, err := client.ClientUnblock(id).Result()
	if err != nil {
		t.Fatal(err)
	}
	if unblocked != 1 {
		t.Fatal("expected CLIENT UNBLOCK to return 1")
	}
	for i := 0; i < 10; i++ {
		id2, err := c.ClientID().Result()
		if err != nil {
			t.Fatal(err)
		}
		if id != id2 {
			t.Errorf("expected client id to be: %d, was: %d", id, id2)
		}
	}
}

func TestNewClient2(t *testing.T) {
	t.Skip()

	client := redis.NewClient(&redis.Options{})
	c := client.Conn()
	c.Close()
	c = client.Conn()
	fmt.Println(c.ClientID().Result())

	fmt.Println(c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result())
	fmt.Println(c.ClientID().Result())

	fmt.Println(c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result())
	fmt.Println(c.ClientID().Result())

	fmt.Println(c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result())
	fmt.Println(c.ClientID().Result())

	fmt.Println(c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result())
	fmt.Println(c.ClientID().Result())
}

func TestNewClient3(t *testing.T) {
	t.Skip()
	client := redis.NewClient(&redis.Options{
		MaxRetries:  5,
		PoolSize:    1,
		PoolTimeout: 10 * time.Second,
	})
	c1 := client.Conn()
	c1.Close()

	c2 := client.Conn()
	fmt.Println(c2.ClientID().Result())

	go func() {
		fmt.Println(c2.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result())
		fmt.Println(c2.ClientID().Result())
	}()

	time.Sleep(1 * time.Second)
	c3 := client.Conn()
	fmt.Println(c3.ClientID().Result())
}
