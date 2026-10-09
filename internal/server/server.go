// Package server owns the MCP protocol boundary. No pCloud operations are
// registered in M0a; the SDK rejects unregistered tools.
package server

import (
	"context"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "0.2.0-m0a"

func Run(ctx context.Context, cfg config.Config) error {
	s := mcp.NewServer(&mcp.Implementation{Name: "pcloud-mcp", Version: Version}, &mcp.ServerOptions{
		SupportedProtocolVersions: []string{"2026-07-28"},
	})
	return s.Run(ctx, &mcp.StdioTransport{MaxLineLength: cfg.MaxFrameBytes})
}
