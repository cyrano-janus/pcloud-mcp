#!/usr/bin/env python3
"""Unauthenticated post-deploy probes; never reads or writes pCloud files."""
import argparse
import datetime
import json
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def verify(base, mode, opener=None):
    parsed = urllib.parse.urlsplit(base)
    if (parsed.scheme != "https" or not parsed.hostname or parsed.username
            or parsed.password or parsed.query or parsed.fragment
            or parsed.path not in ("", "/") or parsed.port not in (None, 443)):
        raise ValueError("Use the actual HTTPS service origin without path, credentials or query")
    base = base.rstrip("/")
    opener = opener or urllib.request.build_opener(
        urllib.request.ProxyHandler({}), NoRedirect(),
        urllib.request.HTTPSHandler(context=ssl.create_default_context()))
    results = []

    def probe(name, path, expected, data=None, headers=None, validate=None):
        req = urllib.request.Request(base + path, data=data, headers=headers or {})
        try:
            try:
                response = opener.open(req, timeout=45)
            except urllib.error.HTTPError as exc:
                response = exc
            with response:
                body = response.read(65537)
                ok = response.status in expected and len(body) <= 65536
                if validate and ok:
                    ok = bool(validate(response.headers, body))
                results.append({"check": name, "status": response.status, "passed": ok})
        except Exception as exc:
            # Do not print response bodies, URLs or headers that may contain secrets.
            results.append({"check": name, "passed": False, "error_type": type(exc).__name__})

    probe("https_health", "/healthz", {200})
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/list",
                       "params": {"_meta": {"io.modelcontextprotocol/protocolVersion": "2026-07-28"}}}).encode()
    headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream",
               "MCP-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/list"}
    if mode == "readiness":
        for method in ("GET", "POST", "DELETE"):
            req = urllib.request.Request(base + "/mcp", method=method)
            try:
                try:
                    res = opener.open(req, timeout=45)
                except urllib.error.HTTPError as exc:
                    res = exc
                with res:
                    results.append({"check": "mcp_disabled_" + method, "status": res.status,
                                    "passed": res.status == 503})
            except Exception as exc:
                results.append({"check": "mcp_disabled_" + method, "passed": False,
                                "error_type": type(exc).__name__})
    else:
        def metadata(_, raw):
            data = json.loads(raw)
            issuers = data.get("authorization_servers", [])
            return (data.get("resource") == base + "/mcp" and bool(issuers)
                    and all(urllib.parse.urlsplit(v).scheme == "https" for v in issuers))
        probe("oauth_resource_metadata", "/.well-known/oauth-protected-resource/mcp", {200}, validate=metadata)
        probe("unauthenticated_tools_denied", "/mcp", {401}, body, headers,
              lambda h, _: "resource_metadata" in h.get("WWW-Authenticate", ""))
        probe("invalid_bearer_denied", "/mcp", {401}, body,
              dict(headers, Authorization="Bearer deployment-probe-invalid"))
        probe("foreign_origin_denied", "/mcp", {403}, body,
              dict(headers, Origin="https://untrusted.invalid"))
        probe("query_token_denied", "/mcp?access_token=deployment-probe-invalid", {400}, body, headers)
    return {"tested_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "service_url": base, "mode": mode, "checks": results,
            "passed": all(r["passed"] for r in results),
            "not_tested": ["authenticated MCP operation", "real pCloud access", "OAuth login and revocation"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("url")
    parser.add_argument("--mode", choices=("readiness", "protected"), default="protected")
    parser.add_argument("--report", default="deployment-report.json")
    args = parser.parse_args()
    try:
        report = verify(args.url, args.mode)
    except ValueError as exc:
        parser.error(str(exc))
    with open(args.report, "w", encoding="utf-8") as target:
        json.dump(report, target, indent=2)
        target.write("\n")
    print(json.dumps(report, indent=2))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    sys.exit(main())
