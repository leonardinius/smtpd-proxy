package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/leonardinius/smtpd-proxy/app/cmd"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Main(ctx, os.Args[1:]...); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1) // nolint:gocritic // defer cancel() is not called
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
			slog.Info("SIGQUIT detected", "dump", getDump())
		}
	}()
	signal.Notify(sigChan, syscall.SIGQUIT)
}
