package media

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFrameLimits(t *testing.T) {
	for _, s := range [][]float64{nil, {-1}, {601}, {0, 0}, {1, 0}, {0, 1, 2, 3, 4}, {math.NaN()}, {math.Inf(1)}} {
		if ValidSeconds(s) {
			t.Fatal("unsafe times accepted", s)
		}
	}
	if !ValidSeconds([]float64{0, 1, 600}) {
		t.Fatal("valid positions rejected")
	}
	var b boundedBuffer
	if _, err := b.Write(make([]byte, MaxFrameBytes+1)); err == nil || b.Len() != 0 {
		t.Fatal("oversized output accepted")
	}
}

func TestActualVideoFrames(t *testing.T) {
	// Mandatory real decoder test: CI must install/provide FFmpeg and prlimit.
	file := filepath.Join(t.TempDir(), "fixture.mp4")
	cmd := exec.Command("ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=160x120:rate=5", "-t", "2", "-threads", "1", "-pix_fmt", "yuv420p", file)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, data)
	}
	images, err := Frames(context.Background(), file, []float64{0, 1})
	if err != nil || len(images) != 2 {
		t.Fatalf("actual extraction: %v", err)
	}
	if _, err := Frames(context.Background(), file, []float64{600}); err == nil {
		t.Fatal("empty out-of-duration frame accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Frames(ctx, file, []float64{0}); err == nil {
		t.Fatal("cancelled extraction accepted")
	}
}

func FuzzFramePositions(f *testing.F) {
	f.Add(0.0, 1.0)
	f.Add(-1.0, 601.0)
	f.Fuzz(func(t *testing.T, a, b float64) {
		if ValidSeconds([]float64{a, b}) && (math.IsNaN(a) || math.IsNaN(b) || a < 0 || b > 600 || b <= a) {
			t.Fatal("unsafe positions")
		}
	})
}
