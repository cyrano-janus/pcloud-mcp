package remote

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestJWTSignatureAndBinding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := remoteConfig()
	cfg.IntrospectionURL = ""
	cfg.OAuthClientID = ""
	cfg.OAuthClientSecret = ""
	cfg.JWKSURL = "https://id.example.com/keys"
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("at+jwt").WithHeader("kid", "test"))
	if err != nil {
		t.Fatal(err)
	}
	otherSigner, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: other}, (&jose.SignerOptions{}).WithHeader("kid", "test"))
	if err != nil {
		t.Fatal(err)
	}
	jwks, _ := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != cfg.JWKSURL || r.Header.Get("Authorization") != "" {
			t.Fatal("unsafe JWKS fetch")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(jwks)))}, nil
	})}
	verify := jwtVerifier(cfg, client)
	for _, tc := range []struct {
		name, claim string
		value       any
		foreign     bool
		valid       bool
	}{
		{"valid", "", nil, false, true}, {"wrong signature", "", nil, true, false}, {"wrong subject", "sub", "stranger", false, false},
		{"wrong issuer", "iss", "https://evil.example", false, false}, {"wrong audience", "aud", "another-api", false, false},
		{"expired", "exp", time.Now().Add(-time.Minute).Unix(), false, false}, {"long lifetime", "exp", time.Now().Add(time.Hour).Unix(), false, false},
		{"missing issued", "iat", 0, false, false}, {"future issued", "iat", time.Now().Add(time.Minute).Unix(), false, false},
		{"future nbf", "nbf", time.Now().Add(time.Minute).Unix(), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := claims()
			delete(data, "active")
			delete(data, "token_type")
			if tc.claim != "" {
				data[tc.claim] = tc.value
			}
			b, _ := json.Marshal(data)
			use := signer
			if tc.foreign {
				use = otherSigner
			}
			signed, err := use.Sign(b)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := signed.CompactSerialize()
			if err != nil {
				t.Fatal(err)
			}
			info, err := verify(context.Background(), raw, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && (info.UserID != "owner" || len(info.Scopes) != 1 || info.Scopes[0] != "files:read") {
				t.Fatal("invalid principal")
			}
		})
	}
	for _, raw := range []string{"", "eyJhbGciOiJub25lIn0.e30.", strings.Repeat("a", 4097)} {
		if _, err := verify(context.Background(), raw, nil); err == nil {
			t.Fatal("malformed JWT accepted")
		}
	}
	// Even a cryptographically valid resource token may not bypass scopes.
	data := claims()
	data["scope"] = "openid"
	b, _ := json.Marshal(data)
	signed, _ := signer.Sign(b)
	raw, _ := signed.CompactSerialize()
	h, err := newHandler(cfg, mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil), client)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", cfg.PublicURL, nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("missing read scope accepted: %d", w.Code)
	}
}

func TestJWKSUnavailableOrOversizedFailsClosed(t *testing.T) {
	cfg := remoteConfig()
	cfg.IntrospectionURL = ""
	cfg.JWKSURL = "https://id.example.com/keys"
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "test"))
	data, _ := json.Marshal(claims())
	signed, _ := signer.Sign(data)
	raw, _ := signed.CompactSerialize()
	for _, body := range []string{"{}", strings.Repeat("x", 65537)} {
		client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if _, err := jwtVerifier(cfg, client)(context.Background(), raw, nil); err == nil {
			t.Fatal("invalid JWKS accepted")
		}
	}
}

func TestJWTConfigurationRejectsAmbiguousVerifier(t *testing.T) {
	cfg := remoteConfig()
	cfg.JWKSURL = "https://id.example.com/keys"
	if Validate(cfg) == nil {
		t.Fatal("two verifiers accepted")
	}
	cfg.IntrospectionURL = ""
	cfg.JWKSURL = "https://evil.example/keys"
	if Validate(cfg) == nil {
		t.Fatal("foreign JWKS host accepted")
	}
	cfg.JWKSURL = "https://id.example.com/keys"
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}

func FuzzJWTRejectsUntrustedInput(f *testing.F) {
	cfg := remoteConfig()
	cfg.IntrospectionURL = ""
	cfg.JWKSURL = "https://id.example.com/keys"
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"keys":[]}`))}, nil
	})}
	verify := jwtVerifier(cfg, client)
	for _, seed := range []string{"", "eyJhbGciOiJub25lIn0.e30.", "a.b.c", strings.Repeat("x", 4097)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if _, err := verify(context.Background(), raw, nil); err == nil {
			t.Fatal("token accepted without any trusted signing key")
		}
	})
}
