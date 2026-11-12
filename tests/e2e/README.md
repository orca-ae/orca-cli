# End-to-end tests

The E2E suite has two complementary layers:

- `commands/commands_test.go` executes every visible API leaf command against a deterministic HTTP
  harness, including multipart uploads, downloads, SSE, auth, discovery, and the expected error for
  the server-declared operation that is intentionally not implemented. The test derives the current
  leaf inventory from the Cobra tree and fails when a command is added, removed, or moved without
  updating its E2E case. It runs on every pull request through the normal `go test ./...` CI job and
  needs no secrets. Hosted-extension commands for connections, packages, functions, sinks, sources,
  Kafka Connect resources, and agent providers remain covered by this layer.
- `e2e-managed-agents.yml` exercises the CLI directly against a real Helm-installed Managed Agents
  stack with a workspace `x-api-key`. It validates deployment and cross-service behavior, including
  policy guardrail lifecycle and pricing reads. The proprietary hosted provider topology is not
  part of this repository's E2E suite.

Run the exhaustive command layer locally with:

```bash
go test ./tests/e2e/commands -run TestEveryLeafCommandE2E -count=1
```

`core.sh` covers extension discovery, environment and agent lifecycle, multipart file upload,
session creation, deterministic Harness execution, event polling, SSE replay, core Trigger
lifecycle and session history, Guardrail lifecycle, seeded Model Price reads, and cleanup.
Policy and pricing extension discovery is required. The suite also verifies that hosted-extension
calls fail with an extension-unavailable error when the engine does not advertise the hosted
extension group (`cloud.sn.io`).

Session cleanup uses `POST /v1/sessions/{id}/archive` and verifies that the response contains the
matching session ID and a nonempty `archived_at`. Failure cleanup also uses archive. The Session
DELETE command is covered by the deterministic HTTP harness.

The workflow inspects the `streamnative/orca-registry-service-ts` and
`streamnative/orca-harness-server` release tag pinned in `dependencies.env` to resolve their
platform-specific digests and OCI source revisions without pulling them into the runner's Docker
daemon. The paired images must carry the same 40-character source revision. That revision selects
the matching Helm Chart and deterministic fixture source, and all resolved revisions are recorded
in the GitHub Job Summary.

The workflow pulls backend images on the runner and loads them into Kind alongside the
locally built deterministic fixture. Registry and Harness are pulled by their resolved digest
and tagged with the chart's expected names; AI Gateway uses the explicit tag pinned by the chart.
The chart's three backend services use `Never` pull policy, so a missing preload fails rather
than authenticating from inside the cluster. Infrastructure images remain version-pinned and
are pulled anonymously by Kind. No in-cluster registry credential is created.

The official `charts/orca-managed-agents` Chart deploys the engine's internal Registry, Harness,
and AI Gateway. Harness uses the Chart's `in-memory` sandbox runtime and the upstream Kind fixture
as its Anthropic endpoint. The fixture returns marker-based deterministic responses and makes no
real model request. PostgreSQL is used for the transcript store to keep the CLI suite smaller than
Managed Agents' own Kafka/OpenSandbox conformance suite.

The direct Managed Agents workflow runs only on trusted `main` pushes, the nightly schedule,
and manual dispatches selecting `main`. It checks out the triggering commit SHA on an ephemeral
GitHub-hosted runner and does not restore PR-populated Go caches. The ordinary `CI` workflow
continues running unit and exhaustive mock-backed CLI command E2E tests on every PR without
secrets. Full-stack integration runs after merge.

Real-service tests isolate each lifecycle scenario. A failed scenario stops at its first failed
command, records the scenario name, and allows every remaining scenario to run. After all tests
finish, the suite prints the combined failure count and exits unsuccessfully when that count is
non-zero.

The workflow uses the configured source and image access settings. Temporary runner
authentication is cleaned up after image preparation. Docker Hub images are pulled anonymously
and remain subject to its rate limits. Repository administration is managed separately.

Test workspace keys, database credentials, object-storage credentials, model placeholder keys,
and session signing keys are local ephemeral test values. Failure diagnostics print resource
status; pod logs are included only while the repository is private. No private-source artifacts
are uploaded.

To run the scenarios against an already reachable direct deployment with the deterministic fixture
and seeded model prices:

```bash
go build -o ./ork ./cmd/ork
unset ORCA_ACCESS_TOKEN
ORCA_BIN="$PWD/ork" \
ORCA_REGISTRY_URL=http://localhost:8080 \
ORCA_API_KEY=... \
ORCA_E2E_EXPECT_EXECUTION=true \
tests/e2e/core.sh
```
