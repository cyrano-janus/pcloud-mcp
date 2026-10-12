// Package onboarding implements a temporary, owner-controlled provider setup flow.
// It never enables MCP operations or sends plaintext provider tokens to a browser.
package onboarding

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cyrano-janus/pcloud-mcp/internal/credential"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"golang.org/x/time/rate"
)

const cookieName = "__Host-pcloud-setup"
const lifetime = 5 * time.Minute
const maxSessions = 32

type Config struct {
	Origin, ClientID, ClientSecret, SetupSecret, EncryptionKey string
	ExpectedUserID                                             int64
}

// ConfigError identifies a configuration field without including its value.
type ConfigError struct{ Field string }

func (e *ConfigError) Error() string { return "invalid hosted setup configuration: " + e.Field }

type exchangeFunc func(context.Context, string, string, string, string) (credential.Data, error)
type session struct {
	state   string
	expires time.Time
	started bool
}
type Handler struct {
	cfg      Config
	host     string
	mu       sync.Mutex
	sessions map[string]*session
	now      func() time.Time
	exchange exchangeFunc
	proofKey [32]byte
	limit    *rate.Limiter
	mux      *http.ServeMux
}

func New(cfg Config) (*Handler, error) { return newHandler(cfg, exchange) }

type exchangeError struct {
	stage string
	cause error
}

func (e *exchangeError) Error() string { return "provider onboarding failed" }
func (e *exchangeError) Unwrap() error { return e.cause }

var errIdentityMismatch = errors.New("identity mismatch")
var errRegionMismatch = errors.New("region mismatch")
var errOwnerMismatch = errors.New("owner mismatch")

// Only fixed stages/categories and numeric provider statuses enter logs.
func setupFailureDiagnostic(err error) string {
	stage := "unknown"
	var wrapped *exchangeError
	if errors.As(err, &wrapped) {
		switch wrapped.stage {
		case "token_exchange", "userinfo", "binding":
			stage = wrapped.stage
		}
	}
	category := "unknown"
	var status *pcloud.HTTPError
	var api pcloud.APIError
	switch {
	case errors.As(err, &status):
		category = "http_status=" + strconv.Itoa(status.StatusCode)
	case errors.As(err, &api):
		category = "provider_error_number=" + strconv.Itoa(api.Code)
	case errors.Is(err, pcloud.ErrTransport):
		category = "transport"
	case errors.Is(err, pcloud.ErrResponse):
		category = "invalid_response"
	case errors.Is(err, errIdentityMismatch):
		category = "identity_mismatch"
	case errors.Is(err, errRegionMismatch):
		category = "region_mismatch"
	case errors.Is(err, errOwnerMismatch):
		category = "owner_mismatch"
	}
	return stage + " " + category
}
func newHandler(cfg Config, fn exchangeFunc) (*Handler, error) {
	u, err := url.Parse(cfg.Origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Opaque != "" {
		return nil, &ConfigError{Field: "RENDER_EXTERNAL_URL"}
	}
	for _, check := range []struct {
		field   string
		invalid bool
	}{
		{"PCLOUD_USER_ID", cfg.ExpectedUserID < 0},
		{"PCLOUD_CLIENT_ID", cfg.ClientID == "" || len(cfg.ClientID) > 256},
		{"PCLOUD_CLIENT_SECRET", cfg.ClientSecret == ""},
		{"PCLOUD_SETUP_SECRET", !credential.ValidKey(cfg.SetupSecret)},
		{"PCLOUD_TOKEN_ENCRYPTION_KEY", !credential.ValidKey(cfg.EncryptionKey) || cfg.SetupSecret == cfg.EncryptionKey},
	} {
		if check.invalid {
			return nil, &ConfigError{Field: check.field}
		}
	}
	h := &Handler{cfg: cfg, host: u.Host, sessions: map[string]*session{}, now: time.Now, exchange: fn, limit: rate.NewLimiter(1, 5), mux: http.NewServeMux()}
	if _, err := rand.Read(h.proofKey[:]); err != nil {
		return nil, errors.New("setup form protection unavailable")
	}
	h.mux.HandleFunc("GET /setup/pcloud", h.form)
	h.mux.HandleFunc("POST /setup/pcloud/start", h.start)
	h.mux.HandleFunc("GET /oauth/pcloud/callback", h.callback)
	return h, nil
}
func exchange(ctx context.Context, region, clientID, secret, code string) (credential.Data, error) {
	result, err := pcloud.ExchangeCode(ctx, region, clientID, secret, code)
	if err != nil {
		return credential.Data{}, &exchangeError{stage: "token_exchange", cause: err}
	}
	client, err := pcloud.New(region, result.Token)
	if err != nil {
		return credential.Data{}, &exchangeError{stage: "userinfo", cause: err}
	}
	identity, err := client.Whoami(ctx)
	if err != nil {
		return credential.Data{}, &exchangeError{stage: "userinfo", cause: err}
	}
	if identity.UserID != result.UserID {
		return credential.Data{}, &exchangeError{stage: "userinfo", cause: errIdentityMismatch}
	}
	return credential.Data{Token: result.Token, Region: region, UserID: identity.UserID}, nil
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// HTML form navigation uses the document's referrer policy to determine
	// Origin. no-referrer would produce Origin:null and fail our exact check.
	// Only the token-free setup form permits same-origin referrers; provider
	// redirects and callbacks retain no-referrer to protect authorization codes.
	if (r.Method == "GET" || r.Method == "HEAD") && r.URL.Path == "/setup/pcloud" && r.URL.RawQuery == "" {
		w.Header().Set("Referrer-Policy", "same-origin")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self' https://my.pcloud.com; frame-ancestors 'none'; base-uri 'none'")
	origin := r.Header.Get("Origin")
	if r.Host != h.host || (origin != "" && origin != h.cfg.Origin) || (r.Method == "POST" && origin == "") {
		http.Error(w, "Zugriff abgewiesen", 403)
		return
	}
	if len(r.URL.RawQuery) > 8192 || (r.URL.Path != "/oauth/pcloud/callback" && r.URL.RawQuery != "") {
		http.Error(w, "Ungültige Anfrage", 400)
		return
	}
	if !h.limit.Allow() {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "Bitte später erneut versuchen", 429)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	h.mux.ServeHTTP(w, r.WithContext(ctx))
}
func nonce() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
func equal(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}
func browserID(r *http.Request) string {
	value := ""
	count := 0
	for _, c := range r.Cookies() {
		if c.Name == cookieName {
			count++
			value = c.Value
		}
	}
	if count != 1 || len(value) != 43 {
		return ""
	}
	return value
}
func (h *Handler) clean() {
	for id, s := range h.sessions {
		if !h.now().Before(s.expires) {
			delete(h.sessions, id)
		}
	}
}

// Public forms carry a browser-bound, expiring proof without reserving memory.
// An independent process key avoids exposing an offline setup-secret oracle.
func (h *Handler) formProof(id, expiry string) string {
	mac := hmac.New(sha256.New, h.proofKey[:])
	_, _ = mac.Write([]byte("pcloud-mcp/setup-form/v1\x00" + id + "\x00" + expiry))
	return expiry + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (h *Handler) validateProof(id, proof string) (time.Time, bool) {
	parts := strings.Split(proof, ".")
	if id == "" || len(parts) != 2 || len(proof) > 80 {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	expires := time.Unix(seconds, 0)
	if err != nil || !h.now().Before(expires) || expires.After(h.now().Add(lifetime)) {
		return time.Time{}, false
	}
	return expires, equal(proof, h.formProof(id, parts[0]))
}

var formTemplate = template.Must(template.New("setup").Parse(`<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>pCloud verbinden</title><main><h1>pCloud verbinden</h1><p>Diese Einrichtung verbindet ein pCloud-Konto. Dateioperationen bleiben gesperrt. Der Einrichtungsschlüssel gehört nur dem Betreiber dieses Dienstes.</p><form method="post" action="/setup/pcloud/start"><input type="hidden" name="csrf" value="{{.}}"><label for="setup_secret">Einrichtungsschlüssel aus Render</label><input id="setup_secret" name="setup_secret" type="password" autocomplete="off" required maxlength="128"><button type="submit">Bei pCloud anmelden und Zugriff freigeben</button></form><p>Die Sitzung läuft nach fünf Minuten ab. Bei einem Neustart muss die Einrichtung neu begonnen werden.</p></main></html>`))

func (h *Handler) form(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clean()
	id := browserID(r)
	if s := h.sessions[id]; s != nil && s.started {
		http.Error(w, "Diese Freigabe wurde bereits begonnen", 409)
		return
	}
	if id == "" {
		var err error
		id, err = nonce()
		if err != nil {
			http.Error(w, "Einrichtung nicht verfügbar", 503)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 300})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = formTemplate.Execute(w, h.formProof(id, strconv.FormatInt(h.now().Add(lifetime).Unix(), 10)))
}
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if r.ParseForm() != nil || len(r.PostForm["csrf"]) != 1 || len(r.PostForm["setup_secret"]) != 1 {
		http.Error(w, "Ungültige Anfrage", 400)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clean()
	id := browserID(r)
	expires, validProof := h.validateProof(id, r.PostForm.Get("csrf"))
	if h.sessions[id] != nil || id == "" {
		http.Error(w, "Einrichtung abgewiesen", 403)
		return
	}
	if !equal(r.PostForm.Get("setup_secret"), h.cfg.SetupSecret) || !validProof {
		http.Error(w, "Einrichtung abgewiesen", 403)
		return
	}
	if len(h.sessions) >= maxSessions {
		http.Error(w, "Einrichtung vorübergehend ausgelastet", 503)
		return
	}
	state, err := nonce()
	if err != nil {
		http.Error(w, "Einrichtung nicht verfügbar", 503)
		return
	}
	h.sessions[id] = &session{state: state, expires: expires, started: true}
	http.Redirect(w, r, pcloud.AuthorizationURL(h.cfg.ClientID, h.cfg.Origin+"/oauth/pcloud/callback", state), http.StatusSeeOther)
}

var resultTemplate = template.Must(template.New("result").Parse(`<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>pCloud-Freigabe erhalten</title><main><h1>pCloud-Freigabe erhalten</h1><p>Das Zugangstoken wird nicht angezeigt. Das folgende Paket ist mit Deinem separaten Schlüssel verschlüsselt.</p><label for="envelope">Verschlüsseltes Paket für PCLOUD_CREDENTIAL_ENVELOPE in Render</label><textarea id="envelope" readonly rows="8" cols="64">{{.Envelope}}</textarea><p>PCLOUD_REGION: {{.Region}}</p><p>PCLOUD_USER_ID: {{.UserID}}</p><p>Wähle zusätzlich PCLOUD_ROOT_FOLDER_ID für Deinen erlaubten Ordner. Speichere das Paket in Render, behalte PCLOUD_TOKEN_ENCRYPTION_KEY und setze anschließend PCLOUD_SETUP_ENABLED=false. Bewahre den Verschlüsselungsschlüssel geschützt auf. Nach Verlust ist eine neue Freigabe nötig.</p><p>Die MCP-Anmeldung ist ein eigener Schritt. Dateioperationen bleiben bis zu ihrer vollständigen Einrichtung gesperrt.</p></main></html>`))

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.clean()
	id := browserID(r)
	s := h.sessions[id]
	if s == nil || !s.started {
		h.mu.Unlock()
		http.Error(w, "Freigabe abgewiesen oder abgelaufen", 400)
		return
	}
	region, code, err := pcloud.ValidateCallback(r.URL.Query(), s.state)
	if err != nil {
		h.mu.Unlock()
		http.Error(w, "Freigabe abgewiesen", 400)
		return
	}
	// Atomically consume before contacting the provider: concurrent replay cannot exchange again.
	delete(h.sessions, id)
	h.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	data, err := h.exchange(r.Context(), region, h.cfg.ClientID, h.cfg.ClientSecret, code)
	if err == nil && data.Region != region {
		err = &exchangeError{stage: "binding", cause: errRegionMismatch}
	}
	if err == nil && h.cfg.ExpectedUserID > 0 && data.UserID != h.cfg.ExpectedUserID {
		err = &exchangeError{stage: "binding", cause: errOwnerMismatch}
	}
	if err != nil {
		log.Print("pCloud setup failed: " + setupFailureDiagnostic(err))
		http.Error(w, "Kontofreigabe konnte nicht verifiziert werden; Einrichtung erneut beginnen", 502)
		return
	}
	envelope, err := credential.Seal(h.cfg.EncryptionKey, data)
	if err != nil {
		http.Error(w, "Zugangspaket konnte nicht erstellt werden", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = resultTemplate.Execute(w, struct{ Envelope, Region, UserID string }{envelope, data.Region, strconv.FormatInt(data.UserID, 10)})
}
