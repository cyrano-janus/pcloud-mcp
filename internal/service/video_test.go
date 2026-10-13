package service

import (
	"context"
	"github.com/cyrano-janus/pcloud-mcp/internal/media"
	"io"
	"os"
	"testing"
)

type videoAPI struct {
	*fakeAPI
	reads  int
	staged string
}

func (f *videoAPI) Video(_ context.Context, _ int64, w io.Writer, size int64) error {
	f.reads++
	if file, ok := w.(*os.File); ok {
		f.staged = file.Name()
	}
	_, err := w.Write([]byte("fixture"))
	return err
}
func TestVideoAuthorization(t *testing.T) {
	for _, kind := range []string{"outside", "foreign", "identity", "missing principal", "type", "size", "input"} {
		t.Run(kind, func(t *testing.T) {
			s, f, ctx := fixture()
			v := &videoAPI{fakeAPI: f}
			s.api = v
			f.file.Name = "clip.mp4"
			f.file.ContentType = "video/mp4"
			in := VideoInput{FileID: 11, Seconds: []float64{0, 1}}
			switch kind {
			case "outside":
				f.file.ParentFolderID = 0
			case "foreign":
				f.file.IsMine = false
			case "identity":
				f.user = 8
			case "missing principal":
				ctx = context.Background()
			case "type":
				f.file.Name = "evil.m3u8"
			case "size":
				f.file.Size = media.MaxVideoBytes + 1
			case "input":
				in.Seconds = []float64{-1}
			}
			if _, err := s.VideoFrames(ctx, in); err == nil {
				t.Fatal("unsafe video accepted")
			}
			if v.reads != 0 {
				t.Fatal("unauthorized download")
			}
		})
	}
}

func TestVideoFailureCleanup(t *testing.T) {
	s, f, ctx := fixture()
	v := &videoAPI{fakeAPI: f}
	s.api = v
	f.file.Name = "clip.mp4"
	f.file.ContentType = "video/mp4"
	if _, err := s.VideoFrames(ctx, VideoInput{FileID: 11, Seconds: []float64{0}}); err == nil {
		t.Fatal("invalid video accepted")
	}
	if v.reads != 1 || v.staged == "" {
		t.Fatal("download not exercised")
	}
	if _, err := os.Stat(v.staged); !os.IsNotExist(err) {
		t.Fatal("private temporary video retained")
	}
}
