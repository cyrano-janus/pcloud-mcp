package onboarding

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cyrano-janus/pcloud-mcp/internal/credential"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
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

func TestProviderFailureDiagnosticsAreRedacted(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&exchangeError{stage: "token_exchange", cause: pcloud.APIError{Code: 2000}}, "token_exchange provider_error_number=2000"},
		{&exchangeError{stage: "userinfo", cause: pcloud.ErrTransport}, "userinfo transport"},
		{&exchangeError{stage: "token_exchange", cause: &pcloud.HTTPError{StatusCode: 403}}, "token_exchange http_status=403"},
		{&exchangeError{stage: "userinfo", cause: pcloud.ErrResponse}, "userinfo invalid_response"},
		{&exchangeError{stage: "private-secret", cause: errors.New("private-secret")}, "unknown unknown"},
		{errors.New("private-secret"), "unknown unknown"},
	} {
		if got := setupFailureDiagnostic(tc.err); got != tc.want {
			t.Fatal("unsafe or imprecise failure diagnostic", got)
		}
	}
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
	for _, kind := range []string{"secret", "csrf", "origin", "missing origin", "null origin", "cookie"} {
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
			case "missing origin":
				origin = ""
			case "null origin":
				origin = "null"
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

func TestSetupFormPreservesSameOriginPost(t *testing.T) {
	h, c, csrf, _ := fixture(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", h.cfg.Origin+"/setup/pcloud", nil))
	if w.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("form policy suppresses Origin on browser POST")
	}
	r := start(h, c, csrf, h.cfg.SetupSecret, h.cfg.Origin)
	if r.Code != 303 || r.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("provider redirect can leak referrer", r.Code)
	}
	u, _ := url.Parse(r.Header().Get("Location"))
	out := callback(h, c, u.Query().Get("state"))
	if out.Code != 200 || out.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("callback can leak provider code")
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

func TestPublicFormsDoNotReserveSessions(t *testing.T) {
	h, c, csrf, _ := fixture(t)
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		h.form(w, httptest.NewRequest("GET", h.cfg.Origin+"/setup/pcloud", nil))
		if w.Code != 200 {
			t.Fatal("public form blocked", w.Code)
		}
	}
	if len(h.sessions) != 0 {
		t.Fatal("unauthenticated forms reserved OAuth sessions")
	}
	if r := start(h, c, csrf, "wrong", h.cfg.Origin); r.Code != 403 || len(h.sessions) != 0 {
		t.Fatal("invalid secret allocated session")
	}
	if r := start(h, c, csrf, h.cfg.SetupSecret, h.cfg.Origin); r.Code != 303 || len(h.sessions) != 1 {
		t.Fatal("owner could not start", r.Code)
	}
}

func TestFormProofBrowserExpiryAndTampering(t *testing.T) {
	for _, kind := range []string{"browser", "expiry", "tampering", "restart"} {
		t.Run(kind, func(t *testing.T) {
			h, c, csrf, _ := fixture(t)
			switch kind {
			case "browser":
				c.Value = strings.Repeat("x", 43)
			case "expiry":
				h.now = func() time.Time { return time.Now().Add(6 * time.Minute) }
			case "tampering":
				csrf = "0" + csrf
			case "restart":
				next, err := New(h.cfg)
				if err != nil {
					t.Fatal(err)
				}
				h = next
			}
			if r := start(h, c, csrf, h.cfg.SetupSecret, h.cfg.Origin); r.Code != 403 || len(h.sessions) != 0 {
				t.Fatal("unbound form accepted", r.Code)
			}
		})
	}
}
