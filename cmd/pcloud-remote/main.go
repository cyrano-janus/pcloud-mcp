// Command pcloud-remote provides a safe remote deployment readiness endpoint.
// MCP stays closed; optional hosted provider onboarding is isolated from file tools.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/onboarding"
	"github.com/cyrano-janus/pcloud-mcp/internal/remote"
	"github.com/cyrano-janus/pcloud-mcp/internal/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func handler() http.Handler { return handlerWithSetup(nil) }
func handlerWithSetup(setup http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "MCP HTTP transport not enabled: OAuth and policy enforcement pending", http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/setup/pcloud" && r.URL.Path != "/setup/pcloud/start" && r.URL.Path != "/oauth/pcloud/callback" {
			http.NotFound(w, r)
			return
		}
		if setup == nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "Gehostete pCloud-Einrichtung noch nicht konfiguriert", 503)
			return
		}
		setup.ServeHTTP(w, r)
	})
	return mux
}

func run(ctx context.Context, port string) error {
	if port == "" {
		port = "10000"
	}
	// PORT is provided by the hosting platform. Disallow address injection.
	for _, c := range port {
		if c < '0' || c > '9' {
			return errors.New("invalid PORT")
		}
	}
	if len(port) > 5 {
		return errors.New("invalid PORT")
	}
	activeHandler, err := runtimeHandler(ctx, os.LookupEnv, server.HTTPHandler)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           activeHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8192,
	}
	errs := make(chan error, 1)
	go func() { errs <- srv.ListenAndServe() }()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv("PORT")); err != nil {
		log.Print(startupDiagnostic(err))
		fmt.Fprintln(os.Stderr, "startup or shutdown failed")
		os.Exit(1)
	}
}

func startupDiagnostic(err error) string {
	var cfgErr *onboarding.ConfigError
	if errors.As(err, &cfgErr) {
		switch cfgErr.Field {
		case "PCLOUD_SETUP_ENABLED", "RENDER", "RENDER_EXTERNAL_URL", "PCLOUD_CLIENT_ID", "PCLOUD_CLIENT_SECRET", "PCLOUD_SETUP_SECRET", "PCLOUD_TOKEN_ENCRYPTION_KEY", "PCLOUD_USER_ID":
			return "startup configuration invalid: " + cfgErr.Field
		}
	}
	return "HTTP readiness service stopped"
}

func setupFromEnv() (http.Handler, error) {
	enabled := os.Getenv("PCLOUD_SETUP_ENABLED")
	if enabled == "" || enabled == "false" {
		return nil, nil
	}
	if enabled != "true" {
		return nil, &onboarding.ConfigError{Field: "PCLOUD_SETUP_ENABLED"}
	}
	if os.Getenv("RENDER") != "true" {
		return nil, &onboarding.ConfigError{Field: "RENDER"}
	}
	secret, err := config.SecretFromEnv("PCLOUD_CLIENT_SECRET")
	if err != nil {
		return nil, &onboarding.ConfigError{Field: "PCLOUD_CLIENT_SECRET"}
	}
	setupSecret, err := config.SecretFromEnv("PCLOUD_SETUP_SECRET")
	if err != nil {
		return nil, &onboarding.ConfigError{Field: "PCLOUD_SETUP_SECRET"}
	}
	encryptionKey, err := config.SecretFromEnv("PCLOUD_TOKEN_ENCRYPTION_KEY")
	if err != nil {
		return nil, &onboarding.ConfigError{Field: "PCLOUD_TOKEN_ENCRYPTION_KEY"}
	}
	var owner int64
	if value := os.Getenv("PCLOUD_USER_ID"); value != "" {
		owner, err = strconv.ParseInt(value, 10, 64)
		if err != nil || owner <= 0 {
			return nil, &onboarding.ConfigError{Field: "PCLOUD_USER_ID"}
		}
	}
	return onboarding.New(onboarding.Config{Origin: os.Getenv("RENDER_EXTERNAL_URL"), ClientID: os.Getenv("PCLOUD_CLIENT_ID"), ClientSecret: secret, SetupSecret: setupSecret, EncryptionKey: encryptionKey, ExpectedUserID: owner})
}

// runtimeHandler makes activation explicit; incomplete configuration never opens MCP.
func runtimeHandler(ctx context.Context, lookup func(string) (string, bool), build func(context.Context, config.Config) (http.Handler, error)) (http.Handler, error) {
	get := func(k string) string { v, _ := lookup(k); return v }
	enabled := get("PCLOUD_MCP_ENABLED")
	if enabled == "" || enabled == "false" {
		setup, err := setupFromEnv()
		if err != nil {
			return nil, err
		}
		return handlerWithSetup(setup), nil
	}
	if enabled != "true" || get("RENDER") != "true" {
		return nil, errors.New("remote activation requires an explicit flag and Render runtime")
	}
	if setup := get("PCLOUD_SETUP_ENABLED"); setup != "" && setup != "false" {
		return nil, errors.New("disable provider setup before activating MCP")
	}
	if writes := get("PCLOUD_ENABLE_WRITES"); writes != "" && writes != "false" {
		return nil, errors.New("native remote activation is read-only")
	}
	// Derive the deployed transport, rather than trusting an accidental stdio default.
	derived := func(k string) (string, bool) {
		switch k {
		case "PCLOUD_MCP_TRANSPORT":
			return "http", true
		case "PCLOUD_MCP_TLS_PROXY":
			return "render", true
		case "PCLOUD_ENABLE_WRITES":
			return "false", true
		}
		return lookup(k)
	}
	cfg, err := config.Load(derived)
	if err != nil {
		return nil, err
	}
	if err := remote.Validate(cfg); err != nil {
		return nil, err
	}
	return build(ctx, cfg)
}
