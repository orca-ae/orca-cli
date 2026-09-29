# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

from __future__ import annotations

import json
import os
import stat
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlencode, urlsplit
from urllib.request import urlopen

from scripts import mcp_oauth_auth_json as oauth


class OAuthFixture:
    def __init__(self, include_refresh=True, require_callback_issuer=False):
        self.include_refresh = include_refresh
        self.require_callback_issuer = require_callback_issuer
        self.registration = None
        self.authorization_query = None
        self.token_form = None

        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def send_json(self, status, payload, headers=None):
                body = json.dumps(payload).encode("utf-8")
                self.send_response(status)
                for name, value in (headers or {}).items():
                    self.send_header(name, value)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_POST(self):  # noqa: N802
                length = int(self.headers.get("Content-Length", "0"))
                body = self.rfile.read(length)
                if self.path == "/mcp":
                    metadata = fixture.base_url + "/.well-known/oauth-protected-resource/mcp"
                    self.send_json(
                        401,
                        {"error": "unauthorized"},
                        {
                            "WWW-Authenticate": (
                                'Bearer resource_metadata="{}", scope="tools.read"'.format(
                                    metadata
                                )
                            )
                        },
                    )
                    return
                if self.path == "/register":
                    fixture.registration = json.loads(body.decode("utf-8"))
                    self.send_json(
                        201,
                        {
                            "client_id": "client-123",
                            "token_endpoint_auth_method": "none",
                        },
                    )
                    return
                if self.path == "/token":
                    fixture.token_form = {
                        key: values[0]
                        for key, values in parse_qs(body.decode("ascii")).items()
                    }
                    payload = {
                        "access_token": "access-secret",
                        "token_type": "bearer",
                        "expires_in": 3600,
                    }
                    if fixture.include_refresh:
                        payload["refresh_token"] = "refresh-secret"
                    self.send_json(200, payload)
                    return
                self.send_json(404, {"error": "not_found"})

            def do_GET(self):  # noqa: N802
                if self.path == "/.well-known/oauth-protected-resource/mcp":
                    self.send_json(
                        200,
                        {
                            "resource": fixture.base_url + "/",
                            "authorization_servers": [fixture.base_url + "/"],
                            "scopes_supported": ["tools.fallback"],
                        },
                    )
                    return
                if self.path == "/.well-known/oauth-authorization-server":
                    self.send_json(
                        200,
                        {
                            "issuer": fixture.base_url + "/",
                            "authorization_endpoint": fixture.base_url + "/authorize",
                            "token_endpoint": fixture.base_url + "/token",
                            "registration_endpoint": fixture.base_url + "/register",
                            "scopes_supported": ["tools.read", "offline_access"],
                            "response_types_supported": ["code"],
                            "grant_types_supported": ["authorization_code", "refresh_token"],
                            "token_endpoint_auth_methods_supported": ["none"],
                            "code_challenge_methods_supported": ["S256"],
                            "authorization_response_iss_parameter_supported": (
                                fixture.require_callback_issuer
                            ),
                        },
                    )
                    return
                self.send_json(404, {"error": "not_found"})

            def log_message(self, format, *args):  # noqa: A002, ANN001
                return

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.base_url = "http://127.0.0.1:{}".format(self.server.server_address[1])
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def browser_open(self, authorization_url):
        query = parse_qs(urlsplit(authorization_url).query)
        self.authorization_query = {key: values[0] for key, values in query.items()}
        callback_url = self.authorization_query["redirect_uri"] + "?" + urlencode(
            {
                "code": "authorization-code",
                "state": self.authorization_query["state"],
            }
        )

        def callback():
            with urlopen(callback_url, timeout=2) as response:
                response.read()

        threading.Thread(target=callback, daemon=True).start()
        return True


class MCPAuthJSONTest(unittest.TestCase):
    def test_end_to_end_dcr_pkce_and_refresh_auth_json(self):
        fixture = OAuthFixture()
        progress = []
        try:
            result = oauth.run_flow(
                oauth.FlowOptions(
                    mcp_server_url=fixture.base_url + "/mcp",
                    timeout_seconds=3,
                    http_timeout_seconds=2,
                    allow_http=True,
                ),
                browser_open=fixture.browser_open,
                progress=progress.append,
            )
        finally:
            fixture.close()

        self.assertEqual(
            result,
            {
                "type": "mcp_oauth",
                "mcp_server_url": fixture.base_url + "/mcp",
                "access_token": "access-secret",
                "refresh": {
                    "refresh_token": "refresh-secret",
                    "token_endpoint": fixture.base_url + "/token",
                    "client_id": "client-123",
                    "token_endpoint_auth": {"type": "none"},
                },
            },
        )
        self.assertEqual(fixture.registration["application_type"], "native")
        self.assertEqual(fixture.registration["token_endpoint_auth_method"], "none")
        self.assertEqual(
            fixture.registration["grant_types"], ["authorization_code", "refresh_token"]
        )
        self.assertEqual(fixture.registration["scope"], "tools.read offline_access")
        self.assertEqual(fixture.authorization_query["resource"], fixture.base_url + "/")
        self.assertEqual(fixture.authorization_query["scope"], "tools.read offline_access")
        self.assertEqual(fixture.authorization_query["code_challenge_method"], "S256")
        self.assertEqual(fixture.token_form["client_id"], "client-123")
        self.assertEqual(fixture.token_form["resource"], fixture.base_url + "/")
        self.assertTrue(fixture.token_form["code_verifier"])
        self.assertFalse(any("access-secret" in message for message in progress))
        self.assertFalse(any("refresh-secret" in message for message in progress))

    def test_no_refresh_omits_refresh_block_and_grant(self):
        fixture = OAuthFixture(include_refresh=False)
        try:
            result = oauth.run_flow(
                oauth.FlowOptions(
                    mcp_server_url=fixture.base_url + "/mcp",
                    timeout_seconds=3,
                    http_timeout_seconds=2,
                    request_refresh=False,
                    allow_http=True,
                ),
                browser_open=fixture.browser_open,
                progress=lambda _: None,
            )
        finally:
            fixture.close()

        self.assertNotIn("refresh", result)
        self.assertEqual(fixture.registration["grant_types"], ["authorization_code"])
        self.assertEqual(fixture.registration["scope"], "tools.read")
        self.assertNotIn("offline_access", fixture.registration["scope"])
        self.assertNotIn("access_type", fixture.authorization_query)

    def test_parse_bearer_challenge(self):
        headers = oauth.Message()
        headers.add_header(
            "WWW-Authenticate",
            'Bearer realm="mcp", resource_metadata="https://example.com/meta", '
            'scope="tools.read tools.write"',
        )
        metadata, scopes = oauth.parse_bearer_challenge(headers)
        self.assertEqual(metadata, "https://example.com/meta")
        self.assertEqual(scopes, ["tools.read", "tools.write"])

    def test_bearer_parser_does_not_consume_next_challenge(self):
        headers = oauth.Message()
        headers.add_header(
            "WWW-Authenticate",
            'Bearer resource_metadata="https://example.com/meta", '
            'Basic realm="login", scope="must-not-be-used"',
        )
        metadata, scopes = oauth.parse_bearer_challenge(headers)
        self.assertEqual(metadata, "https://example.com/meta")
        self.assertEqual(scopes, [])

    def test_resource_identity_accepts_same_origin_parent_and_rejects_other_origin(self):
        oauth.validate_resource_identity(
            "https://mcp.example.com/mcp", "https://mcp.example.com/"
        )
        with self.assertRaises(oauth.FlowError):
            oauth.validate_resource_identity(
                "https://mcp.example.com/mcp", "https://evil.example.com/"
            )

    def test_allow_http_is_limited_to_numeric_loopback(self):
        self.assertEqual(
            oauth.validate_url("http://127.0.0.1:8080/mcp", allow_http=True),
            "http://127.0.0.1:8080/mcp",
        )
        with self.assertRaises(oauth.FlowError):
            oauth.validate_url("http://example.com/mcp", allow_http=True)

    def test_wrong_state_callback_is_ignored(self):
        receiver = oauth.LoopbackReceiver("127.0.0.1:0")
        receiver.expect_state("expected")
        try:
            wrong = receiver.redirect_uri + "?" + urlencode({"code": "bad", "state": "wrong"})
            valid = receiver.redirect_uri + "?" + urlencode({"code": "good", "state": "expected"})

            def callbacks():
                try:
                    urlopen(wrong, timeout=2).read()
                except Exception:
                    pass
                with urlopen(valid, timeout=2) as response:
                    response.read()

            threading.Thread(target=callbacks, daemon=True).start()
            result = receiver.wait(3)
        finally:
            receiver.close()
        self.assertEqual(result["code"], "good")

    def test_required_callback_issuer_is_enforced(self):
        fixture = OAuthFixture(require_callback_issuer=True)
        try:
            with self.assertRaisesRegex(oauth.FlowError, "missing required issuer"):
                oauth.run_flow(
                    oauth.FlowOptions(
                        mcp_server_url=fixture.base_url + "/mcp",
                        timeout_seconds=3,
                        http_timeout_seconds=2,
                        allow_http=True,
                    ),
                    browser_open=fixture.browser_open,
                    progress=lambda _: None,
                )
        finally:
            fixture.close()

    def test_output_file_is_private(self):
        auth = {"type": "mcp_oauth", "access_token": "secret"}
        with tempfile.TemporaryDirectory() as directory:
            path = os.path.join(directory, "auth.json")
            oauth.write_auth_json(auth, path, pretty=True)
            mode = stat.S_IMODE(os.stat(path).st_mode)
            self.assertEqual(mode, 0o600)
            with open(path, encoding="utf-8") as handle:
                self.assertEqual(json.load(handle), auth)


if __name__ == "__main__":
    unittest.main()
