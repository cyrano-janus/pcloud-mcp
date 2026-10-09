# M0a security boundaries and OAuth concept

## Current implementation

M0a is a client-launched, local stdio process. It opens no HTTP listener, makes
no pCloud calls, accepts no credentials and registers no tools, resources or
prompts. It is a protocol foundation, not a file-management release.

The local process owner's OS permissions are the trust boundary. Only launch
the executable from a trusted local client and protect the binary and client
configuration from modification by other users. stdio is not remote-user
authentication. Do not expose the process through an unauthenticated bridge.

## Threat model

| Threat | Current control | Remaining work |
| --- | --- | --- |
| Oversized protocol input | SDK frame cap; default 1 MiB, configurable from 1024 bytes to 1 MiB; boundary tests | Concurrency, request budgets and response limits with real tools |
| Untrusted configuration | Strict integer/range validation; values omitted from errors | Secret-store integration with credentials |
| Untrusted protocol errors | Process-level diagnostics omit SDK error details; stdout reserved for MCP | Review and redact all future service/API errors |
| Unauthorized file changes | No tools registered; destructive tool call test fails | Identity, ownership and default-deny policy before M1 |
| Dependency compromise | SDK pin, go.sum verification, pinned CI actions/tools, vulnerability and secret scans | Regular reviewed updates; scans do not prove absence of vulnerabilities |
| Local malicious client | No pCloud capability or credentials present | OS isolation; limits do not prevent every CPU/resource exhaustion attack |

No protection against multi-tenant attacks, SSRF, replay or token theft is
claimed for M0a: their corresponding network and credential paths do not exist.

## Remote OAuth design obligations (not implemented)

Before adding Streamable HTTP, specify and test:

1. TLS termination, trusted proxy/host configuration and MCP authorization
   against specification 2026-07-28.
2. The authorization-server choice, protected-resource metadata and correct
   resource/audience binding. MCP credentials and pCloud credentials are
   separate; do not pass arbitrary incoming MCP tokens through to pCloud.
3. Verified pCloud EU/US endpoints, actual OAuth/scopes semantics and grant
   lifecycle from primary documentation. These facts remain open in SPEC.md.
4. PKCE S256, redirect allowlists, CSRF/state protection, expiry and revocation;
   persistent grant/refresh state where required. Do not claim rotation without
   replay detection and invalidation.
5. Per-user credential, authorization and audit isolation. Never accept a
   client-supplied user ID as proof of identity.

Future request path: verified identity -> policy enforcement -> tool handler
-> domain service -> pCloud API client. A handler must never call pCloud HTTP
directly. Interfaces and ownership checks must be implemented with the first
real use case, with TDD and policy/authorization mutations.

## Release boundary

The current tests are bounded application protocol regression tests, not a full
MCP conformance assessment or ChatGPT/Claude compatibility certification.
Real client interoperability and security acceptance remain required by SPEC.md.
No external hosting provider has been selected in the repository.
