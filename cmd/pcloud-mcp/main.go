package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
	"github.com/cyrano-janus/pcloud-mcp/internal/server"
	"github.com/cyrano-janus/pcloud-mcp/internal/service"
	"time"
)

func main() { os.Exit(run()) }

func run() int {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "--version" {
		fmt.Println(server.Version)
		return 0
	}
	if len(os.Args) > 2 || (command != "serve" && command != "check") {
		fmt.Fprintln(os.Stderr, "usage: pcloud-mcp [serve|check|--version]")
		return 2
	}
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if command == "check" {
		api, err := pcloud.New(cfg.Region, cfg.AccessToken)
		if err != nil {
			fmt.Fprintln(os.Stderr, "configure pCloud credentials, region, user and root folder first")
			return 2
		}
		svc := service.New(api, policy.Policy{UserID: cfg.UserID}, cfg.RootFolderID, os.Stderr)
		ctx, cancel := context.WithTimeout(policy.WithPrincipal(ctx, policy.Principal{UserID: cfg.UserID}), 30*time.Second)
		defer cancel()
		if _, err := svc.List(ctx, cfg.RootFolderID, 0, 1); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ok", "user_id": cfg.UserID, "root_folder_id": cfg.RootFolderID})
		return 0
	}
	if err := server.Run(ctx, cfg); err != nil && ctx.Err() == nil {
		// SDK errors can contain untrusted payloads. Keep diagnostics generic.
		fmt.Fprintln(os.Stderr, "MCP transport stopped with an error")
		return 1
	}
	return 0
}
