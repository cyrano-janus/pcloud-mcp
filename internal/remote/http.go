// Package remote is an OAuth resource server for one explicitly bound owner.
// Token issuance, consent, PKCE, rotation and revocation belong to an external
// OAuth authorization server. Introspection is performed on every request.
package remote

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/netguard"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/time/rate"
)

func secureURL(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("absolute HTTPS URL without credentials, query or fragment required")
	}
	return u, nil
}
func Validate(cfg config.Config) error {
	public, err := secureURL(cfg.PublicURL)
	if err != nil || public.Path != "/mcp" {
		return errors.New("public resource URL must be HTTPS with path /mcp")
	}
	issuer, err := secureURL(cfg.Issuer)
	if err != nil {
		return err
	}
	endpoint, err := secureURL(cfg.IntrospectionURL)
	if err != nil {
		return err
	}
	if endpoint.Host != issuer.Host {
		return errors.New("introspection endpoint must share the configured issuer host")
	}
	if cfg.OAuthClientID == "" || cfg.OAuthClientSecret == "" || cfg.OAuthSubject == "" || cfg.UserID <= 0 {
		return errors.New("complete OAuth owner binding required")
	}
	host, _, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		return errors.New("invalid listener")
	}
	if cfg.RenderProxy && (cfg.TLSCert != "" || cfg.TLSKey != "") {
		return errors.New("Render terminates TLS at its edge; do not configure local certificates")
	}
	if (cfg.TLSCert == "") != (cfg.TLSKey == "") {
		return errors.New("both TLS certificate and key required")
	}
	if cfg.TLSCert == "" && !cfg.RenderProxy {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("plaintext listener must use an explicit loopback IP behind local TLS termination")
		}
	}
	return nil
}

type audience []string

func (a *audience) UnmarshalJSON(data []byte) error {
	var single string
	if json.Unmarshal(data, &single) == nil {
		*a = []string{single}
		return nil
	}
	return json.Unmarshal(data, (*[]string)(a))
}
func verifier(cfg config.Config, client *http.Client) auth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if len(token) == 0 || len(token) > 4096 {
			return nil, auth.ErrInvalidToken
		}
		body := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
		req, err := http.NewRequestWithContext(ctx, "POST", cfg.IntrospectionURL, strings.NewReader(body.Encode()))
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		req.SetBasicAuth(cfg.OAuthClientID, cfg.OAuthClientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := client.Do(req)
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, auth.ErrInvalidToken
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(data) > 65536 {
			return nil, auth.ErrInvalidToken
		}
		var info struct {
			Active    bool     `json:"active"`
			Issuer    string   `json:"iss"`
			Subject   string   `json:"sub"`
			Audience  audience `json:"aud"`
			Expires   int64    `json:"exp"`
			Issued    int64    `json:"iat"`
			NotBefore int64    `json:"nbf"`
			Scope     string   `json:"scope"`
			Type      string   `json:"token_type"`
		}
		if json.Unmarshal(data, &info) != nil {
			return nil, auth.ErrInvalidToken
		}
		now := time.Now().Unix()
		if !info.Active || info.Issuer != cfg.Issuer || info.Subject != cfg.OAuthSubject || !slices.Contains(info.Audience, cfg.PublicURL) || !strings.EqualFold(info.Type, "Bearer") || info.Expires <= now || info.Issued <= 0 || info.Issued > now+30 || info.Expires-info.Issued > 900 || info.NotBefore > now {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{UserID: info.Subject, Scopes: strings.Fields(info.Scope), Expiration: time.Unix(info.Expires, 0)}, nil
	}
}
func Handler(cfg config.Config, server *mcp.Server) (http.Handler, error) {
	return newHandler(cfg, server, netguard.Client(10*time.Second))
}
func newHandler(cfg config.Config, server *mcp.Server, client *http.Client) (http.Handler, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	public, _ := url.Parse(cfg.PublicURL)
	origin := public.Scheme + "://" + public.Host
	metadataURL := origin + "/.well-known/oauth-protected-resource/mcp"
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: int64(cfg.MaxFrameBytes), PropagateRequestCancellation: true, DisableLocalhostProtection: true})
	// Exact Host/Origin checks below replace the SDK's localhost heuristic, so a
	// loopback TLS proxy can use the external resource hostname safely.
	bound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := auth.TokenInfoFromContext(r.Context())
		if info == nil {
			http.Error(w, "unauthorized", 401)
			return
		}
		ctx := policy.WithPrincipal(r.Context(), policy.Principal{UserID: cfg.UserID, Write: slices.Contains(info.Scopes, "files:write")})
		stream.ServeHTTP(w, r.WithContext(ctx))
	})
	secured := auth.RequireBearerToken(verifier(cfg, client), &auth.RequireBearerTokenOptions{ResourceMetadataURL: metadataURL, Scopes: []string{"files:read"}})(bound)
	mux := http.NewServeMux()
	mux.Handle("/mcp", secured)
	scopes := []string{"files:read"}
	if cfg.EnableWrites {
		scopes = append(scopes, "files:write")
	}
	metadata := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]any{"resource": cfg.PublicURL, "authorization_servers": []string{cfg.Issuer}, "scopes_supported": scopes, "bearer_methods_supported": []string{"header"}})
	})
	mux.Handle("/.well-known/oauth-protected-resource/mcp", metadata)
	mux.Handle("/.well-known/oauth-protected-resource", metadata)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	limit := rate.NewLimiter(10, 20)
	slots := make(chan struct{}, 8)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !strings.EqualFold(r.Host, public.Host) || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin) {
			http.Error(w, "host or origin denied", 403)
			return
		}
		if r.URL.RawQuery != "" {
			http.Error(w, "query parameters not accepted", 400)
			return
		}
		if len(r.Header.Values("Authorization")) > 1 {
			http.Error(w, "ambiguous authorization", 400)
			return
		}
		if !limit.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit", 429)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "server busy", 503)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	}), nil
}
func Run(ctx context.Context, cfg config.Config, server *mcp.Server) error {
	handler, err := Handler(cfg, server)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	done := make(chan error, 1)
	go func() {
		if cfg.TLSCert != "" {
			done <- srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			done <- srv.ListenAndServe()
		}
	}()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := srv.Shutdown(shutdown)
		if err != nil {
			_ = srv.Close()
		}
		<-done
		return err
	}
}
