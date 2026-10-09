// Package policy enforces identity and operation permissions independently of tools.
package policy

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrDenied = errors.New("operation denied by server policy")

type Principal struct {
	UserID int64
	Write  bool
}
type key struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, key{}, p)
}
func FromContext(ctx context.Context) Principal { p, _ := ctx.Value(key{}).(Principal); return p }

type Policy struct {
	UserID       int64
	EnableWrites bool
}

func (p Policy) Authorize(actor Principal, op string, owner bool) error {
	if p.UserID <= 0 || actor.UserID != p.UserID || !owner {
		return ErrDenied
	}
	switch op {
	case "whoami", "list_folder", "get_file_info", "search_files", "read_text_file":
		return nil
	case "create_folder", "copy_file", "upload_file", "save_artifact":
		if p.EnableWrites && actor.Write {
			return nil
		}
	}
	return ErrDenied
}
func ValidName(name string) bool {
	if name == "" || len(name) > 255 || name == "." || name == ".." || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
