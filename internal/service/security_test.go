package service

import (
	"context"
	"github.com/cyrano-janus/pcloud-mcp/internal/model"
	"github.com/cyrano-janus/pcloud-mcp/internal/pcloud"
	"strings"
	"testing"
)

func TestSearchLimitsAndFilters(t *testing.T) {
	s, f, ctx := fixture()
	folder := f.folders[10]
	folder.Contents = []model.Metadata{{FileID: 11, Name: "report.txt", IsMine: true, ParentFolderID: 10, Size: 42, ContentType: "text/plain", Modified: "Thu, 08 Oct 2026 10:00:00 +0000"}, {FileID: 12, Name: "foreign.txt", ParentFolderID: 10}}
	f.folders[10] = folder
	result, err := s.Search(ctx, SearchInput{FolderID: 10, Name: "REPORT", Path: "report", Extension: ".txt", MIME: "text/plain", MinSize: 10, MaxSize: 100, ModifiedAfter: "2026-10-01T00:00:00Z", ModifiedBefore: "2026-10-09T00:00:00Z"})
	if err != nil || len(result.Files) != 1 {
		t.Fatal("filters failed", err)
	}
	if _, err := s.Search(ctx, SearchInput{FolderID: 10, ModifiedAfter: "invalid"}); err == nil {
		t.Fatal("invalid date accepted")
	}
	folder.Contents = nil
	for i := int64(1); i <= 2100; i++ {
		folder.Contents = append(folder.Contents, model.Metadata{FileID: i, Name: "x.txt", IsMine: true, ParentFolderID: 10})
	}
	f.folders[10] = folder
	result, err = s.Search(ctx, SearchInput{FolderID: 10, Name: "not present"})
	if err != nil || !result.Truncated || result.EntriesScanned > 2001 {
		t.Fatal("search budget not enforced", err)
	}
}
func TestTextAndUploadLimits(t *testing.T) {
	s, f, ctx := fixture()
	f.file.Size = pcloud.MaxTextBytes + 1
	if _, err := s.ReadText(ctx, 11); err == nil || f.texts != 0 {
		t.Fatal("oversized text read")
	}
	if _, err := s.Upload(ctx, "save_artifact", 10, "x.txt", strings.Repeat("A", ((pcloud.MaxUploadBytes+2)/3)*4+4), strings.Repeat("0", 64)); err == nil || f.writes != 0 {
		t.Fatal("oversized upload accepted")
	}
	if _, err := s.Copy(ctx, 11, 0, "copy.txt"); err == nil || f.writes != 0 {
		t.Fatal("copy outside root accepted")
	}
}
func TestListDoesNotLeakForeignOrNestedEntries(t *testing.T) {
	s, f, ctx := fixture()
	folder := f.folders[10]
	folder.Contents = append(folder.Contents, model.Metadata{FileID: 12, Name: "foreign.txt", ParentFolderID: 10})
	f.folders[10] = folder
	list, err := s.List(ctx, 10, 0, 1)
	if err != nil || len(list.Entries) != 1 || list.Entries[0].FileID != 11 || list.Truncated {
		t.Fatal("list leaked foreign entry", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Search(cancelled, SearchInput{FolderID: 10}); err == nil {
		t.Fatal("cancelled search continued")
	}
}

func TestWriteResultMustMatchOwnedDestination(t *testing.T) {
	for _, m := range []model.Metadata{
		{},
		{IsMine: false, FileID: 11, ParentFolderID: 10, Name: "x"},
		{IsMine: true, FileID: 11, ParentFolderID: 0, Name: "x"},
		{IsMine: true, FolderID: 11, IsFolder: true, ParentFolderID: 10, Name: "x"},
	} {
		s, _, _ := fixture()
		if _, err := s.write("copy_file", 11, 10, func() (model.Metadata, error) { return m, nil }); err == nil {
			t.Fatal("invalid mutation result reported as success")
		}
	}
}
