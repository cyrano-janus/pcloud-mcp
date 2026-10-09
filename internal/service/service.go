// Package service combines provider-independent policy with bounded file use cases.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/cyrano-janus/pcloud-mcp/internal/model"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"github.com/cyrano-janus/pcloud-mcp/internal/policy"
)

type API interface {
	Whoami(context.Context) (model.Identity, error)
	ListFolder(context.Context, int64) (model.Metadata, error)
	Stat(context.Context, int64) (model.Metadata, error)
	Text(context.Context, int64) (string, error)
	CreateFolder(context.Context, int64, string) (model.Metadata, error)
	Copy(context.Context, int64, int64, string) (model.Metadata, error)
	Upload(context.Context, int64, string, []byte) (model.Metadata, error)
}
type Service struct {
	api     API
	policy  policy.Policy
	root    int64
	audit   io.Writer
	auditMu sync.Mutex
}

func New(api API, p policy.Policy, root int64, audit io.Writer) *Service {
	return &Service{api: api, policy: p, root: root, audit: audit}
}

var ErrInput = errors.New("invalid input or resource limit exceeded")
var ErrScope = errors.New("object outside the permitted owned folder tree")

func (s *Service) identity(ctx context.Context, op string) (model.Identity, error) {
	if err := s.policy.Authorize(policy.FromContext(ctx), op, true); err != nil {
		return model.Identity{}, err
	}
	user, err := s.api.Whoami(ctx)
	if err != nil {
		return user, err
	}
	if user.UserID != s.policy.UserID {
		return model.Identity{}, policy.ErrDenied
	}
	return user, nil
}
func (s *Service) Whoami(ctx context.Context) (model.Identity, error) {
	return s.identity(ctx, "whoami")
}
func (s *Service) folder(ctx context.Context, id int64, budget *int) (model.Metadata, error) {
	if id < 0 {
		return model.Metadata{}, ErrInput
	}
	seen := map[int64]bool{}
	current := id
	var target model.Metadata
	for depth := 0; depth < 32; depth++ {
		if seen[current] || *budget <= 0 {
			return target, ErrScope
		}
		seen[current] = true
		*budget--
		m, err := s.api.ListFolder(ctx, current)
		if err != nil {
			return target, err
		}
		if !m.IsMine || !m.IsFolder || m.FolderID != current {
			return target, ErrScope
		}
		if depth == 0 {
			target = m
		}
		if current == s.root {
			return target, nil
		}
		if current == 0 || m.ParentFolderID < 0 {
			return target, ErrScope
		}
		current = m.ParentFolderID
	}
	return target, ErrScope
}
func (s *Service) file(ctx context.Context, id int64, budget *int) (model.Metadata, error) {
	if id <= 0 || *budget <= 0 {
		return model.Metadata{}, ErrInput
	}
	*budget--
	m, err := s.api.Stat(ctx, id)
	if err != nil {
		return m, err
	}
	if !m.IsMine || m.IsFolder || m.FileID != id || m.Size < 0 {
		return model.Metadata{}, ErrScope
	}
	if _, err = s.folder(ctx, m.ParentFolderID, budget); err != nil {
		return model.Metadata{}, err
	}
	m.Contents = nil
	return m, nil
}

type ListResult struct {
	Entries    []model.Metadata `json:"entries"`
	NextOffset int              `json:"next_offset,omitempty"`
	Truncated  bool             `json:"truncated"`
}

func (s *Service) List(ctx context.Context, id int64, offset, limit int) (ListResult, error) {
	out := ListResult{Entries: []model.Metadata{}}
	if offset < 0 || offset > 10000 || limit < 0 || limit > 200 {
		return out, ErrInput
	}
	if limit == 0 {
		limit = 100
	}
	if _, err := s.identity(ctx, "list_folder"); err != nil {
		return out, err
	}
	budget := 100
	m, err := s.folder(ctx, id, &budget)
	if err != nil {
		return out, err
	}
	visible := make([]model.Metadata, 0, len(m.Contents))
	for _, entry := range m.Contents {
		if validChild(entry, id) {
			entry.Contents = nil
			visible = append(visible, entry)
		}
	}
	if offset >= len(visible) {
		return out, nil
	}
	end := min(offset+limit, len(visible))
	out.Entries = visible[offset:end]
	if end < len(visible) {
		out.Truncated = true
		out.NextOffset = end
	}
	return out, nil
}
func validChild(m model.Metadata, parent int64) bool {
	return m.IsMine && m.ParentFolderID == parent && policy.ValidName(m.Name) && m.Size >= 0 && ((m.IsFolder && m.FolderID > 0) || (!m.IsFolder && m.FileID > 0))
}
func (s *Service) Info(ctx context.Context, id int64) (model.Metadata, error) {
	if _, err := s.identity(ctx, "get_file_info"); err != nil {
		return model.Metadata{}, err
	}
	budget := 100
	return s.file(ctx, id, &budget)
}

type TextResult struct {
	Text      string `json:"text"`
	Untrusted bool   `json:"untrusted_content"`
}

func (s *Service) ReadText(ctx context.Context, id int64) (TextResult, error) {
	if _, err := s.identity(ctx, "read_text_file"); err != nil {
		return TextResult{}, err
	}
	budget := 100
	m, err := s.file(ctx, id, &budget)
	if err != nil {
		return TextResult{}, err
	}
	switch strings.ToLower(path.Ext(m.Name)) {
	case ".txt", ".md", ".csv", ".tsv", ".json", ".yaml", ".yml", ".xml", ".log":
	default:
		return TextResult{}, errors.New("unsupported text file extension")
	}
	if m.Size > pcloud.MaxTextBytes {
		return TextResult{}, ErrInput
	}
	text, err := s.api.Text(ctx, id)
	if err != nil {
		return TextResult{}, err
	}
	return TextResult{Text: text, Untrusted: true}, nil
}

type SearchInput struct {
	FolderID       int64  `json:"folder_id" jsonschema:"Root folder to search within the permitted tree"`
	Name           string `json:"name,omitempty" jsonschema:"Case-insensitive filename substring"`
	Path           string `json:"path_contains,omitempty" jsonschema:"Case-insensitive relative path substring"`
	Extension      string `json:"extension,omitempty" jsonschema:"Extension including the dot, e.g. .pdf"`
	MIME           string `json:"mime_type,omitempty"`
	MinSize        int64  `json:"min_size,omitempty"`
	MaxSize        int64  `json:"max_size,omitempty"`
	ModifiedAfter  string `json:"modified_after,omitempty" jsonschema:"RFC3339 timestamp"`
	ModifiedBefore string `json:"modified_before,omitempty" jsonschema:"RFC3339 timestamp"`
	Limit          int    `json:"limit,omitempty" jsonschema:"Maximum matches, 1 to 100; default 50"`
}
type Match struct {
	Metadata model.Metadata `json:"metadata"`
	Path     string         `json:"path"`
}
type SearchResult struct {
	Files          []Match `json:"files"`
	Truncated      bool    `json:"truncated"`
	FoldersScanned int     `json:"folders_scanned"`
	EntriesScanned int     `json:"entries_scanned"`
}

func (s *Service) Search(ctx context.Context, in SearchInput) (SearchResult, error) {
	out := SearchResult{Files: []Match{}}
	if in.Limit < 0 || in.Limit > 100 || in.MinSize < 0 || in.MaxSize < 0 || (in.MaxSize > 0 && in.MaxSize < in.MinSize) || len(in.Name) > 255 || len(in.Path) > 1024 || len(in.Extension) > 64 || len(in.MIME) > 128 {
		return out, ErrInput
	}
	if in.Limit == 0 {
		in.Limit = 50
	}
	var after, before time.Time
	var err error
	if in.ModifiedAfter != "" {
		after, err = time.Parse(time.RFC3339, in.ModifiedAfter)
		if err != nil {
			return out, ErrInput
		}
	}
	if in.ModifiedBefore != "" {
		before, err = time.Parse(time.RFC3339, in.ModifiedBefore)
		if err != nil {
			return out, ErrInput
		}
	}
	if !after.IsZero() && !before.IsZero() && after.After(before) {
		return out, ErrInput
	}
	if _, err := s.identity(ctx, "search_files"); err != nil {
		return out, err
	}
	budget := 100
	type node struct {
		id       int64
		relative string
		depth    int
	}
	queue := []node{{in.FolderID, "/", 0}}
	seen := map[int64]bool{}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if out.FoldersScanned >= 20 || budget <= 0 {
			out.Truncated = true
			break
		}
		n := queue[0]
		queue = queue[1:]
		if seen[n.id] {
			out.Truncated = true
			continue
		}
		seen[n.id] = true
		folder, err := s.folder(ctx, n.id, &budget)
		if err != nil {
			return SearchResult{}, err
		}
		out.FoldersScanned++
		for _, m := range folder.Contents {
			out.EntriesScanned++
			if out.EntriesScanned > 2000 {
				out.Truncated = true
				return out, nil
			}
			if !validChild(m, n.id) {
				continue
			}
			relative := path.Join(n.relative, m.Name)
			m.Contents = nil
			if m.IsFolder {
				if n.depth < 8 && len(queue) < 100 {
					queue = append(queue, node{m.FolderID, relative, n.depth + 1})
				} else {
					out.Truncated = true
				}
				continue
			}
			if !strings.Contains(strings.ToLower(m.Name), strings.ToLower(in.Name)) || !strings.Contains(strings.ToLower(relative), strings.ToLower(in.Path)) {
				continue
			}
			if in.Extension != "" && !strings.EqualFold(path.Ext(m.Name), in.Extension) {
				continue
			}
			if in.MIME != "" && !strings.EqualFold(m.ContentType, in.MIME) {
				continue
			}
			if m.Size < in.MinSize || (in.MaxSize > 0 && m.Size > in.MaxSize) {
				continue
			}
			if !after.IsZero() || !before.IsZero() {
				modified, e := time.Parse(time.RFC1123Z, m.Modified)
				if e != nil || (!after.IsZero() && modified.Before(after)) || (!before.IsZero() && modified.After(before)) {
					continue
				}
			}
			out.Files = append(out.Files, Match{m, relative})
			if len(out.Files) >= in.Limit {
				out.Truncated = true
				return out, nil
			}
		}
	}
	return out, nil
}
func (s *Service) auditEvent(requestID, op, result string, objectID int64) error {
	if s.audit == nil {
		return errors.New("audit unavailable")
	}
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	return json.NewEncoder(s.audit).Encode(struct {
		Time      time.Time `json:"time"`
		RequestID string    `json:"request_id"`
		UserID    int64     `json:"user_id"`
		Operation string    `json:"operation"`
		ObjectID  int64     `json:"object_id"`
		Result    string    `json:"result"`
	}{time.Now().UTC(), requestID, s.policy.UserID, op, objectID, result})
}
func (s *Service) write(op string, target, parent int64, fn func() (model.Metadata, error)) (model.Metadata, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return model.Metadata{}, errors.New("request ID unavailable")
	}
	requestID := hex.EncodeToString(random[:])
	if err := s.auditEvent(requestID, op, "started", target); err != nil {
		return model.Metadata{}, errors.New("write refused: audit unavailable")
	}
	m, err := fn()
	if err == nil && (!validChild(m, parent) || m.IsFolder != (op == "create_folder")) {
		m = model.Metadata{}
		err = errors.New("write outcome uncertain: unexpected provider metadata; inspect pCloud before retry")
	}
	result := "succeeded"
	if err != nil {
		result = "failed_or_unknown"
	}
	if e := s.auditEvent(requestID, op, result, target); e != nil {
		return model.Metadata{}, errors.New("write outcome uncertain: final audit failed; inspect pCloud before retry")
	}
	m.Contents = nil
	return m, err
}
func (s *Service) CreateFolder(ctx context.Context, parent int64, name string) (model.Metadata, error) {
	if !policy.ValidName(name) {
		return model.Metadata{}, ErrInput
	}
	if _, err := s.identity(ctx, "create_folder"); err != nil {
		return model.Metadata{}, err
	}
	budget := 100
	if _, err := s.folder(ctx, parent, &budget); err != nil {
		return model.Metadata{}, err
	}
	return s.write("create_folder", parent, parent, func() (model.Metadata, error) { return s.api.CreateFolder(ctx, parent, name) })
}
func (s *Service) Copy(ctx context.Context, fileID, parent int64, name string) (model.Metadata, error) {
	if !policy.ValidName(name) {
		return model.Metadata{}, ErrInput
	}
	if _, err := s.identity(ctx, "copy_file"); err != nil {
		return model.Metadata{}, err
	}
	budget := 100
	if _, err := s.file(ctx, fileID, &budget); err != nil {
		return model.Metadata{}, err
	}
	if _, err := s.folder(ctx, parent, &budget); err != nil {
		return model.Metadata{}, err
	}
	return s.write("copy_file", fileID, parent, func() (model.Metadata, error) { return s.api.Copy(ctx, fileID, parent, name) })
}

type UploadResult struct {
	Metadata         model.Metadata `json:"metadata"`
	SHA256           string         `json:"sha256"`
	ConflictBehavior string         `json:"conflict_behavior"`
}

func (s *Service) Upload(ctx context.Context, op string, parent int64, name, encoded, digest string) (UploadResult, error) {
	if (op != "upload_file" && op != "save_artifact") || !policy.ValidName(name) || len(encoded) > base64.StdEncoding.EncodedLen(pcloud.MaxUploadBytes) || len(digest) != 64 {
		return UploadResult{}, ErrInput
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) > pcloud.MaxUploadBytes {
		return UploadResult{}, ErrInput
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, digest) {
		return UploadResult{}, errors.New("SHA-256 integrity check failed")
	}
	if _, err := s.identity(ctx, op); err != nil {
		return UploadResult{}, err
	}
	budget := 100
	if _, err := s.folder(ctx, parent, &budget); err != nil {
		return UploadResult{}, err
	}
	meta, err := s.write(op, parent, parent, func() (model.Metadata, error) { return s.api.Upload(ctx, parent, name, data) })
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{meta, actual, "pCloud chooses a new filename on conflict; inspect returned metadata.name"}, nil
}
