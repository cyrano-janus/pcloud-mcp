// Package server owns the MCP protocol boundary, not provider HTTP calls.
package server

import (
	"context"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
	"github.com/cyrano-janus/pcloud-mcp/internal/remote"
	"github.com/cyrano-janus/pcloud-mcp/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
)

const Version = "0.3.0-rc.1"

func Run(ctx context.Context, cfg config.Config) error {
	var svc *service.Service
	if cfg.AccessToken != "" {
		api, err := pcloud.New(cfg.Region, cfg.AccessToken)
		if err != nil {
			return err
		}
		svc = service.New(api, policy.Policy{UserID: cfg.UserID, EnableWrites: cfg.EnableWrites}, cfg.RootFolderID, os.Stderr)
	}
	s := New(cfg, svc)
	if cfg.Transport == "http" {
		return remote.Run(ctx, cfg, s)
	}
	return s.Run(ctx, &mcp.StdioTransport{MaxLineLength: cfg.MaxFrameBytes})
}

func New(cfg config.Config, svc *service.Service) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "pcloud-mcp", Version: Version}, &mcp.ServerOptions{
		SupportedProtocolVersions: []string{"2026-07-28"},
		SetCacheable:              func(_ context.Context, _ mcp.Request, c *mcp.Cacheable) { c.TTLMs = 0; c.CacheScope = "private" },
	})
	if svc != nil {
		register(s, cfg, svc)
	}
	return s
}
