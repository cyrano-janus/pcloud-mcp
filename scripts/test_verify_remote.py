import importlib.util
import io
import json
import pathlib
import unittest
import urllib.error

spec = importlib.util.spec_from_file_location("probe", pathlib.Path(__file__).with_name("verify_remote.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class Response(io.BytesIO):
    def __init__(self, status, body=b"", headers=None):
        super().__init__(body)
        self.status = status
        self.headers = headers or {}


class Opener:
    def __init__(self, statuses, mode="protected"):
        self.statuses = iter(statuses)
        self.mode = mode

    def open(self, req, timeout):
        status = next(self.statuses)
        if status == 302:
            raise urllib.error.HTTPError(req.full_url, 302, "redirect", {}, io.BytesIO())
        body = json.dumps({"resource": "https://service.example/mcp",
                           "authorization_servers": ["https://identity.example"]}).encode()
        return Response(status, body, {"WWW-Authenticate": 'Bearer resource_metadata="https://service.example/.well-known/oauth-protected-resource/mcp"'})


class DeploymentProbeTests(unittest.TestCase):
    def test_protected_boundaries(self):
        report = probe.verify("https://service.example", "protected", Opener([200, 200, 401, 401, 403, 400]))
        self.assertTrue(report["passed"])
        self.assertIn("real pCloud access", report["not_tested"])

    def test_successful_unauthenticated_access_fails(self):
        report = probe.verify("https://service.example", "protected", Opener([200, 200, 200, 401, 403, 400]))
        self.assertFalse(report["passed"])

    def test_redirect_is_not_health_success(self):
        report = probe.verify("https://service.example", "readiness", Opener([302, 503, 503, 503]))
        self.assertFalse(report["passed"])

    def test_readiness_requires_disabled_endpoint(self):
        self.assertTrue(probe.verify("https://service.example", "readiness", Opener([200, 503, 503, 503]))["passed"])
        self.assertFalse(probe.verify("https://service.example", "readiness", Opener([200, 200, 503, 503]))["passed"])

    def test_rejects_plaintext_credentials_and_paths(self):
        for url in ["http://service.example", "https://user:secret@service.example", "https://service.example/mcp", "https://service.example?token=x"]:
            with self.subTest(url=url), self.assertRaises(ValueError):
                probe.verify(url, "protected")


if __name__ == "__main__":
    unittest.main()
