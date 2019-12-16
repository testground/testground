package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	processContext     context.Context
	processContextOnce sync.Once
)

func ProcessContext() context.Context {
	processContextOnce.Do(func() {
		var cancel context.CancelFunc
		processContext, cancel = context.WithCancel(context.Background())

		notify := make(chan os.Signal, 2)
		signal.Notify(notify, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			defer signal.Stop(notify)

			<-notify
			cancel()

			select {
			case <-time.After(30 * time.Second):
				fmt.Println("Timed out on shutdown, terminating...")
			case <-notify:
				fmt.Println("Received another interrupt before graceful shutdown, terminating...")
			}
			os.Exit(-1)
		}()
	})
	return processContext
}
