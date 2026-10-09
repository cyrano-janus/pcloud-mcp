// pcloud-upload is a real MCP client for safely delivering a local artifact.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() { os.Exit(run()) }
func run() int {
	source := flag.String("file", "", "Local file to upload, maximum 4 MiB")
	folder := flag.Int64("folder", -1, "Destination pCloud folder ID")
	name := flag.String("name", "", "Destination name (default: local filename)")
	server := flag.String("server", "./bin/pcloud-mcp", "Path to pcloud-mcp executable")
	flag.Parse()
	if *source == "" || *folder < 0 {
		fmt.Fprintln(os.Stderr, "usage: pcloud-upload -file FILE -folder ID [-name NAME] [-server BINARY]")
		return 2
	}
	file, err := os.Open(*source)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read source file")
		return 1
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > pcloud.MaxUploadBytes {
		fmt.Fprintln(os.Stderr, "source must be a regular file up to 4 MiB")
		return 2
	}
	data, err := io.ReadAll(io.LimitReader(file, pcloud.MaxUploadBytes+1))
	if err != nil || len(data) > pcloud.MaxUploadBytes {
		fmt.Fprintln(os.Stderr, "source exceeds limit or is unreadable")
		return 1
	}
	if *name == "" {
		*name = filepath.Base(*source)
	}
	sum := sha256.Sum256(data)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, *server)
	cmd.Stderr = os.Stderr
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PCLOUD_MCP_TRANSPORT=") && !strings.HasPrefix(entry, "PCLOUD_MCP_MAX_FRAME_BYTES=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "PCLOUD_MCP_TRANSPORT=stdio", "PCLOUD_MCP_MAX_FRAME_BYTES=8388608")
	client := mcp.NewClient(&mcp.Implementation{Name: "pcloud-artifact-uploader", Version: "0.3.0-rc.1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot connect to MCP server; check configuration")
		return 1
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "save_artifact", Arguments: map[string]any{"folder_id": *folder, "name": *name, "data_base64": base64.StdEncoding.EncodeToString(data), "sha256": hex.EncodeToString(sum[:])}})
	if err != nil {
		fmt.Fprintln(os.Stderr, "MCP call failed; inspect pCloud before retrying")
		return 1
	}
	if result.IsError {
		fmt.Fprintln(os.Stderr, "upload refused or failed; inspect pCloud before retrying")
		return 1
	}
	_ = json.NewEncoder(os.Stdout).Encode(result.StructuredContent)
	return 0
}
