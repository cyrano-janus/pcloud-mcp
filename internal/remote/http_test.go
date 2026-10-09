package remote

import (
	"context"
	"encoding/json"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func remoteConfig() config.Config {
	return config.Config{PublicURL: "https://mcp.example.com/mcp", Issuer: "https://id.example.com", IntrospectionURL: "https://id.example.com/introspect", OAuthClientID: "server", OAuthClientSecret: "client-secret", OAuthSubject: "owner", UserID: 7, MaxFrameBytes: 1048576, Listen: "127.0.0.1:8080"}
}

func TestAuthenticatedHTTPPrincipalAndScope(t *testing.T) {
	for _, scope := range []string{"files:read", "files:read files:write", "files:write"} {
		t.Run(scope, func(t *testing.T) {
			cfg := remoteConfig()
			srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
			type output struct {
				UserID int64 `json:"user_id"`
				Write  bool  `json:"write"`
			}
			mcp.AddTool(srv, &mcp.Tool{Name: "principal", Description: "test principal"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, output, error) {
				p := policy.FromContext(ctx)
				return nil, output{p.UserID, p.Write}, nil
			})
			httpClient := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
				data := claims()
				data["scope"] = scope
				b, _ := json.Marshal(data)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
			})}
			handler, err := newHandler(cfg, srv, httpClient)
			if err != nil {
				t.Fatal(err)
			}
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"principal","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"http-test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
			req := httptest.NewRequest("POST", cfg.PublicURL, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("Authorization", "Bearer opaque-token")
			req.Header.Set("MCP-Protocol-Version", "2026-07-28")
			req.Header.Set("Mcp-Method", "tools/call")
			req.Header.Set("Mcp-Name", "principal")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if scope == "files:write" {
				if res.Code != 403 {
					t.Fatalf("missing read scope accepted: %d", res.Code)
				}
				return
			}
			if res.Code != 200 {
				t.Fatalf("authenticated request failed: %d %s", res.Code, res.Body.String())
			}
			var decoded struct {
				Result struct {
					Output output `json:"structuredContent"`
				} `json:"result"`
			}
			if json.Unmarshal(res.Body.Bytes(), &decoded) != nil || decoded.Result.Output.UserID != 7 || decoded.Result.Output.Write != (scope == "files:read files:write") {
				t.Fatalf("principal not bound: %s", res.Body.String())
			}
		})
	}
}
func claims() map[string]any {
	return map[string]any{"active": true, "iss": "https://id.example.com", "sub": "owner", "aud": "https://mcp.example.com/mcp", "exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "scope": "files:read", "token_type": "Bearer"}
}
func TestTokenValidation(t *testing.T) {
	for _, test := range []struct {
		name, key string
		value     any
		valid     bool
	}{
		{"valid", "", nil, true}, {"inactive", "active", false, false}, {"wrong audience", "aud", "https://other.example.com/mcp", false}, {"wrong issuer", "iss", "https://evil.example.com", false}, {"wrong subject", "sub", "other", false}, {"expired", "exp", time.Now().Add(-time.Minute).Unix(), false}, {"unbounded lifetime", "exp", time.Now().Add(time.Hour).Unix(), false}, {"missing expiry", "exp", 0, false}, {"future token", "nbf", time.Now().Add(time.Hour).Unix(), false}, {"refresh token", "token_type", "refresh_token", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := claims()
			if test.key != "" {
				data[test.key] = test.value
			}
			c := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.RawQuery != "" {
					t.Fatal("token in URL")
				}
				_ = r.ParseForm()
				if r.Form.Get("token") != "opaque-token" {
					t.Fatal("missing introspection token")
				}
				b, _ := json.Marshal(data)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
			})}
			v := verifier(remoteConfig(), c)
			_, err := v(context.Background(), "opaque-token", nil)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
		})
	}
}
func TestHTTPBoundary(t *testing.T) {
	cfg := remoteConfig()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	h, err := newHandler(cfg, srv, &http.Client{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, host, origin string
		status             int
	}{
		{"/mcp", "mcp.example.com", "", 401}, {"/mcp", "evil.example.com", "", 403}, {"/mcp", "mcp.example.com", "https://evil.example.com", 403}, {"/mcp?access_token=secret", "mcp.example.com", "", 400}, {"/.well-known/oauth-protected-resource/mcp", "mcp.example.com", "", 200}, {"/healthz", "mcp.example.com", "", 200},
	} {
		r := httptest.NewRequest("GET", "https://"+tc.host+tc.path, nil)
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
	}
}
func TestConfigurationRequiresTLS(t *testing.T) {
	cfg := remoteConfig()
	cfg.Listen = "0.0.0.0:8080"
	if err := Validate(cfg); err == nil {
		t.Fatal("plaintext public listener accepted")
	}
	cfg = remoteConfig()
	cfg.IntrospectionURL = "https://evil.example.com/introspect"
	if err := Validate(cfg); err == nil {
		t.Fatal("cross-host credential destination accepted")
	}
}

func TestRenderTLSBoundary(t *testing.T) {
	cfg := remoteConfig()
	cfg.Listen = "0.0.0.0:10000"
	if Validate(cfg) == nil {
		t.Fatal("public plaintext enabled without proxy mode")
	}
	cfg.RenderProxy = true
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.TLSCert = "cert"
	cfg.TLSKey = "key"
	if Validate(cfg) == nil {
		t.Fatal("ambiguous Render TLS configuration accepted")
	}
}
