package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/server"
)

func main() { os.Exit(run()) }

func run() int {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, cfg); err != nil && ctx.Err() == nil {
		// SDK errors can contain untrusted payloads. Keep diagnostics generic.
		fmt.Fprintln(os.Stderr, "MCP transport stopped with an error")
		return 1
	}
	return 0
}
