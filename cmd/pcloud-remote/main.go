// Command pcloud-remote provides a safe remote deployment readiness endpoint.
// M0b deliberately exposes no MCP tools or credentials: remote OAuth is not implemented.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func handler() http.Handler {
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
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
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
		log.Print("HTTP readiness service stopped")
		fmt.Fprintln(os.Stderr, "startup or shutdown failed")
		os.Exit(1)
	}
}
