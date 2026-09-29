# AGENTS.md

Instructions for coding agents working in this repository. [CONTRIBUTING.md](CONTRIBUTING.md) is the
guide for people; these rules add what an agent can't infer from the code.

## What this is

`ork` is the command-line client for Orca Agent Engine. It's the Go module
`github.com/orca-ae/orca-cli`, built on [cobra](https://github.com/spf13/cobra) and the Go SDK
`github.com/orca-ae/orca-sdk-go`.

| Path                  | What it is                                                                  |
| --------------------- | --------------------------------------------------------------------------- |
| `cmd/ork/`            | The `main` package                                                          |
| `pkg/cmd/root/`       | The root command, its global flags and the list of top-level commands       |
| `pkg/cmd/workspace/`  | Every API command. Other programs embed this package                        |
| `pkg/cmd/local/`      | `ork local`: a single-machine engine run with Docker Compose from `assets/` |
| `pkg/mcpoauth/`       | The OAuth flow behind `ork agent vaults credentials create`                 |
| `tests/e2e/`          | The in-process command suite (`commands/`) and direct Managed Agents E2E  |
| `scripts/`            | Release helpers and the license-header check                                |

## Build and test

```bash
go build ./cmd/ork
gofmt -l .                        # must print nothing
go vet ./...
go test ./...
scripts/check-license-headers.sh  # --fix adds missing headers
```

## Rules

- **Every leaf command has an E2E case.** `TestEveryLeafCommandE2E` reads the command inventory from
  the cobra tree. It fails when a command is added, removed or moved without a matching case in
  `tests/e2e/commands/commands_test.go`.
- **`pkg/cmd/workspace` is a library.** Other programs build their own command trees from
  `NewGroupCommand`, the `NewCmd*` constructors, `Options` and `IOStreams`. Keep those signatures
  compatible, and put standalone-only behavior in `pkg/cmd/root` or `cmd/ork`.
- **The SDK is imported as `registry`:** `registry "github.com/orca-ae/orca-sdk-go"`. Keep the
  alias; the command files are written against it.
- **Two credentials, never both.** `--access-token` (`ORCA_ACCESS_TOKEN`) sends
  `Authorization: Bearer`, and `--api-key` (`ORCA_API_KEY`) sends `x-api-key`. The flags are
  mutually exclusive, and the server treats `x-api-key` as authoritative whenever it's present.
- **The deployment URL is the host root.** `--registry-url` (`ORCA_REGISTRY_URL`) takes no `/v1`,
  `/v1/registry` or `/api/v1` suffix; the CLI strips those with a deprecation warning.
- **Hosted-only commands check discovery first.** `health`, `connections`, `sources`, `sinks`,
  `functions`, `kafka-connect`, `packages` and `agent providers` exist only on the hosted
  distribution. They run only when `GET /apis` lists the hosted extension group, whose wire name is
  `cloud.sn.io`. In prose, call it "the hosted extension group".
- **Secrets stay out of help text and output.** Flag defaults for credentials stay empty, so cobra
  never prints an environment value. `ork local` prints the path of a generated key, never the key.
- **Every source file starts with the license header** in its own comment syntax, after any shebang:
  `Copyright The Orca Authors`, then `SPDX-License-Identifier: Apache-2.0`.
- **Files and commit messages are public.** Don't write internal hostnames, private repository
  names, customer names, personal paths, credentials or AI session links into them. The
  real-service E2E suites may name the repositories, images and endpoints they test against.

## Commits and outward actions

- Use Conventional Commit subjects, for example `fix(local): parse only stdout from compose run`.
- Add one `Assisted-by:` trailer when you helped. `.claude/settings.json` adds it for Claude Code.
- Never add `Signed-off-by`: only a human signs off. Don't credit a tool with `Co-authored-by`, and
  don't add session links.
- Don't push, open pull requests or issues, or post comments unless a human approved that specific
  action. [AI_POLICY.md](AI_POLICY.md) explains why.
