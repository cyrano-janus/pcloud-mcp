// pcloud-auth performs the provider's confidential-client code flow locally.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/cyrano-janus/pcloud-mcp/internal/config"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() { os.Exit(run()) }
func run() int {
	output := flag.String("out", "secrets/pcloud-token", "New token file; existing files are never replaced")
	flag.Parse()
	clientID := os.Getenv("PCLOUD_CLIENT_ID")
	secret, err := config.SecretFromEnv("PCLOUD_CLIENT_SECRET")
	if err != nil || clientID == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "configure PCLOUD_CLIENT_ID and PCLOUD_CLIENT_SECRET or PCLOUD_CLIENT_SECRET_FILE")
		return 2
	}
	if _, err := os.Lstat(*output); !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "token destination must not already exist")
		return 2
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return 1
	}
	state := hex.EncodeToString(nonce[:])
	listener, err := net.Listen("tcp", "127.0.0.1:8976")
	if err != nil {
		fmt.Fprintln(os.Stderr, "local callback port 8976 unavailable")
		return 1
	}
	defer listener.Close()
	type approval struct{ region, code string }
	approved := make(chan approval, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != "GET" || r.Host != "127.0.0.1:8976" || len(r.URL.RawQuery) > 8192 {
			http.Error(w, "Callback rejected", 400)
			return
		}
		region, code, err := pcloud.ValidateCallback(r.URL.Query(), state)
		if err != nil {
			http.Error(w, "Callback rejected", 400)
			return
		}
		select {
		case approved <- approval{region, code}:
			fmt.Fprintln(w, "Authorization received. You may close this window.")
		default:
			http.Error(w, "Callback already received", 409)
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() { _ = srv.Serve(listener) }()
	defer srv.Close()
	fmt.Fprintln(os.Stderr, "Register this exact redirect URI in your pCloud app: http://127.0.0.1:8976/callback")
	fmt.Fprintln(os.Stderr, "Open in your local browser:", pcloud.AuthorizationURL(clientID, "http://127.0.0.1:8976/callback", state))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var a approval
	select {
	case a = <-approved:
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "authorization timed out")
		return 1
	}
	credential, err := pcloud.ExchangeCode(ctx, a.region, clientID, secret, a.code)
	if err != nil {
		fmt.Fprintln(os.Stderr, "token exchange failed; no token saved")
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0700); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create token directory")
		return 1
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create new token file")
		return 1
	}
	_, writeErr := file.WriteString(credential.Token + "\n")
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(*output)
		fmt.Fprintln(os.Stderr, "token file could not be saved")
		return 1
	}
	fmt.Printf("Token saved with mode 0600. Configure:\nPCLOUD_REGION=%s\nPCLOUD_USER_ID=%d\nPCLOUD_ACCESS_TOKEN_FILE=%s\nThen choose PCLOUD_ROOT_FOLDER_ID.\n", a.region, credential.UserID, *output)
	return 0
}
