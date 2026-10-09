package server_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pcloud-mcp-test-")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "pcloud-mcp")
	cmd := exec.Command("go", "build", "-race", "-o", binary, "../../cmd/pcloud-mcp")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func request(method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
}

// Each call gets a fresh subprocess; the modern protocol needs no initialize handshake.
func exchange(t *testing.T, frame []byte) (map[string]any, string) {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Env = append(cleanEnv(), "PCLOUD_MCP_MAX_FRAME_BYTES=1024")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	done := make(chan struct {
		value map[string]any
		err   error
	}, 1)
	go func() {
		var value map[string]any
		err := json.NewDecoder(stdout).Decode(&value)
		done <- struct {
			value map[string]any
			err   error
		}{value, err}
	}()
	if _, err := stdin.Write(append(frame, '\n')); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	select {
	case result := <-done:
		value = result.value
		if result.err != nil && result.err != io.EOF {
			t.Fatalf("invalid stdout: %v", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("response timeout")
	}
	_ = stdin.Close()
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case <-wait:
	case <-time.After(5 * time.Second):
		t.Fatal("EOF shutdown timeout")
	}
	return value, stderr.String()
}

func cleanEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PCLOUD_MCP_") {
			env = append(env, entry)
		}
	}
	return env
}

func frame(t *testing.T, method, version string) []byte {
	t.Helper()
	params := map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": version, "io.modelcontextprotocol/clientInfo": map[string]any{"name": "test", "version": "1"}, "io.modelcontextprotocol/clientCapabilities": map[string]any{}}}
	if method == "tools/call" {
		params["name"] = "delete_file"
		params["arguments"] = map[string]any{}
	}
	data, err := json.Marshal(request(method, params))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDiscovery(t *testing.T) {
	res, stderr := exchange(t, frame(t, "server/discover", "2026-07-28"))
	if res["error"] != nil || res["result"] == nil {
		t.Fatalf("discovery failed: %v; stderr=%s", res, stderr)
	}
	encoded, _ := json.Marshal(res["result"])
	if !bytes.Contains(encoded, []byte("2026-07-28")) || !bytes.Contains(encoded, []byte("pcloud-mcp")) {
		t.Fatalf("missing identity/version: %s", encoded)
	}
	result := res["result"].(map[string]any)
	versions, ok := result["supportedVersions"].([]any)
	if !ok || len(versions) != 1 || versions[0] != "2026-07-28" {
		t.Fatalf("unexpected supported versions: %v", versions)
	}
	capabilities, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("missing capabilities")
	}
	for _, feature := range []string{"tools", "resources", "prompts"} {
		if _, present := capabilities[feature]; present {
			t.Fatalf("unimplemented capability advertised: %s", feature)
		}
	}
}

func TestUnsupportedVersion(t *testing.T) {
	res, _ := exchange(t, frame(t, "server/discover", "2099-01-01"))
	errValue, ok := res["error"].(map[string]any)
	if !ok || errValue["code"] != float64(-32022) {
		t.Fatalf("wrong version error: %v", res)
	}
	data, _ := json.Marshal(errValue["data"])
	if !bytes.Contains(data, []byte("2026-07-28")) {
		t.Fatalf("missing supported versions: %s", data)
	}
}

func TestNoPCloudTools(t *testing.T) {
	res, _ := exchange(t, frame(t, "tools/call", "2026-07-28"))
	if res["error"] == nil {
		t.Fatalf("unregistered destructive tool succeeded: %v", res)
	}
}

func TestFrameBoundary(t *testing.T) {
	base := frame(t, "server/discover", "2026-07-28")
	for _, size := range []int{1024, 1025} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			// Pad within the JSON object; trailing whitespace is not MCP framing.
			padded := append(append([]byte{}, base[:len(base)-1]...), bytes.Repeat([]byte(" "), size-len(base))...)
			padded = append(padded, '}')
			res, _ := exchange(t, padded)
			if size == 1024 && res["result"] == nil {
				t.Fatalf("frame at limit rejected: %v", res)
			}
			if size == 1025 && res["result"] != nil {
				t.Fatal("oversized frame accepted")
			}
		})
	}
}

func TestInvalidConfigDoesNotLeak(t *testing.T) {
	cmd := exec.Command(binary)
	cmd.Env = append(cleanEnv(), "PCLOUD_MCP_MAX_FRAME_BYTES=sentinel-secret")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("invalid configuration accepted")
	}
	if bytes.Contains(out, []byte("sentinel-secret")) {
		t.Fatal("configuration leaked")
	}
}

func TestSignalShutdown(t *testing.T) {
	cmd := exec.Command(binary)
	cmd.Env = cleanEnv()
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	_, _ = stdin.Write(append(frame(t, "server/discover", "2026-07-28"), '\n'))
	ready := make(chan error, 1)
	go func() { _, err := bufio.NewReader(stdout).ReadBytes('\n'); ready <- err }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("startup timeout")
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		if err != nil {
			t.Fatalf("unclean signal shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("signal shutdown timeout")
	}
}
