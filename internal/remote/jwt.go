package remote

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/modelcontextprotocol/go-sdk/auth"
)

// jwtVerifier uses the maintained OIDC verifier for signature, exact issuer,
// resource audience and expiry. No token-supplied URL is ever fetched. RS256 is
// deliberately pinned; symmetric/unsigned tokens are not accepted.
func jwtVerifier(cfg config.Config, client *http.Client) auth.TokenVerifier {
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), boundedJWKSClient(client)), cfg.JWKSURL)
	v := oidc.NewVerifier(cfg.Issuer, keys, &oidc.Config{ClientID: cfg.PublicURL, SupportedSigningAlgs: []string{"RS256"}})
	return func(ctx context.Context, raw string, _ *http.Request) (*auth.TokenInfo, error) {
		if len(raw) == 0 || len(raw) > 4096 {
			return nil, auth.ErrInvalidToken
		}
		token, err := v.Verify(ctx, raw)
		if err != nil {
			return nil, auth.ErrInvalidToken
		}
		var claims struct {
			Scope     string `json:"scope"`
			Issued    int64  `json:"iat"`
			Expires   int64  `json:"exp"`
			NotBefore int64  `json:"nbf"`
		}
		if token.Claims(&claims) != nil {
			return nil, auth.ErrInvalidToken
		}
		now := time.Now().Unix()
		if token.Subject != cfg.OAuthSubject || claims.Issued <= 0 || claims.Issued > now+30 || claims.Expires <= now || claims.Expires <= claims.Issued || claims.Expires-claims.Issued > 900 || claims.NotBefore > now {
			return nil, auth.ErrInvalidToken
		}
		// ID tokens normally target the client, not this resource; both the exact
		// resource audience and resource scopes are required at the HTTP boundary.
		return &auth.TokenInfo{UserID: token.Subject, Scopes: strings.Fields(claims.Scope), Expiration: time.Unix(claims.Expires, 0)}, nil
	}
}

// The OIDC library reads a complete JWKS response; bound it before decoding.
type jwksTransport struct{ base http.RoundTripper }

func (t jwksTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(data) > 65536 {
		return nil, errors.New("JWKS response unavailable or oversized")
	}
	res.Body = io.NopCloser(bytes.NewReader(data))
	res.ContentLength = int64(len(data))
	return res, nil
}
func boundedJWKSClient(client *http.Client) *http.Client {
	copy := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = jwksTransport{base: base}
	return &copy
}
