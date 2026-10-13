#!/usr/bin/env python3
"""Targeted, deterministic mutation checks in disposable copies, not the checkout.

This is a finite regression set, not a general mutation score. Future policy and
authorization code must add its own mutations and tests.
"""
import pathlib
import shutil
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
MUTATIONS = [
    ("config validation removed", "internal/config/config.go",
     "err != nil || n < MinFrameBytes || n > MaxFrameBytesLimit",
     "false", "./internal/config", "TestLoad"),
    ("maximum boundary shifted", "internal/config/config.go",
     "n > MaxFrameBytesLimit", "n >= MaxFrameBytesLimit",
     "./internal/config", "TestLoad"),
    ("transport limit increased by one", "internal/server/server.go",
     "MaxLineLength: cfg.MaxFrameBytes", "MaxLineLength: cfg.MaxFrameBytes + 1",
     "./internal/server", "TestFrameBoundary"),
    ("identity binding removed", "internal/policy/policy.go",
     "actor.UserID != p.UserID || ", "",
     "./internal/policy", "TestAuthorization"),
    ("write switch removed", "internal/policy/policy.go",
     "p.EnableWrites && actor.Write", "actor.Write",
     "./internal/policy", "TestAuthorization"),
    ("destructive operation allowed", "internal/policy/policy.go",
     'case "whoami",', 'case "delete_file", "whoami",',
     "./internal/policy", "TestAuthorization"),
    ("root restriction removed", "internal/service/service.go",
     "current == s.root", "true",
     "./internal/service", "TestScopeAndOwnership"),
    ("file ownership removed", "internal/service/service.go",
     "!m.IsMine || m.IsFolder", "m.IsFolder",
     "./internal/service", "TestScopeAndOwnership"),
    ("upload integrity removed", "internal/service/service.go",
     "!strings.EqualFold(actual, digest)", "false",
     "./internal/service", "TestUploadIntegrity"),
    ("audience binding removed", "internal/remote/http.go",
     "!slices.Contains(info.Audience, cfg.PublicURL)", "false",
     "./internal/remote", "TestTokenValidation"),
    ("issuer binding removed", "internal/remote/http.go",
     "info.Issuer != cfg.Issuer || ", "",
     "./internal/remote", "TestTokenValidation"),
    ("subject binding removed", "internal/remote/http.go",
     "info.Subject != cfg.OAuthSubject || ", "",
     "./internal/remote", "TestTokenValidation"),
    ("copy no-overwrite removed", "internal/pcloud/client.go",
     '"noover": {"1"}', '"noover": {"0"}',
     "./internal/pcloud", "TestTokenAndCopySafety"),
    ("upload conflict protection removed", "internal/pcloud/client.go",
     '{"renameifexists", "1"}', '{"renameifexists", "0"}',
     "./internal/pcloud", "TestMultipartSafety"),
    ("hosted setup secret removed", "internal/onboarding/http.go",
     '!equal(r.PostForm.Get("setup_secret"), h.cfg.SetupSecret)', 'false',
     "./internal/onboarding", "TestSetupRequiresSecretCSRFAndOrigin"),
    ("hosted setup CSRF removed", "internal/onboarding/http.go",
     '!validProof', 'false',
     "./internal/onboarding", "TestSetupRequiresSecretCSRFAndOrigin"),
    ("hosted setup origin removed", "internal/onboarding/http.go",
     'origin != h.cfg.Origin', 'false',
     "./internal/onboarding", "TestSetupRequiresSecretCSRFAndOrigin"),
    ("hosted setup expiry removed", "internal/onboarding/http.go",
     '!h.now().Before(s.expires)', 's.expires.IsZero()',
     "./internal/onboarding", "TestCallbackBrowserStateExpiryAndReplay"),
    ("hosted callback replay allowed", "internal/onboarding/http.go",
     'delete(h.sessions, id)\n\th.mu.Unlock()', 'h.mu.Unlock()',
     "./internal/onboarding", "TestCallbackBrowserStateExpiryAndReplay"),
    ("encrypted credential owner removed", "internal/config/runtime.go",
     'encryptedOwner != 0 && cfg.UserID != encryptedOwner', 'encryptedOwner < 0',
     "./internal/config", "TestEncryptedCredentialOwnerBinding"),
    ("setup form browser MAC removed", "internal/onboarding/http.go",
     'equal(proof, h.formProof(id, parts[0]))', 'true',
     "./internal/onboarding", "TestFormProofBrowserExpiryAndTampering"),
    ("health independence removed", "internal/remote/http.go",
     'r.URL.Path == "/healthz"', 'r.URL.Path == "/unused-health"',
     "./internal/remote", "TestHealthIndependentOfPublicRequestBudget"),
    ("unsafe startup error exposed", "cmd/pcloud-remote/main.go",
     'return "HTTP readiness service stopped"', 'return err.Error()',
     "./cmd/pcloud-remote", "TestStartupDiagnosticsNeverExposeValues"),
    ("setup form browser Origin suppressed", "internal/onboarding/http.go",
     'w.Header().Set("Referrer-Policy", "same-origin")', 'w.Header().Set("Referrer-Policy", "no-referrer")',
     "./internal/onboarding", "TestSetupFormPreservesSameOriginPost"),
    ("unsafe provider error exposed", "internal/onboarding/http.go",
     'category := "unknown"', 'category := err.Error()',
     "./internal/onboarding", "TestProviderFailureDiagnosticsAreRedacted"),
    ("conflicting token identity accepted", "internal/pcloud/oauth.go",
     'owner > 0 && owner != id', 'false',
     "./internal/pcloud", "TestOAuthCredentialIdentityFormats"),

    ("native activation flag bypassed", "cmd/pcloud-remote/main.go",
     'enabled != "true" || get("RENDER") != "true"', 'false',
     "./cmd/pcloud-remote", "TestRemoteActivationBoundaries"),
    ("native setup separation bypassed", "cmd/pcloud-remote/main.go",
     'setup != "" && setup != "false"', 'setup != "" && false',
     "./cmd/pcloud-remote", "TestRemoteActivationBoundaries"),
    ("JWT signature check disabled", "internal/remote/jwt.go",
     'SupportedSigningAlgs: []string{"RS256"}', 'SupportedSigningAlgs: []string{"RS256"}, InsecureSkipSignatureCheck: true',
     "./internal/remote", "TestJWTSignatureAndBinding"),
    ("JWT owner binding bypassed", "internal/remote/jwt.go",
     'token.Subject != cfg.OAuthSubject', 'false',
     "./internal/remote", "TestJWTSignatureAndBinding"),
    ("JWT audience check bypassed", "internal/remote/jwt.go",
     'ClientID: cfg.PublicURL', 'ClientID: cfg.PublicURL, SkipClientIDCheck: true',
     "./internal/remote", "TestJWTSignatureAndBinding"),
    ("provider preflight bypassed", "internal/server/server.go",
     'if _, err := svc.List(checkCtx, cfg.RootFolderID, 0, 1); err != nil {',
     'if _, err := svc.List(checkCtx, cfg.RootFolderID, 0, 1); err != nil && false {',
     "./internal/server", "TestProviderPreflightRejectsUnownedRoot"),
    ("video type gate bypassed", "internal/service/video.go",
     'if !(ext == ".mp4"', 'if false && !(ext == ".mp4"',
     "./internal/service", "TestVideoAuthorization"),
    ("video size gate bypassed", "internal/service/video.go",
     'm.Size <= 0 || m.Size > media.MaxVideoBytes', 'm.Size <= 0',
     "./internal/service", "TestVideoAuthorization"),
    ("video host allowlist bypassed", "internal/pcloud/video.go",
     '!strings.HasSuffix(host, ".pcloud.com")', 'false',
     "./internal/pcloud", "TestVideoDownloadBoundary"),
    ("video frame count bypassed", "internal/media/frames.go",
     'len(seconds) > MaxFrames', 'false',
     "./internal/media", "TestFrameLimits"),
    ("video output size bypassed", "internal/media/frames.go",
     'b.Len()+len(p) > MaxFrameBytes', 'false',
     "./internal/media", "TestFrameLimits"),

]

def test(work, package, case):
    return subprocess.run(["go", "test", "-count=1", "-timeout=45s", package,
                           "-run", "^" + case + "$"], cwd=work,
                          capture_output=True, text=True, timeout=60)

for name, filename, before, after, package, case in MUTATIONS:
    with tempfile.TemporaryDirectory(prefix="pcloud-mutation-") as tmp:
        work = pathlib.Path(tmp) / "repo"
        shutil.copytree(ROOT, work, ignore=shutil.ignore_patterns(".git", "bin", "__pycache__", "secrets", ".env", ".env.*"))
        baseline = test(work, package, case)
        if baseline.returncode:
            raise SystemExit("Baseline failed:\n" + baseline.stdout + baseline.stderr)
        path = work / filename
        source = path.read_text()
        if source.count(before) != 1:
            raise SystemExit("Mutation anchor is ambiguous: " + name)
        mutated = source.replace(before, after)
        # Removing the validation also makes err unused. Retain compilation.
        if name == "config validation removed":
            mutated = mutated.replace("if false {", "_ = err\n\t\tif false {")
        if name == "hosted setup CSRF removed":
            mutated = mutated.replace('if !equal(r.PostForm.Get("setup_secret")', '_ = validProof\n\tif !equal(r.PostForm.Get("setup_secret")')
        path.write_text(mutated)
        result = test(work, package, case)
        output = result.stdout + result.stderr
        if result.returncode == 0:
            raise SystemExit("SURVIVED: " + name)
        if "--- FAIL: " not in output or "[build failed]" in output:
            raise SystemExit("Invalid mutation result:\n" + output)
        print("KILLED: " + name)
