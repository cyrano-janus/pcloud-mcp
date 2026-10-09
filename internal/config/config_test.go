package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		set, valid  bool
		want        int
	}{
		{"default", "", false, true, 1048576},
		{"minimum", "1024", true, true, 1024},
		{"maximum", "8388608", true, true, 8388608},
		{"below", "1023", true, false, 0},
		{"above", "8388609", true, false, 0},
		{"empty", "", true, false, 0},
		{"negative", "-1", true, false, 0},
		{"overflow", "99999999999999999999999999", true, false, 0},
		{"non-number", "sentinel-secret", true, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(func(k string) (string, bool) {
				if k != "PCLOUD_MCP_MAX_FRAME_BYTES" {
					return "", false
				}
				return tc.value, tc.set
			})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), tc.value) && tc.value != "" {
				t.Fatal("error exposed configured value")
			}
			if err == nil && cfg.MaxFrameBytes != tc.want {
				t.Fatalf("got %d, want %d", cfg.MaxFrameBytes, tc.want)
			}
		})
	}
}

func FuzzConfig(f *testing.F) {
	for _, seed := range []string{"", "1023", "1024", "1048576", "1048577", "-1", "secret", "999999999999999999999"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		cfg, err := Load(func(k string) (string, bool) {
			if k != "PCLOUD_MCP_MAX_FRAME_BYTES" {
				return "", false
			}
			return value, true
		})
		if err == nil && (cfg.MaxFrameBytes < 1024 || cfg.MaxFrameBytes > 8388608) {
			t.Fatal("accepted out-of-range frame limit")
		}
	})
}
