# M0a local verification

Verified on 2026-10-09 with Go 1.26.9 on Linux amd64 and MCP Go SDK v1.8.0.
Tests were first run before implementation and failed for missing `config.Load`
and the missing executable. The completed implementation passed:

| Check | Observed result |
| --- | --- |
| Build with `-trimpath` | Binary built |
| `go test -race -count=1 -timeout=90s ./...` | Configuration and process integration tests passed |
| `go vet ./...` | Passed |
| `go mod verify` | All modules verified |
| Configuration fuzzing, 30 seconds, two workers | Passed; 2,484,907 executions in this run |
| Targeted mutations | All three killed by test failures, not compilation errors |
| `scripts/smoke_test.py` against built binary | Discovery returned identity `pcloud-mcp`, version `0.2.0-m0a`, supported MCP version `2026-07-28` |
| Module-level `govulncheck` | No vulnerabilities found after dependency update |
| Gitleaks working-directory and existing-history scans | No leaks found |

The module scan originally reported GO-2026-5024 in `golang.org/x/sys v0.41.0`,
a Windows issue not called by this Linux build. The dependency was explicitly
updated to the reported fixed version v0.44.0; the MCP SDK remains v1.8.0.

The transport tests use raw JSON-RPC against a subprocess, not an in-process
mock. For an unknown modern version, they use `2099-01-01`. A pre-modern version
such as `1900-01-01` follows different SDK compatibility behavior and is not
claimed to have passed the modern unsupported-version test.

Fuzz execution counts are machine-dependent and not acceptance thresholds.
The mutation set covers configuration validation, its maximum boundary, and
the transport limit. It is not a general mutation score or authorization proof.

Full conformance, real ChatGPT/Claude interoperability, OAuth, pCloud API calls,
tenant isolation and file-operation security are not yet verified. Hosted CI
results must be checked independently after the commit is published.
