package config

import (
	"errors"
	"github.com/cyrano-janus/pcloud-mcp/internal/credential"
	"io"
	"os"
	"strconv"
	"strings"
)

// SecretFromEnv loads one environment or mounted-file secret without logging it.
func SecretFromEnv(key string) (string, error) { return secret(key, os.LookupEnv) }

func loadRuntime(cfg Config, lookup func(string) (string, bool)) (Config, error) {
	get := func(k string) string { v, _ := lookup(k); return v }
	var err error
	cfg.AccessToken, err = secret("PCLOUD_ACCESS_TOKEN", lookup)
	if err != nil {
		return cfg, err
	}
	var encryptedOwner int64
	cfg.Region = get("PCLOUD_REGION")
	if envelope, set := lookup("PCLOUD_CREDENTIAL_ENVELOPE"); set {
		if cfg.AccessToken != "" {
			return cfg, errors.New("configure an encrypted credential or a plaintext token, not both")
		}
		key, err := secret("PCLOUD_TOKEN_ENCRYPTION_KEY", lookup)
		if err != nil {
			return cfg, err
		}
		data, err := credential.Open(key, envelope)
		if err != nil {
			return cfg, errors.New("encrypted pCloud credential unavailable or invalid")
		}
		if cfg.Region != "" && cfg.Region != data.Region {
			return cfg, errors.New("encrypted credential region mismatch")
		}
		cfg.AccessToken = data.Token
		cfg.Region = data.Region
		encryptedOwner = data.UserID
	}
	cfg.Transport = get("PCLOUD_MCP_TRANSPORT")
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.Transport != "stdio" && cfg.Transport != "http" {
		return cfg, errors.New("PCLOUD_MCP_TRANSPORT must be stdio or http")
	}
	if v, set := lookup("PCLOUD_ENABLE_WRITES"); set {
		if v != "true" && v != "false" {
			return cfg, errors.New("PCLOUD_ENABLE_WRITES must be true or false")
		}
		cfg.EnableWrites = v == "true"
	}
	if cfg.AccessToken != "" {
		if cfg.Region != "eu" && cfg.Region != "us" {
			return cfg, errors.New("PCLOUD_REGION must explicitly be eu or us")
		}
		cfg.UserID, err = strconv.ParseInt(get("PCLOUD_USER_ID"), 10, 64)
		if err != nil || cfg.UserID <= 0 {
			return cfg, errors.New("positive PCLOUD_USER_ID required")
		}
		if encryptedOwner != 0 && cfg.UserID != encryptedOwner {
			return cfg, errors.New("encrypted credential owner mismatch")
		}
		cfg.RootFolderID, err = strconv.ParseInt(get("PCLOUD_ROOT_FOLDER_ID"), 10, 64)
		if err != nil || cfg.RootFolderID < 0 {
			return cfg, errors.New("nonnegative PCLOUD_ROOT_FOLDER_ID required; 0 explicitly grants the account root")
		}
	} else if cfg.EnableWrites || cfg.Transport == "http" {
		return cfg, errors.New("pCloud OAuth token required for writes or HTTP")
	}
	cfg.Listen = get("PCLOUD_MCP_LISTEN")
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8080"
	}
	cfg.PublicURL = get("PCLOUD_MCP_PUBLIC_URL")
	if proxy := get("PCLOUD_MCP_TLS_PROXY"); proxy != "" {
		if proxy != "render" || get("RENDER") != "true" {
			return cfg, errors.New("TLS proxy mode requires the Render runtime")
		}
		port, err := strconv.Atoi(get("PORT"))
		if err != nil || port < 1024 || port > 65535 {
			return cfg, errors.New("valid Render PORT required")
		}
		cfg.RenderProxy = true
		cfg.Listen = "0.0.0.0:" + strconv.Itoa(port)
		if cfg.PublicURL == "" {
			cfg.PublicURL = strings.TrimSuffix(get("RENDER_EXTERNAL_URL"), "/") + "/mcp"
		}
	}
	cfg.TLSCert = get("PCLOUD_MCP_TLS_CERT")
	cfg.TLSKey = get("PCLOUD_MCP_TLS_KEY")
	cfg.Issuer = get("PCLOUD_MCP_OAUTH_ISSUER")
	cfg.IntrospectionURL = get("PCLOUD_MCP_INTROSPECTION_URL")
	cfg.OAuthClientID = get("PCLOUD_MCP_OAUTH_CLIENT_ID")
	cfg.OAuthSubject = get("PCLOUD_MCP_OAUTH_SUBJECT")
	cfg.OAuthClientSecret, err = secret("PCLOUD_MCP_OAUTH_CLIENT_SECRET", lookup)
	if err != nil {
		return cfg, err
	}
	if cfg.Transport == "http" && (cfg.PublicURL == "" || cfg.Issuer == "" || cfg.IntrospectionURL == "" || cfg.OAuthClientID == "" || cfg.OAuthClientSecret == "" || cfg.OAuthSubject == "") {
		return cfg, errors.New("HTTP requires public URL, OAuth issuer, introspection endpoint, client credentials and allowed subject")
	}
	return cfg, nil
}
func secret(key string, lookup func(string) (string, bool)) (string, error) {
	value, hasValue := lookup(key)
	file, hasFile := lookup(key + "_FILE")
	if hasValue && hasFile {
		return "", errors.New("configure a secret value or secret file, not both")
	}
	if hasFile {
		f, err := os.Open(file)
		if err != nil {
			return "", errors.New("secret file unavailable")
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() || st.Size() > 4096 {
			return "", errors.New("invalid secret file")
		}
		// Secret volumes may be root-owned, read-only group-readable mounts. They
		// must never be writable by group/others or readable by arbitrary users.
		if st.Mode().Perm()&0027 != 0 {
			return "", errors.New("secret file must not be world-readable or group/world-writable")
		}
		data, err := io.ReadAll(io.LimitReader(f, 4097))
		if err != nil || len(data) > 4096 {
			return "", errors.New("invalid secret file")
		}
		value = strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	}
	if (hasValue || hasFile) && (value == "" || len(value) > 4096 || strings.ContainsAny(value, "\r\n")) {
		return "", errors.New("invalid secret value")
	}
	return value, nil
}
