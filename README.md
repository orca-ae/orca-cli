# ORK

CLI to manage & interact with resources in Orca Agent Engine

**ORK** stands for **Orca Control**: the `k` is for control, as in `kubectl`. The binary is `ork`
rather than `orca` so it never collides with GNOME's Orca screen reader, which installs
`/usr/bin/orca` on most Linux desktops.

`ork` works with the managed-agents API of an [Orca Agent Engine](https://github.com/orca-ae/orca-agent-engine)
deployment: agents and their versions, sessions and their event streams, environments, vaults and
credentials, memory stores, files, skills and triggers. It also covers the policy and pricing
extensions, and the resources that only the hosted distribution serves. `ork local` runs a complete
engine on your machine with Docker Compose.

## Install

**Homebrew** (macOS and Linux):

```bash
brew install orca-ae/tap/ork
```

**Release archives.** Each [release](https://github.com/orca-ae/orca-cli/releases) has an
`ork_<tag>_<os>_<arch>` archive for Linux, macOS and Windows on amd64 and arm64, and a
`checksums.txt`. Check an archive before you unpack it:

```bash
sha256sum --ignore-missing -c checksums.txt   # on macOS: shasum -a 256 --ignore-missing -c checksums.txt
```

Each archive also carries the license, the notice and the licenses of the third-party code
compiled into `ork`.

**Go** 1.25 or later:

```bash
go install github.com/orca-ae/orca-cli/cmd/ork@latest
```

**Container image:** `ghcr.io/orca-ae/orca-cli`. See [Container image](#container-image).

`ork --version` prints the installed version.

Maintainers publish through Release Please: merge the Release PR to trigger artifact publication,
then merge the Homebrew formula PR. See [Publishing a release](CONTRIBUTING.md#publishing-a-release).

## Quick start

Start an engine on this machine, then use it. You need Docker with Compose v2.

```bash
LOCAL_DIR="$HOME/.ork-local"
ork local --data-dir "$LOCAL_DIR" start

export ORCA_REGISTRY_URL=http://127.0.0.1:8080
export ORCA_API_KEY="$(cat "$LOCAL_DIR/secrets/workspace-api-key")"
ork agent list

ork local --data-dir "$LOCAL_DIR" stop
```

[Run a local engine](#run-a-local-engine) describes what the stack contains and how to configure
it.

## Connect to a deployment

Pass the deployment host root as `--registry-url`, with no `/v1`, `/v1/registry` or `/api/v1`
suffix: every path resolves relative to the host root. A URL that ends in one of those legacy
suffixes still works, and the CLI strips the suffix with a deprecation warning on stderr.

`ork` authenticates with one of two credentials, which are mutually exclusive:

- `--api-key`: a workspace API key, sent as `x-api-key`. A self-hosted engine, including
  `ork local`, issues these.
- `--access-token`: an OIDC access token, sent as `Authorization: Bearer`.

```bash
ork --registry-url http://localhost:8080 --api-key "$ORCA_API_KEY" agent list
ork --registry-url https://example.com --access-token "$TOKEN" agent list
```

Each flag has an environment default: `ORCA_REGISTRY_URL`, `ORCA_API_KEY` and `ORCA_ACCESS_TOKEN`.
Help text never prints the credentials.

`health`, `connections`, `sources`, `sinks`, `functions`, `kafka-connect`, `packages` and
`agent providers` manage resources that only the hosted distribution serves. They require the
deployment to advertise the hosted extension group, `cloud.sn.io`, so a self-hosted engine doesn't
have them. Run `api-groups` to see which API groups a deployment advertises, then `api-resources`
to inspect a group's resources. Discovery is authenticated like the API surfaces it describes:

```bash
ork --registry-url https://example.com --access-token "$TOKEN" api-groups
ork --registry-url https://example.com --access-token "$TOKEN" api-resources -o json
```

`ork` has no workspace context or interactive connection selection, so pass `--connection`.
`--use-connection` is reserved for programs that embed these commands and inject a selector.

## Managed agents

Create a managed-agent cloud environment with package passthrough by repeating package-manager
flags. The CLI sends these values as `config.packages`; it doesn't install packages locally.

```bash
ork --registry-url https://example.com --access-token "$TOKEN" \
  agent environments create --name claude-env \
  --scope account \
  --package-apt git --package-npm typescript --package-pip pytest
```

Create a managed-agent session with one or more vaults by repeating `--vault-id`. The CLI sends
these values as the API's `vault_ids` array.

```bash
ork --registry-url https://example.com --access-token "$TOKEN" \
  agent sessions create --environment-id env_123 --agent agent_123 \
  --vault-id vlt_123 --title "Investigation"

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent sessions update sess_123 --vault-id vlt_123 --vault-id vlt_456
```

Send typed session events and parse event streams as SSE. Cursor `0` replays the full transcript. A
nonzero cursor is the SSE `id:` field (the outer `.id` in the CLI's NDJSON), not the `evt_...` ID
inside `data`, and replay is inclusive.

```bash
ork --registry-url https://example.com --access-token "$TOKEN" \
  agent sessions events send message --session sess_123 --text "Run bash echo MARKER"

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent sessions events stream --session sess_123 --from-cursor 0 \
  --event-delta agent.message --timeout 30s
```

Memory entries, memory versions, and file content downloads are exposed directly:

```bash
ork --registry-url https://example.com --access-token "$TOKEN" \
  agent memory-stores memories create --memory-store mems_123 \
  --path notes/project.md --content "project note"

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent memory-stores memories list --memory-store mems_123 \
  --depth 1 --path-prefix notes/ --view full

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent memory-versions list --memory-store mems_123 \
  --api-key-id key_123 --operation modified --view basic

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent files content file_123 --output-file ./downloaded.txt

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent sessions files content file_456 --session sess_123 --output-file ./session-output.txt

ork --registry-url https://example.com --access-token "$TOKEN" \
  agent sessions files list --session sess_123 --after-id file_123 --limit 100
```

Agent triggers use the core `/v1/triggers` API. A self-hosted engine supports cron triggers with
`SESSION_PER_EVENT`; the hosted distribution additionally supports Kafka and Pulsar sources, more
session modes, and multiple replicas. The deployment validates unsupported combinations. Trigger
session templates use `--vault-id`, sent as `session.vault_ids`.

```bash
ork --registry-url https://example.com --access-token "$TOKEN" \
  agent triggers create --name daily-trigger --agent agent_123 \
  --source-type cron --session-mode SESSION_PER_EVENT \
  --schedule "0 9 * * *" --timezone Asia/Shanghai --payload "Create the daily report" \
  --environment-id env_123 --vault-id vlt_123
```

The hosted distribution also accepts `--source-type kafka` or `--source-type pulsar`, with
`--connection` plus `--topic` or `--topic-pattern`, and supports the `SESSION_PER_TOPIC`,
`SESSION_PER_KEY` and `SHARED` session modes.

Session outcomes and immutable skill-version bundles are available through the managed-agent
command tree:

```bash
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" agent sessions outcome session_123 -o json
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" \
  agent skills versions content skill_123 1 --output-file ./skill.zip
```

## Registry operations

Discover core API versions and call the unauthenticated core probes. `healthz` and `readyz` only
require the deployment host root:

```bash
ork --registry-url https://example.com --access-token "$TOKEN" api-versions
ork --registry-url https://example.com healthz
ork --registry-url https://example.com readyz -o json
```

Discover and use the policy and pricing extensions. Every extension command first checks the
deployment's authenticated `GET /apis` response, so an older or unsupported deployment fails with
a clear capability error instead of an endpoint 404:

```bash
ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" api-groups
ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" \
  api-resources --group policy.runorca.ai

ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" guardrails list-types -o json
ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" guardrails create \
  --config-json '{"name":"protect-production","phases":["request"],"scope":"explicit","rule":{"kind":"expression","expression":"true","on_false":"deny"}}'
ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" guardrails list --limit 100

ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" model-prices list --limit 10
ork --registry-url "$REGISTRY_URL" --api-key "$ORCA_API_KEY" \
  model-prices get model-alpha --provider provider-a -o json
```

`guardrails create` and `guardrails update` accept either `--config-json` or `--file` with the full
request object, including builtin or expression rules and nullable update fields.

### Hosted resources

These commands need a deployment that serves the hosted extension group. Probe it, and validate a
connection's configuration without creating it:

```bash
ork --registry-url https://example.com --access-token "$TOKEN" health ready
ork --registry-url https://example.com --access-token "$TOKEN" \
  connections validate --name kafka-dev --type kafka --kafka-bootstrap-servers broker:9092
```

Function, source and sink creation requires `--connection`, and connection assignment is immutable
during updates. Functions, sources and sinks accept `--sn-service-account`, and source and sink
configs also accept `--log-topic`. Lifecycle and status commands accept an optional instance ID;
functions also expose stats, trigger and state APIs.

```bash
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" functions status word-count 0
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" functions stats word-count 0 -o json
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" functions trigger word-count --data '{"value":1}' --topic input
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" functions state put word-count counter --state-json '{"numberValue":7}'
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" sources restart ingest 0
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" sinks status archive 0
```

Kafka Connect commands include worker health, registry and installed plugin catalogs, raw
config/status/task/topic views, restart options, and active-topic reset:

```bash
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" kafka-connect health
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" kafka-connect available-connectors
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" kafka-connect get status orders-sink -o json
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" kafka-connect get task-status orders-sink 0
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" kafka-connect restart connector orders-sink --include-tasks --only-failed
ork --registry-url "$REGISTRY_URL" --access-token "$TOKEN" kafka-connect reset topics orders-sink
```

The hosted Kafka Connect runtime doesn't implement the connector config `PATCH` that its OpenAPI
description declares, and plugin config validation is explicitly unsupported. Use
`kafka-connect apply` with a complete config; `--dry-run` performs local checks only.

## MCP OAuth vault credentials

The CLI can authorize an MCP HTTP server and register the resulting credential directly in a vault,
without Python or manually copying tokens:

```bash
ork --registry-url https://example.com --access-token "$TOKEN" \
  agent vaults credentials create \
  --vault vlt_123 \
  --display-name "Example MCP" \
  --mcp-server-url https://mcp.example.com/mcp \
  --output json
```

The native Go flow discovers protected-resource and authorization-server metadata, uses
Authorization Code + PKCE S256 and a random state, opens a browser, receives a loopback callback,
exchanges the code, and sends the auth object directly to the registry. Progress and the
authorization URL go to stderr; stdout contains only the credential result, not OAuth tokens.
Tokens are not cached or written to local files. The registry owns subsequent token refresh.

By default, a new client is dynamically registered on each run. The flow prefers public-client
authentication (`none`) when advertised, and otherwise supports `client_secret_basic`. For Basic
authentication, the registration response must supply a client secret; the CLI uses it for the
token exchange and includes it in the vault's refresh settings without printing or caching it.

For a pre-registered public client, pass `--oauth-client-id <id>` and configure its redirect URI as
`http://127.0.0.1:<port>/oauth/callback`, selecting that port with `--callback-address`. The
provider must accept this exact redirect URI (or explicitly permit dynamic loopback ports).
Basic client credentials are obtained through dynamic registration. Pre-registered confidential
clients, client-credentials grants, and other token endpoint authentication methods are not supported.

OAuth proxies sometimes publish one discovery location in protected-resource metadata while their
authorization-server metadata identifies an upstream issuer. The default flow rejects that
mismatch. If you have independently verified the expected issuer, pin it explicitly:

```bash
ork agent vaults credentials create \
  --vault vlt_123 --display-name "Example MCP" \
  --mcp-server-url https://mcp.example.com/mcp \
  --oauth-issuer https://identity.example/ \
  --output json
```

When exactly one authorization server is advertised, discovery still uses that location and checks
its metadata and callback against the pinned issuer. All OAuth endpoints must remain on the proxy's
origin. With multiple advertised servers, `--oauth-issuer` must select one of them; a mismatching
issuer never silently replaces the selected server.

For a headless or SSH flow, pin and forward the callback port, then open the printed URL in a local
browser:

```bash
# From the local machine, forward the callback port to the CLI host:
ssh -L 53900:127.0.0.1:53900 user@cli-host

# On the CLI host, with registry authentication configured:
ork agent vaults credentials create \
  --vault vlt_123 --display-name "Example MCP" \
  --mcp-server-url https://mcp.example.com/mcp \
  --no-browser \
  --callback-address 127.0.0.1:53900
```

Additional options:

- `--oauth-issuer <issuer>` selects an advertised authorization server or pins the expected issuer
  of a single OAuth proxy.
- `--oauth-scope "tools.read"` overrides requested scopes; repeat as needed.
- `--oauth-timeout 5m` bounds the complete OAuth flow.
- `--no-refresh` disables requesting offline access/a refresh grant. Without a returned refresh
  token, the credential will eventually require reauthorization.
- `--allow-http` permits numeric loopback HTTP endpoints for local tests only. All other OAuth
  endpoints require HTTPS; protocol redirects are not followed.

The credential includes expiry and refresh `resource`/`scope` when available. Use a registry
version supporting these fields. A successful local OAuth flow does not guarantee the registry can
reach the MCP/token endpoints: registry egress policies may prohibit private or loopback
destinations.

If vault registration fails after OAuth, tokens are not saved for retry. Check the vault first (a
lost response may hide a successful creation), then rerun if needed. The existing
`--auth-json <json>` path remains supported and is mutually exclusive with `--mcp-server-url`. The
standalone `scripts/mcp_oauth_auth_json.py` remains available as a legacy auth JSON helper; prefer
the native flow to avoid passing OAuth secrets in process arguments.

Design reference: [pi-mcp-adapter OAuth support](https://github.com/nicobailon/pi-mcp-adapter/blob/main/OAUTH.md).

## Run a local engine

`ork local` starts a single-machine engine with Docker Compose. It doesn't need a Kubernetes
cluster or a checkout of the engine repository, but it needs Docker with Compose v2 and a host
installation of `ork`: the CLI container image doesn't contain the Docker client or the host socket.

```bash
LOCAL_DIR="$HOME/.ork-local"
ork local --data-dir "$LOCAL_DIR" start
ork local --data-dir "$LOCAL_DIR" status
ork local --data-dir "$LOCAL_DIR" stop
```

The stack runs the published Registry and Harness images, Postgres for the Registry, transcript,
file and memory stores, and RustFS, an S3-compatible object store, for files. The first start
creates a local organization, a workspace and a workspace API key. `ork local start` prints the
key's path rather than its value. Without `--data-dir`, the directory is `ork/local/` under the OS
user configuration directory. `stop` keeps the Postgres and RustFS Docker volumes and the local key
files.

The default engine images are `ghcr.io/orca-ae/orca-registry-service-ts:0.5.1` (Registry and
migrations), `ghcr.io/orca-ae/orca-harness-server:0.5.1`, and
`ghcr.io/orca-ae/orca-ai-gateway:0.4.3` for the optional gateway.

The CLI doesn't save provider credentials. Set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` in the shell
before `ork local start` to pass them to Harness, and restart the stack after you change them.

This stack uses Harness's **unisolated** `in-memory` sandbox, so run only trusted agents and code.
The Registry's `local` secret store keeps vault values only in process memory, so they don't survive
a Registry restart.

`ork local start --with-gateway` also starts AI Gateway and switches Harness's default model egress
to it; without the gateway, Harness calls model providers directly. MCP servers and gateway egress
need the gateway.

`ORCA_LOCAL_REGISTRY_PORT` and `ORCA_LOCAL_ADMIN_PORT` override the loopback ports (8080 and 18082).
`ORCA_LOCAL_REGISTRY_IMAGE`, `ORCA_LOCAL_HARNESS_IMAGE` and `ORCA_LOCAL_GATEWAY_IMAGE` override the
image references, for testing another compatible release.

## Container image

Tagged releases publish a multi-platform image for `linux/amd64` and `linux/arm64`:

- `ghcr.io/orca-ae/orca-cli:vX.Y.Z`
- `ghcr.io/orca-ae/orca-cli:X.Y.Z`
- `ghcr.io/orca-ae/orca-cli:sha-<12-character-commit>`
- `ghcr.io/orca-ae/orca-cli:latest`, for stable releases only

Images carry BuildKit provenance and SBOM attestations, and are signed with keyless cosign through
GitHub OIDC.

The image runs as UID/GID `1000`, keeps `/bin/sh` so it can serve as a toolset pod, and uses
`/workspace` as its working directory. `ork` is the entrypoint:

```bash
export ORCA_REGISTRY_URL=https://example.com
read -rsp 'Orca access token: ' ORCA_ACCESS_TOKEN && echo
export ORCA_ACCESS_TOKEN

docker run --rm \
  --env ORCA_REGISTRY_URL \
  --env ORCA_ACCESS_TOKEN \
  ghcr.io/orca-ae/orca-cli:X.Y.Z agent list
```

Pass credentials at runtime, never through Docker build arguments or image layers. For Kubernetes,
source the registry credential and any provider API keys from Secrets, and avoid literal secret
values in Helm values or Pod specs:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: orca-toolset
spec:
  automountServiceAccountToken: false
  containers:
    - name: orca
      image: ghcr.io/orca-ae/orca-cli:X.Y.Z
      command: [/bin/sh, -c]
      args:
        - |
          child=
          shutdown() {
            if [ -n "$child" ]; then
              kill "$child" 2>/dev/null || true
              wait "$child" 2>/dev/null || true
            fi
            exit 0
          }
          trap shutdown TERM INT
          sleep infinity &
          child=$!
          wait "$child"
      env:
        - name: ORCA_REGISTRY_URL
          value: https://example.com
        - name: ORCA_ACCESS_TOKEN
          valueFrom:
            secretKeyRef:
              name: orca-toolset-registry
              key: access-token
      envFrom:
        # Optional provider credentials such as ANTHROPIC_API_KEY or
        # OPENAI_API_KEY for other tools executed in this pod.
        - secretRef:
            name: orca-toolset-provider-credentials
            optional: true
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop: [ALL]
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
```

Anyone allowed to `exec` into this pod can use its credentials. Restrict `pods/exec` RBAC, use
least-privilege short-lived credentials where possible, restart the pod after rotating
Secret-backed environment variables, and pin release tags or image digests instead of `latest`.

## Documentation

- The [Orca documentation](https://docs.runorca.ai) covers Orca Agent Engine and its concepts.
- [docs/openapi-command-gaps.md](docs/openapi-command-gaps.md) lists the engine API operations that
  `ork` doesn't cover yet.
- The engine's [compatibility table](https://github.com/orca-ae/orca-agent-engine/blob/main/docs/compatibility.md)
  records which `ork` release each engine release was tested with.

## Where to talk

Report bugs and request features in [Issues](https://github.com/orca-ae/orca-cli/issues), and ask
questions or share ideas in [Discussions](https://github.com/orca-ae/orca-cli/discussions).
Problems in the engine itself belong in the
[engine repository](https://github.com/orca-ae/orca-agent-engine).

## Contributing

Contributions are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) explains how to build and test, the
DCO sign-off, and how to propose changes; read the [AI policy](AI_POLICY.md) if you use AI tools.
The [E2E guide](tests/e2e/README.md) covers the exhaustive command harness and the direct
Managed Agents Helm/Kind suite, including deterministic execution, policy, and pricing scenarios.
Report security vulnerabilities privately, as [SECURITY.md](SECURITY.md) describes.

## License

ORK is licensed under the [Apache License 2.0](LICENSE).
