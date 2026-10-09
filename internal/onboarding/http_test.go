package onboarding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cyrano-janus/pcloud-mcp/internal/credential"
)

func fixture(t *testing.T) (*Handler, *http.Cookie, string, *int) {
	t.Helper()
	cfg := Config{Origin: "https://service.example", ClientID: "app", ClientSecret: "client-secret", SetupSecret: strings.Repeat("ab", 32), EncryptionKey: strings.Repeat("cd", 32)}
	calls := new(int)
	h, err := newHandler(cfg, func(context.Context, string, string, string, string) (credential.Data, error) {
		*calls++
		return credential.Data{Token: "never-show-this-token", Region: "eu", UserID: 7}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", cfg.Origin+"/setup/pcloud", nil))
	if r.Code != 200 {
		t.Fatal(r.Code)
	}
	cookie := r.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe cookie")
	}
	csrf := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(r.Body.String())[1]
	return h, cookie, csrf, calls
}
func start(h *Handler, c *http.Cookie, csrf, secret, origin string) *httptest.ResponseRecorder {
	form := url.Values{"csrf": {csrf}, "setup_secret": {secret}}
	req := httptest.NewRequest("POST", h.cfg.Origin+"/setup/pcloud/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	if c != nil {
		req.AddCookie(c)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, req)
	return r
}
func callback(h *Handler, c *http.Cookie, state string) *httptest.ResponseRecorder {
	q := url.Values{"state": {state}, "code": {"one-use-code"}, "hostname": {"eapi.pcloud.com"}, "locationid": {"2"}}
	req := httptest.NewRequest("GET", h.cfg.Origin+"/oauth/pcloud/callback?"+q.Encode(), nil)
	if c != nil {
		req.AddCookie(c)
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, req)
	return r
}
func TestSetupRequiresSecretCSRFAndOrigin(t *testing.T) {
	for _, kind := range []string{"secret", "csrf", "origin", "cookie"} {
		t.Run(kind, func(t *testing.T) {
			h, c, csrf, calls := fixture(t)
			secret := h.cfg.SetupSecret
			origin := h.cfg.Origin
			switch kind {
			case "secret":
				secret = "wrong"
			case "csrf":
				csrf = "wrong"
			case "origin":
				origin = "https://foreign.example"
			case "cookie":
				c = nil
			}
			r := start(h, c, csrf, secret, origin)
			if r.Code == 303 || r.Code == 200 || *calls != 0 {
				t.Fatal("unauthorized setup accepted", r.Code)
			}
		})
	}
}
func TestCallbackBrowserStateExpiryAndReplay(t *testing.T) {
	for _, kind := range []string{"state", "cookie", "expired", "valid"} {
		t.Run(kind, func(t *testing.T) {
			h, c, csrf, calls := fixture(t)
			r := start(h, c, csrf, h.cfg.SetupSecret, h.cfg.Origin)
			if r.Code != 303 {
				t.Fatal("start failed", r.Code, r.Body.String())
			}
			redirect, _ := url.Parse(r.Header().Get("Location"))
			state := redirect.Query().Get("state")
			if redirect.Host != "my.pcloud.com" || redirect.Query().Get("redirect_uri") != h.cfg.Origin+"/oauth/pcloud/callback" {
				t.Fatal("wrong redirect")
			}
			cbCookie := c
			switch kind {
			case "state":
				state = "wrong"
			case "cookie":
				cbCookie = nil
			case "expired":
				h.now = func() time.Time { return time.Now().Add(6 * time.Minute) }
			}
			out := callback(h, cbCookie, state)
			if kind != "valid" {
				if out.Code == 200 || *calls != 0 {
					t.Fatal("unbound callback accepted")
				}
				return
			}
			if out.Code != 200 || *calls != 1 || strings.Contains(out.Body.String(), "never-show-this-token") {
				t.Fatal("unsafe callback", out.Code)
			}
			envelope := regexp.MustCompile(`<textarea[^>]*>([^<]+)</textarea>`).FindStringSubmatch(out.Body.String())[1]
			data, err := credential.Open(h.cfg.EncryptionKey, envelope)
			if err != nil || data.UserID != 7 {
				t.Fatal("credential not usable", err)
			}
			if callback(h, c, state).Code == 200 || *calls != 1 {
				t.Fatal("callback replay accepted")
			}
		})
	}
}
func TestSetupConfigurationAndOwnerBinding(t *testing.T) {
	h, _, _, _ := fixture(t)
	for _, change := range []func(*Config){func(c *Config) { c.Origin = "http://service.example" }, func(c *Config) { c.EncryptionKey = "weak" }, func(c *Config) { c.SetupSecret = c.EncryptionKey }, func(c *Config) { c.ClientSecret = "" }} {
		cfg := h.cfg
		change(&cfg)
		if _, err := New(cfg); err == nil {
			t.Fatal("unsafe config accepted")
		}
	}
	h, c, csrf, calls := fixture(t)
	h.cfg.ExpectedUserID = 8
	r := start(h, c, csrf, h.cfg.SetupSecret, h.cfg.Origin)
	u, _ := url.Parse(r.Header().Get("Location"))
	out := callback(h, c, u.Query().Get("state"))
	if out.Code == 200 || *calls != 1 {
		t.Fatal("wrong provider owner accepted")
	}
}
