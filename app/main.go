package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/leonardinius/smtpd-proxy/app/cmd"
	"github.com/leonardinius/smtpd-proxy/app/zlog"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Main(ctx, os.Args[1:]...); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

// getDump reads runtime stack and returns as a string
func getDump() string {
	maxSize := 5 * 1024 * 1024
	stacktrace := make([]byte, maxSize)
	length := runtime.Stack(stacktrace, true)
	if length > maxSize {
		length = maxSize
	}
	return string(stacktrace[:length])
}

// nolint:gochecknoinits // can't avoid it in this place
func init() {
	// catch SIGQUIT and print stack traces
	sigChan := make(chan os.Signal, 1)
	go func() {
		for range sigChan {
			zlog.Infof("SIGQUIT detected, dump:\n%s", getDump())
		}
	}()
	signal.Notify(sigChan, syscall.SIGQUIT)
}
