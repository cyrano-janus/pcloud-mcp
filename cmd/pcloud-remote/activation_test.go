package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/credential"
)

func TestRemoteActivationFailsClosed(t *testing.T) {
	for _, value := range []string{"", "false", "true", "yes"} {
		t.Run(value, func(t *testing.T) {
			env := map[string]string{"PCLOUD_MCP_ENABLED": value}
			called := false
			h, err := runtimeHandler(context.Background(), func(k string) (string, bool) { v, ok := env[k]; return v, ok }, func(context.Context, config.Config) (http.Handler, error) { called = true; return nil, nil })
			if called {
				t.Fatal("remote builder reached without complete configuration")
			}
			if value == "" || value == "false" {
				if err != nil {
					t.Fatal(err)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("POST", "/mcp", nil))
				if w.Code != 503 {
					t.Fatal("MCP opened by default")
				}
			} else if err == nil {
				t.Fatal("invalid or incomplete activation accepted")
			}
		})
	}
}

func activationEnvironment(t *testing.T) map[string]string {
	t.Helper()
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	envelope, err := credential.Seal(key, credential.Data{Token: "test-provider-token", Region: "eu", UserID: 7})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"PCLOUD_MCP_ENABLED": "true", "PCLOUD_SETUP_ENABLED": "false", "RENDER": "true", "PORT": "10000",
		"RENDER_EXTERNAL_URL": "https://mcp.example.com", "PCLOUD_CREDENTIAL_ENVELOPE": envelope, "PCLOUD_TOKEN_ENCRYPTION_KEY": key,
		"PCLOUD_USER_ID": "7", "PCLOUD_ROOT_FOLDER_ID": "123", "PCLOUD_MCP_OAUTH_ISSUER": "https://id.example.com/realms/mcp",
		"PCLOUD_MCP_INTROSPECTION_URL": "https://id.example.com/realms/mcp/protocol/openid-connect/token/introspect",
		"PCLOUD_MCP_OAUTH_CLIENT_ID":   "api", "PCLOUD_MCP_OAUTH_CLIENT_SECRET": "test-introspection-secret", "PCLOUD_MCP_OAUTH_SUBJECT": "owner",
	}
}

func TestRemoteActivationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		valid            bool
	}{
		{"configured", "", "", true}, {"invalid activation value", "PCLOUD_MCP_ENABLED", "yes", false}, {"foreign runtime", "RENDER", "false", false}, {"setup still enabled", "PCLOUD_SETUP_ENABLED", "true", false},
		{"writes requested", "PCLOUD_ENABLE_WRITES", "true", false}, {"missing owner", "PCLOUD_MCP_OAUTH_SUBJECT", "", false},
		{"foreign introspection", "PCLOUD_MCP_INTROSPECTION_URL", "https://evil.example/introspect", false},
		{"bad envelope", "PCLOUD_CREDENTIAL_ENVELOPE", "invalid", false}, {"wrong provider owner", "PCLOUD_USER_ID", "8", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := activationEnvironment(t)
			if tc.key != "" {
				env[tc.key] = tc.value
			}
			called := false
			h, err := runtimeHandler(context.Background(), func(k string) (string, bool) { v, ok := env[k]; return v, ok }, func(_ context.Context, cfg config.Config) (http.Handler, error) {
				called = true
				if cfg.Transport != "http" || !cfg.RenderProxy || cfg.EnableWrites || cfg.RootFolderID != 123 || cfg.PublicURL != "https://mcp.example.com/mcp" {
					t.Fatal("unsafe derived configuration")
				}
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }), nil
			})
			if called != tc.valid || (err == nil) != tc.valid {
				t.Fatalf("called=%v err=%v", called, err)
			}
			if tc.valid {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("POST", "/mcp", nil))
				if w.Code != 401 {
					t.Fatal("remote boundary bypassed")
				}
			}
		})
	}
}

func TestRemoteJWTActivationWithoutIntrospectionCredentials(t *testing.T) {
	env := activationEnvironment(t)
	delete(env, "PCLOUD_MCP_INTROSPECTION_URL")
	delete(env, "PCLOUD_MCP_OAUTH_CLIENT_ID")
	delete(env, "PCLOUD_MCP_OAUTH_CLIENT_SECRET")
	env["PCLOUD_MCP_JWKS_URL"] = "https://id.example.com/keys"
	called := false
	_, err := runtimeHandler(context.Background(), func(k string) (string, bool) { v, ok := env[k]; return v, ok }, func(_ context.Context, cfg config.Config) (http.Handler, error) {
		called = true
		if cfg.JWKSURL != "https://id.example.com/keys" {
			t.Fatal("missing JWT config")
		}
		return http.NotFoundHandler(), nil
	})
	if err != nil || !called {
		t.Fatal("JWT activation failed", err)
	}
}
