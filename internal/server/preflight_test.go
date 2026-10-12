package server

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/model"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
	"github.com/cyrano-janus/pcloud-mcp/internal/service"
)

type preflightFixture struct {
	apiFixture
	owner int64
	mine  bool
}

func (a preflightFixture) Whoami(context.Context) (model.Identity, error) {
	return model.Identity{UserID: a.owner}, nil
}
func (a preflightFixture) ListFolder(_ context.Context, id int64) (model.Metadata, error) {
	return model.Metadata{FolderID: id, IsFolder: true, IsMine: a.mine}, nil
}
func TestProviderPreflightRejectsUnownedRoot(t *testing.T) {
	cfg := config.Config{Transport: "http", PublicURL: "https://mcp.example.com/mcp", Issuer: "https://id.example.com", JWKSURL: "https://id.example.com/keys", OAuthSubject: "owner", UserID: 7, RootFolderID: 10, Listen: "127.0.0.1:8080", MaxFrameBytes: 1048576}
	for _, tc := range []struct {
		name        string
		owner       int64
		mine, valid bool
	}{{"owner root", 7, true, true}, {"foreign account", 8, true, false}, {"unowned root", 7, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.New(preflightFixture{owner: tc.owner, mine: tc.mine}, policy.Policy{UserID: 7}, 10, io.Discard)
			h, err := checkedHTTPHandler(context.Background(), cfg, svc)
			if (err == nil) != tc.valid || (h != nil) != tc.valid {
				t.Fatalf("unsafe preflight: handler=%v err=%v", h != nil, err)
			}
			if tc.valid {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("POST", cfg.PublicURL, nil))
				if w.Code != 401 {
					t.Fatal("unauthenticated handler exposed")
				}
			}
		})
	}
}
