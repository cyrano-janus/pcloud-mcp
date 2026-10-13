// Package media extracts bounded still images, never classifies a dance.
package media

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"math"
	"os/exec"
	"strconv"
	"time"
)

const MaxVideoBytes = 128 << 20
const MaxFrameBytes = 128 << 10
const MaxFrames = 4

var ErrMedia = errors.New("video preview unavailable, invalid media or extraction limit exceeded")
var slots = make(chan struct{}, 1)

func ValidSeconds(seconds []float64) bool {
	if len(seconds) < 1 || len(seconds) > MaxFrames {
		return false
	}
	for i, s := range seconds {
		if math.IsNaN(s) || math.IsInf(s, 0) || s < 0 || s > 600 || (i > 0 && s <= seconds[i-1]) {
			return false
		}
	}
	return true
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > MaxFrameBytes {
		return 0, ErrMedia
	}
	return b.Buffer.Write(p)
}

// Linux prlimit applies per-child address-space, CPU, descriptor and output-file
// limits. FFmpeg receives no application secrets and no network protocols.
func Frames(ctx context.Context, file string, seconds []float64) ([][]byte, error) {
	if !ValidSeconds(seconds) {
		return nil, ErrMedia
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return nil, errors.New("video preview busy")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.New("video preview requires FFmpeg")
	}
	limiter, err := exec.LookPath("prlimit")
	if err != nil {
		return nil, errors.New("video preview requires Linux prlimit")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out := make([][]byte, 0, len(seconds))
	for _, second := range seconds {
		args := []string{"--as=402653184", "--cpu=15", "--nofile=64", "--fsize=1048576", "--", ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-max_alloc", "33554432", "-threads", "1", "-protocol_whitelist", "file,pipe", "-format_whitelist", "mov,matroska,webm", "-ss", strconv.FormatFloat(second, 'f', 3, 64), "-i", file, "-map", "0:v:0", "-an", "-sn", "-dn", "-frames:v", "1", "-vf", "scale=640:360:force_original_aspect_ratio=decrease", "-threads", "1", "-q:v", "5", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1"}
		cmd := exec.CommandContext(ctx, limiter, args...)
		cmd.Env = []string{"LC_ALL=C", "OMP_NUM_THREADS=1", "OPENBLAS_NUM_THREADS=1"}
		cmd.WaitDelay = time.Second
		var b boundedBuffer
		cmd.Stdout = &b
		if cmd.Run() != nil {
			return nil, ErrMedia
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(b.Bytes()))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 640 || cfg.Height > 360 {
			return nil, ErrMedia
		}
		out = append(out, append([]byte(nil), b.Bytes()...))
	}
	return out, nil
}
