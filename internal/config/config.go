// Package config validates environment-based runtime configuration.
package config

import (
	"errors"
	"strconv"
)

const DefaultMaxFrameBytes = 1 << 20
const MinFrameBytes = 1024
const MaxFrameBytesLimit = 8 << 20

type Config struct {
	MaxFrameBytes                                                            int
	AccessToken, Region                                                      string
	UserID, RootFolderID                                                     int64
	RenderProxy                                                              bool
	EnableWrites                                                             bool
	Transport, Listen, PublicURL                                             string
	TLSCert, TLSKey                                                          string
	Issuer, IntrospectionURL, OAuthClientID, OAuthClientSecret, OAuthSubject string
}

// Load never includes environment values in errors. A present but empty value
// is invalid; an absent value selects the safe default.
func Load(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MaxFrameBytes: DefaultMaxFrameBytes}
	if value, set := lookup("PCLOUD_MCP_MAX_FRAME_BYTES"); set {
		n, err := strconv.Atoi(value)
		if err != nil || n < MinFrameBytes || n > MaxFrameBytesLimit {
			return Config{}, errors.New("PCLOUD_MCP_MAX_FRAME_BYTES must be an integer between 1024 and 8388608")
		}
		cfg.MaxFrameBytes = n
	}
	return loadRuntime(cfg, lookup)
}
