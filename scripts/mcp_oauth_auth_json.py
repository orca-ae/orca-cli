#!/usr/bin/env python3
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

"""Build MCP OAuth auth JSON for `ork agent vaults credentials create`."""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import os
import re
import secrets
import sys
import time
import webbrowser
from dataclasses import dataclass
from email.message import Message
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any, Callable, Dict, Iterable, List, Optional, Sequence, Tuple
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, parse_qsl, urlencode, urljoin, urlsplit, urlunsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener


MAX_RESPONSE_BYTES = 1024 * 1024
CALLBACK_PATH = "/oauth/callback"


class FlowError(Exception):
    pass


@dataclass
class FlowOptions:
    mcp_server_url: str
    issuer: Optional[str] = None
    scopes: Sequence[str] = ()
    callback_address: str = "127.0.0.1:0"
    timeout_seconds: float = 300.0
    http_timeout_seconds: float = 20.0
    client_name: str = "orca-cli"
    request_refresh: bool = True
    open_browser: bool = True
    allow_http: bool = False


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # noqa: ANN001
        return None


OPENER = build_opener(NoRedirect())


def validate_url(value: str, allow_http: bool) -> str:
    value = value.strip()
    try:
        parsed = urlsplit(value)
        _ = parsed.port
    except ValueError as exc:
        raise FlowError("invalid URL {!r}: {}".format(value, exc)) from exc
    schemes = {"https", "http"} if allow_http else {"https"}
    if parsed.scheme not in schemes:
        hint = "; use --allow-http only for trusted local tests" if not allow_http else ""
        raise FlowError("URL must use HTTPS{}: {}".format(hint, safe_url(value)))
    if parsed.scheme == "http" and parsed.hostname not in ("127.0.0.1", "::1"):
        raise FlowError("HTTP is allowed only for numeric loopback test servers")
    if not parsed.hostname or parsed.username is not None or parsed.password is not None:
        raise FlowError("URL must contain a host and no userinfo: {}".format(safe_url(value)))
    if parsed.fragment:
        raise FlowError("URL fragments are not allowed: {}".format(safe_url(value)))
    return value


def safe_url(value: str) -> str:
    parsed = urlsplit(value)
    return urlunsplit((parsed.scheme, parsed.netloc, parsed.path, "", ""))


def origin(value: str) -> str:
    parsed = urlsplit(value)
    return urlunsplit((parsed.scheme, parsed.netloc, "", "", ""))


def validate_resource_identity(mcp_url: str, resource: str) -> None:
    """Allow an exact MCP resource or a same-origin parent path such as origin root."""
    mcp = urlsplit(mcp_url)
    claimed = urlsplit(resource)
    mcp_port = mcp.port or (443 if mcp.scheme == "https" else 80)
    claimed_port = claimed.port or (443 if claimed.scheme == "https" else 80)
    if (mcp.scheme.lower(), mcp.hostname.lower(), mcp_port) != (
        claimed.scheme.lower(),
        claimed.hostname.lower(),
        claimed_port,
    ):
        raise FlowError("protected-resource metadata points to a different origin")
    mcp_path = mcp.path.rstrip("/") or "/"
    resource_path = claimed.path.rstrip("/") or "/"
    if claimed.query or not (
        resource_path == "/"
        or mcp_path == resource_path
        or mcp_path.startswith(resource_path + "/")
    ):
        raise FlowError("protected-resource metadata does not identify the MCP endpoint")


def fetch(
    url: str,
    timeout: float,
    allow_http: bool,
    method: str = "GET",
    headers: Optional[Dict[str, str]] = None,
    body: Optional[bytes] = None,
) -> Tuple[int, Message, bytes]:
    validate_url(url, allow_http)
    request = Request(url, data=body, headers=headers or {}, method=method)
    response = None
    try:
        response = OPENER.open(request, timeout=timeout)
    except HTTPError as exc:
        response = exc
    except (URLError, OSError) as exc:
        reason = getattr(exc, "reason", exc)
        raise FlowError("request to {} failed: {}".format(safe_url(url), reason)) from exc
    try:
        data = response.read(MAX_RESPONSE_BYTES + 1)
        if len(data) > MAX_RESPONSE_BYTES:
            raise FlowError("response from {} exceeds 1 MiB".format(safe_url(url)))
        return int(response.status), response.headers, data
    finally:
        response.close()


def json_object(body: bytes, source: str) -> Dict[str, Any]:
    try:
        value = json.loads(body.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise FlowError("{} returned invalid JSON".format(source)) from exc
    if not isinstance(value, dict):
        raise FlowError("{} returned non-object JSON".format(source))
    return value


def http_error(source: str, status: int, body: bytes) -> FlowError:
    detail = ""
    try:
        payload = json.loads(body.decode("utf-8"))
        if isinstance(payload, dict):
            parts = [
                value
                for value in (payload.get("error"), payload.get("error_description"))
                if isinstance(value, str)
            ]
            if parts:
                detail = ": " + " - ".join(parts)[:400]
    except (UnicodeDecodeError, json.JSONDecodeError):
        pass
    return FlowError("{} failed with HTTP {}{}".format(source, status, detail))


AUTH_PARAM = re.compile(
    r"(?i)(resource_metadata|scope)\s*=\s*(?:\"((?:\\.|[^\"])*)\"|([^,\s]+))"
)


def parse_bearer_challenge(headers: Message) -> Tuple[Optional[str], List[str]]:
    metadata = None
    scopes: List[str] = []
    for value in headers.get_all("WWW-Authenticate", []):
        match = re.match(r"(?i)^\s*Bearer(?:\s+|$)", value)
        if not match:
            continue
        parameters = value[match.end():]
        next_challenge = re.search(
            r",\s*[A-Za-z][A-Za-z0-9_.-]*\s+(?=[A-Za-z][A-Za-z0-9_.-]*\s*=)",
            parameters,
        )
        if next_challenge:
            parameters = parameters[: next_challenge.start()]
        for parameter in AUTH_PARAM.finditer(parameters):
            name = parameter.group(1).lower()
            raw = parameter.group(2) if parameter.group(2) is not None else parameter.group(3)
            raw = re.sub(r"\\(.)", r"\1", raw)
            if name == "resource_metadata" and metadata is None:
                metadata = raw
            elif name == "scope":
                scopes.extend(raw.split())
    return metadata, dedupe(scopes)


def probe_mcp(options: FlowOptions) -> Tuple[Optional[str], List[str]]:
    payload = json.dumps(
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2025-06-18",
                "capabilities": {},
                "clientInfo": {"name": "orca-cli-oauth", "version": "1"},
            },
        },
        separators=(",", ":"),
    ).encode("utf-8")
    _, headers, _ = fetch(
        options.mcp_server_url,
        options.http_timeout_seconds,
        options.allow_http,
        method="POST",
        headers={
            "Accept": "application/json, text/event-stream",
            "Content-Type": "application/json",
        },
        body=payload,
    )
    return parse_bearer_challenge(headers)


def prm_candidates(mcp_url: str, advertised: Optional[str]) -> List[str]:
    parsed = urlsplit(mcp_url)
    candidates = []
    if advertised:
        candidates.append(urljoin(mcp_url, advertised))
    path = parsed.path.rstrip("/")
    if path:
        candidates.append(origin(mcp_url) + "/.well-known/oauth-protected-resource" + path)
    candidates.append(origin(mcp_url) + "/.well-known/oauth-protected-resource")
    return dedupe(candidates)


def discover(options: FlowOptions) -> Dict[str, Any]:
    advertised_prm, challenge_scopes = probe_mcp(options)
    prm: Dict[str, Any] = {}
    for candidate in prm_candidates(options.mcp_server_url, advertised_prm):
        try:
            status, _, body = fetch(
                candidate,
                options.http_timeout_seconds,
                options.allow_http,
                headers={"Accept": "application/json"},
            )
            if 200 <= status < 300:
                prm = json_object(body, "protected-resource metadata")
                break
        except FlowError:
            continue

    resource = prm.get("resource", options.mcp_server_url)
    servers = prm.get("authorization_servers", [])
    prm_scopes = prm.get("scopes_supported", [])
    if not isinstance(resource, str):
        raise FlowError("protected-resource metadata resource must be a string")
    if not isinstance(servers, list) or not all(isinstance(item, str) for item in servers):
        raise FlowError("protected-resource metadata authorization_servers must be strings")
    if not isinstance(prm_scopes, list) or not all(isinstance(item, str) for item in prm_scopes):
        prm_scopes = []
    validate_url(resource, options.allow_http)
    validate_resource_identity(options.mcp_server_url, resource)
    for server in servers:
        validate_url(server, options.allow_http)

    if options.issuer:
        if servers and options.issuer not in servers:
            raise FlowError("--issuer must match one of: {}".format(", ".join(servers)))
        issuer = options.issuer
    elif len(servers) > 1:
        raise FlowError("multiple authorization servers; rerun with --issuer: {}".format(", ".join(servers)))
    else:
        issuer = servers[0] if servers else origin(options.mcp_server_url)
    validate_url(issuer, options.allow_http)

    parsed_issuer = urlsplit(issuer)
    issuer_path = parsed_issuer.path.rstrip("/")
    issuer_origin = origin(issuer)
    metadata_candidates = [
        issuer_origin + "/.well-known/oauth-authorization-server" + issuer_path,
        issuer_origin + "/.well-known/openid-configuration" + issuer_path,
    ]
    if issuer_path:
        metadata_candidates.extend(
            [
                issuer.rstrip("/") + "/.well-known/oauth-authorization-server",
                issuer.rstrip("/") + "/.well-known/openid-configuration",
            ]
        )

    metadata = None
    for candidate in dedupe(metadata_candidates):
        try:
            status, _, body = fetch(
                candidate,
                options.http_timeout_seconds,
                options.allow_http,
                headers={"Accept": "application/json"},
            )
            if 200 <= status < 300:
                metadata = json_object(body, "authorization-server metadata")
                break
        except FlowError:
            continue
    if metadata is None:
        raise FlowError("could not discover authorization-server metadata")

    metadata_issuer = metadata.get("issuer", issuer)
    if not isinstance(metadata_issuer, str) or metadata_issuer != issuer:
        raise FlowError("authorization-server metadata issuer mismatch")
    required = ("authorization_endpoint", "token_endpoint", "registration_endpoint")
    if not all(isinstance(metadata.get(name), str) and metadata[name] for name in required):
        raise FlowError("authorization-server metadata is missing required endpoints")
    methods = metadata.get("code_challenge_methods_supported", [])
    if not isinstance(methods, list) or "S256" not in methods:
        raise FlowError("authorization server does not advertise PKCE S256")
    auth_methods = metadata.get("token_endpoint_auth_methods_supported", [])
    if isinstance(auth_methods, list) and auth_methods and "none" not in auth_methods:
        raise FlowError("authorization server does not support public OAuth clients")
    server_scopes = metadata.get("scopes_supported", [])
    if not isinstance(server_scopes, list) or not all(isinstance(item, str) for item in server_scopes):
        server_scopes = []
    for name in required:
        validate_url(metadata[name], options.allow_http)

    explicit = split_scopes(options.scopes)
    scopes = explicit or challenge_scopes or list(prm_scopes)
    if options.request_refresh and "offline_access" in server_scopes:
        scopes.append("offline_access")

    return {
        "resource": resource,
        "issuer": metadata_issuer,
        "authorization_endpoint": metadata["authorization_endpoint"],
        "token_endpoint": metadata["token_endpoint"],
        "registration_endpoint": metadata["registration_endpoint"],
        "scopes": dedupe(scopes),
        "authorization_response_iss_parameter_supported": bool(
            metadata.get("authorization_response_iss_parameter_supported", False)
        ),
    }


class LoopbackReceiver:
    def __init__(self, address: str):
        if not address.startswith("127.0.0.1:"):
            raise FlowError("--callback-address must use 127.0.0.1:<port>")
        try:
            port = int(address.rsplit(":", 1)[1])
        except ValueError as exc:
            raise FlowError("invalid callback port") from exc
        if port < 0 or port > 65535:
            raise FlowError("invalid callback port")
        self.result: Optional[Dict[str, str]] = None
        self.expected_state: Optional[str] = None
        receiver = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):  # noqa: N802
                parsed = urlsplit(self.path)
                if parsed.path != CALLBACK_PATH:
                    self.send_response(404)
                    self.end_headers()
                    return
                result = {
                    key: values[0]
                    for key, values in parse_qs(parsed.query, keep_blank_values=True).items()
                    if values
                }
                if not receiver.expected_state or result.get("state") != receiver.expected_state:
                    body = b"Invalid OAuth state. Return to the terminal and retry."
                    self.send_response(400)
                    self.send_header("Content-Type", "text/plain; charset=utf-8")
                    self.send_header("Content-Length", str(len(body)))
                    self.send_header("Cache-Control", "no-store")
                    self.end_headers()
                    self.wfile.write(body)
                    return
                receiver.result = result
                body = b"OAuth authorization received. Return to the terminal."
                self.send_response(200)
                self.send_header("Content-Type", "text/plain; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, format, *args):  # noqa: A002, ANN001
                return

        try:
            self.server = HTTPServer(("127.0.0.1", port), Handler)
        except OSError as exc:
            raise FlowError("failed to bind OAuth callback: {}".format(exc)) from exc
        self.redirect_uri = "http://127.0.0.1:{}{}".format(
            self.server.server_address[1], CALLBACK_PATH
        )

    def wait(self, timeout: float) -> Dict[str, str]:
        deadline = time.monotonic() + timeout
        while self.result is None:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise FlowError("timed out waiting for OAuth callback")
            self.server.timeout = min(0.5, remaining)
            self.server.handle_request()
        return self.result

    def expect_state(self, state: str) -> None:
        self.expected_state = state

    def close(self) -> None:
        self.server.server_close()


def register_client(options: FlowOptions, config: Dict[str, Any], redirect_uri: str) -> str:
    grants = ["authorization_code"]
    if options.request_refresh:
        grants.append("refresh_token")
    payload: Dict[str, Any] = {
        "client_name": options.client_name,
        "application_type": "native",
        "redirect_uris": [redirect_uri],
        "grant_types": grants,
        "response_types": ["code"],
        "token_endpoint_auth_method": "none",
    }
    if config["scopes"]:
        payload["scope"] = " ".join(config["scopes"])
    status, _, body = fetch(
        config["registration_endpoint"],
        options.http_timeout_seconds,
        options.allow_http,
        method="POST",
        headers={"Accept": "application/json", "Content-Type": "application/json"},
        body=json.dumps(payload, separators=(",", ":")).encode("utf-8"),
    )
    if status not in (200, 201):
        raise http_error("dynamic client registration", status, body)
    response = json_object(body, "dynamic client registration")
    client_id = response.get("client_id")
    if not isinstance(client_id, str) or not client_id:
        raise FlowError("dynamic client registration response is missing client_id")
    if response.get("token_endpoint_auth_method", "none") != "none":
        raise FlowError("dynamic client registration did not create a public client")
    return client_id


def build_authorization_url(
    options: FlowOptions,
    config: Dict[str, Any],
    client_id: str,
    redirect_uri: str,
    state: str,
    challenge: str,
) -> str:
    parsed = urlsplit(config["authorization_endpoint"])
    query = parse_qsl(parsed.query, keep_blank_values=True)
    query.extend(
        [
            ("response_type", "code"),
            ("client_id", client_id),
            ("redirect_uri", redirect_uri),
            ("state", state),
            ("code_challenge", challenge),
            ("code_challenge_method", "S256"),
            ("resource", config["resource"]),
        ]
    )
    if config["scopes"]:
        query.append(("scope", " ".join(config["scopes"])))
    if options.request_refresh:
        query.append(("access_type", "offline"))
    return urlunsplit((parsed.scheme, parsed.netloc, parsed.path, urlencode(query), ""))


def exchange_code(
    options: FlowOptions,
    config: Dict[str, Any],
    client_id: str,
    redirect_uri: str,
    code: str,
    verifier: str,
) -> Dict[str, Any]:
    form = urlencode(
        {
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": redirect_uri,
            "client_id": client_id,
            "code_verifier": verifier,
            "resource": config["resource"],
        }
    ).encode("ascii")
    status, _, body = fetch(
        config["token_endpoint"],
        options.http_timeout_seconds,
        options.allow_http,
        method="POST",
        headers={
            "Accept": "application/json",
            "Content-Type": "application/x-www-form-urlencoded",
        },
        body=form,
    )
    if not 200 <= status < 300:
        raise http_error("token exchange", status, body)
    tokens = json_object(body, "token exchange")
    if not isinstance(tokens.get("access_token"), str) or not tokens["access_token"]:
        raise FlowError("token response is missing access_token")
    return tokens


def run_flow(
    options: FlowOptions,
    browser_open: Callable[[str], bool] = webbrowser.open,
    progress: Callable[[str], None] = lambda message: print(message, file=sys.stderr),
) -> Dict[str, Any]:
    options.mcp_server_url = validate_url(options.mcp_server_url, options.allow_http)
    progress("Discovering OAuth metadata for {}".format(safe_url(options.mcp_server_url)))
    config = discover(options)
    receiver = LoopbackReceiver(options.callback_address)
    try:
        progress("Registering public OAuth client")
        client_id = register_client(options, config, receiver.redirect_uri)
        verifier = secrets.token_urlsafe(32)
        challenge = base64.urlsafe_b64encode(
            hashlib.sha256(verifier.encode("ascii")).digest()
        ).rstrip(b"=").decode("ascii")
        state = secrets.token_urlsafe(24)
        receiver.expect_state(state)
        authorization_url = build_authorization_url(
            options, config, client_id, receiver.redirect_uri, state, challenge
        )
        progress("Open this URL to authorize:\n{}".format(authorization_url))
        if options.open_browser:
            try:
                opened = browser_open(authorization_url)
            except Exception as exc:
                opened = False
                progress("Browser launch failed: {}".format(exc))
            if not opened:
                progress("Browser was not opened automatically; use the URL above.")
        progress("Waiting for callback on {}".format(receiver.redirect_uri))
        callback = receiver.wait(options.timeout_seconds)
    finally:
        receiver.close()

    if callback.get("error"):
        detail = callback.get("error_description")
        raise FlowError(
            "authorization failed: {}{}".format(
                callback["error"], ": " + detail if detail else ""
            )
        )
    if callback.get("iss") and callback["iss"] != config["issuer"]:
        raise FlowError("OAuth callback issuer mismatch")
    if config["authorization_response_iss_parameter_supported"] and not callback.get("iss"):
        raise FlowError("OAuth callback is missing required issuer parameter")
    code = callback.get("code")
    if not code:
        raise FlowError("OAuth callback is missing authorization code")

    progress("Exchanging authorization code for tokens")
    tokens = exchange_code(
        options, config, client_id, receiver.redirect_uri, code, verifier
    )
    auth: Dict[str, Any] = {
        "type": "mcp_oauth",
        "mcp_server_url": options.mcp_server_url,
        "access_token": tokens["access_token"],
    }
    refresh_token = tokens.get("refresh_token")
    if isinstance(refresh_token, str) and refresh_token:
        auth["refresh"] = {
            "refresh_token": refresh_token,
            "token_endpoint": config["token_endpoint"],
            "client_id": client_id,
            "token_endpoint_auth": {"type": "none"},
        }
    else:
        progress("Warning: no refresh token returned; reauthorization will be required.")
    return auth


def split_scopes(values: Iterable[str]) -> List[str]:
    return dedupe(scope for value in values for scope in value.split())


def dedupe(values: Iterable[str]) -> List[str]:
    result = []
    seen = set()
    for value in values:
        if value and value not in seen:
            result.append(value)
            seen.add(value)
    return result


def write_auth_json(auth: Dict[str, Any], output: str, pretty: bool) -> None:
    content = json.dumps(
        auth,
        indent=2 if pretty else None,
        separators=None if pretty else (",", ":"),
        sort_keys=pretty,
    ) + "\n"
    if output == "-":
        sys.stdout.write(content)
        return
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            fd = -1
            handle.write(content)
    finally:
        if fd >= 0:
            os.close(fd)


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(
        description="Build MCP OAuth auth JSON for an Orca/Anthropic vault credential."
    )
    result.add_argument("mcp_server_url", help="Remote MCP Streamable HTTP endpoint")
    result.add_argument("--issuer", help="Authorization-server issuer when multiple exist")
    result.add_argument("--scope", action="append", default=[], help="OAuth scope override")
    result.add_argument(
        "--callback-address",
        default="127.0.0.1:0",
        help="Loopback callback address (default: 127.0.0.1:0)",
    )
    result.add_argument("--timeout", type=float, default=300, help="Callback timeout seconds")
    result.add_argument("--http-timeout", type=float, default=20, help="HTTP timeout seconds")
    result.add_argument("--client-name", default="orca-cli", help="DCR client name")
    result.add_argument("--no-browser", action="store_true", help="Do not open browser")
    result.add_argument("--no-refresh", action="store_true", help="Do not request refresh grant")
    result.add_argument(
        "--allow-http", action="store_true", help="Allow HTTP for trusted local tests"
    )
    result.add_argument("-o", "--output", default="-", help="Output file or '-' for stdout")
    result.add_argument("--pretty", action="store_true", help="Pretty-print JSON")
    return result


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = parser().parse_args(argv)
    options = FlowOptions(
        mcp_server_url=args.mcp_server_url,
        issuer=args.issuer,
        scopes=args.scope,
        callback_address=args.callback_address,
        timeout_seconds=args.timeout,
        http_timeout_seconds=args.http_timeout,
        client_name=args.client_name,
        request_refresh=not args.no_refresh,
        open_browser=not args.no_browser,
        allow_http=args.allow_http,
    )
    try:
        write_auth_json(run_flow(options), args.output, args.pretty)
        return 0
    except FlowError as exc:
        print("error: {}".format(exc), file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("error: interrupted", file=sys.stderr)
        return 130
    except OSError as exc:
        print("error: {}".format(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
