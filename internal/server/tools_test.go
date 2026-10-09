package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/model"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
	"github.com/cyrano-janus/pcloud-mcp/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
	"time"
)

type apiFixture struct{}

func (apiFixture) Whoami(context.Context) (model.Identity, error) {
	return model.Identity{UserID: 7}, nil
}
func (apiFixture) ListFolder(_ context.Context, id int64) (model.Metadata, error) {
	return model.Metadata{FolderID: id, IsFolder: true, IsMine: true, Contents: []model.Metadata{{FileID: 11, Name: "a.txt", IsMine: true, ParentFolderID: id, Size: 5}}}, nil
}
func (apiFixture) Stat(context.Context, int64) (model.Metadata, error) {
	return model.Metadata{FileID: 11, Name: "a.txt", IsMine: true, ParentFolderID: 10, Size: 5}, nil
}
func (apiFixture) Text(context.Context, int64) (string, error) { return "hello", nil }
func (apiFixture) CreateFolder(context.Context, int64, string) (model.Metadata, error) {
	return model.Metadata{FolderID: 12, IsMine: true, IsFolder: true, ParentFolderID: 10, Name: "new"}, nil
}
func (apiFixture) Copy(context.Context, int64, int64, string) (model.Metadata, error) {
	return model.Metadata{FileID: 13, IsMine: true, ParentFolderID: 10, Name: "copy.txt"}, nil
}
func (apiFixture) Upload(_ context.Context, _ int64, name string, data []byte) (model.Metadata, error) {
	return model.Metadata{FileID: 14, Name: name, IsMine: true, ParentFolderID: 10, Size: int64(len(data))}, nil
}
func session(t *testing.T, cfg config.Config) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	svc := service.New(apiFixture{}, policy.Policy{UserID: 7, EnableWrites: cfg.EnableWrites}, 10, &bytes.Buffer{})
	srv := New(cfg, svc)
	a, b := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
func TestToolsEndToEnd(t *testing.T) {
	cs := session(t, config.Config{UserID: 7, EnableWrites: true})
	ctx := context.Background()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 9 {
		t.Fatalf("tool count %v %v", listed, err)
	}
	sum := sha256.Sum256([]byte("hello"))
	upload := map[string]any{"folder_id": 10, "name": "hello.txt", "data_base64": base64.StdEncoding.EncodeToString([]byte("hello")), "sha256": hex.EncodeToString(sum[:])}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"whoami", map[string]any{}}, {"list_folder", map[string]any{"folder_id": 10}}, {"get_file_info", map[string]any{"file_id": 11}}, {"search_files", map[string]any{"folder_id": 10, "name": "a"}}, {"read_text_file", map[string]any{"file_id": 11}}, {"create_folder", map[string]any{"folder_id": 10, "name": "new"}}, {"copy_file", map[string]any{"file_id": 11, "folder_id": 10, "name": "copy.txt"}}, {"upload_file", upload}, {"save_artifact", upload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil || result.IsError {
				t.Fatalf("tool failed: result=%+v err=%v", result, err)
			}
		})
	}
}
func TestReadOnlyRegistration(t *testing.T) {
	cs := session(t, config.Config{UserID: 7})
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(listed.Tools) != 5 {
		t.Fatal("read-only tool surface incorrect", err)
	}
}
func TestHTTPDoesNotTrustLocalOwner(t *testing.T) {
	cs := session(t, config.Config{Transport: "http", UserID: 7, EnableWrites: true})
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "whoami", Arguments: map[string]any{}})
	if err == nil && !result.IsError {
		t.Fatal("HTTP call without verified principal accepted")
	}
}
