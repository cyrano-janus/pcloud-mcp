// Package config validates environment-based runtime configuration.
package config

import (
	"errors"
	"strconv"
)

const DefaultMaxFrameBytes = 1 << 20
const MinFrameBytes = 1024

type Config struct{ MaxFrameBytes int }

// Load never includes environment values in errors. A present but empty value
// is invalid; an absent value selects the safe default.
func Load(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MaxFrameBytes: DefaultMaxFrameBytes}
	if value, set := lookup("PCLOUD_MCP_MAX_FRAME_BYTES"); set {
		n, err := strconv.Atoi(value)
		if err != nil || n < MinFrameBytes || n > DefaultMaxFrameBytes {
			return Config{}, errors.New("PCLOUD_MCP_MAX_FRAME_BYTES must be an integer between 1024 and 1048576")
		}
		cfg.MaxFrameBytes = n
	}
	return cfg, nil
}
