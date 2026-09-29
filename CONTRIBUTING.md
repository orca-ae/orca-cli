# Contributing to ORK

Thanks for your interest in ORK (`ork`), the command-line client for Orca Agent Engine. Bug
reports, fixes, documentation, tests and feedback are all welcome.

> **Using an AI assistant?** Read the [AI policy](AI_POLICY.md) first.
> **Are you a coding agent?** Start with [AGENTS.md](AGENTS.md).

## Ways to contribute

- **Report a bug or request a feature.** Open an
  [issue](https://github.com/orca-ae/orca-cli/issues/new/choose). If the problem is in the engine
  rather than the CLI, use the
  [engine's issues](https://github.com/orca-ae/orca-agent-engine/issues/new/choose) instead.
- **Ask a question or share an idea.** Start a
  [discussion](https://github.com/orca-ae/orca-cli/discussions).
- **Fix something.** Comment on the issue to say you're working on it, so nobody duplicates your
  work.
- **Improve the docs.** If something confused you, it will confuse the next person too.

## Where to talk

| For                                                | Use                                                                                     |
| -------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Bugs and concrete feature requests                 | [Issues](https://github.com/orca-ae/orca-cli/issues)                                    |
| Questions                                          | [Discussions: Q&A](https://github.com/orca-ae/orca-cli/discussions/categories/q-a)      |
| Ideas and designs to discuss before you write code | [Discussions: Ideas](https://github.com/orca-ae/orca-cli/discussions/categories/ideas)  |
| Security vulnerabilities                           | Report privately, as described in [SECURITY.md](SECURITY.md)                            |
| Conduct concerns                                   | See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)                                            |

The project doesn't run a chat server or a mailing list, so decisions happen where everyone can
read them.

## Before you write code

- **Small, self-contained changes** can go straight to a pull request. Examples: a bug fix with a
  test, a documentation correction, a typo.
- **For anything larger**, open an issue or a discussion first. That includes a new command group,
  a refactor across packages, a change in behavior, and any change to a public contract:
  - a command's name, arguments or flags
  - the default output, the shape of `-o json` output, or exit statuses
  - an `ORCA_*` environment variable, or how credentials and the deployment URL are handled
  - the data directory, files, ports or services of `ork local`
  - the Go API of `pkg/cmd/workspace`, which other programs embed
  - the container image's user, working directory or entrypoint

  Agreeing on the approach first saves you from writing code that has to be redone.

- **Changes other people build on need an Orca Improvement Proposal (OIP).** Start with a thread in
  [Discussions → Ideas](https://github.com/orca-ae/orca-cli/discussions/categories/ideas), then open
  a pull request that adds the OIP under [`proposals/`](proposals/README.md). The code lands after
  the OIP is accepted; [`proposals/README.md`](proposals/README.md) says when one is needed.
- **Commands follow the engine's API.** Check requests and responses against the engine's
  [OpenAPI description](https://github.com/orca-ae/orca-agent-engine/blob/main/services/registry-service-ts/openapi/managed-agents.yaml),
  and keep [docs/openapi-command-gaps.md](docs/openapi-command-gaps.md) current.

## Build and test

You need Go 1.25 or later. `ork local` and one of its tests also need Docker with the Compose
plugin.

```bash
git clone https://github.com/orca-ae/orca-cli.git
cd orca-cli
go build ./cmd/ork
```

Before you open a pull request, run the checks that CI runs:

```bash
gofmt -l .                        # must print nothing; `gofmt -w .` fixes what it lists
go vet ./...
go test ./...                     # unit tests and the exhaustive command suite
scripts/check-license-headers.sh  # every source file has the license header; --fix adds it
```

`go test ./...` includes `TestEveryLeafCommandE2E` in [tests/e2e/commands](tests/e2e/commands),
which runs every leaf command against an in-process fixture. It fails when you add, remove or move
a command without adding its case.

To try a change against a real engine, run one on this machine with `go run ./cmd/ork local start`,
as the [README](README.md#quick-start) describes.

### What CI runs

- `ci.yml` runs the checks above, validates the workflows with actionlint, checks that every
  third-party dependency carries a license we can ship, and builds the CLI.
- `release-dry-run.yml` builds the release binaries and the multi-platform image without publishing
  anything, when a pull request changes the CLI or the release tooling.
- `e2e-managed-agents.yml` runs the CLI directly against a Helm-installed Managed Agents stack.
  It needs the `SNBOT_GITHUB_TOKEN` repository secret, so it runs on this repository's own branches
  and skips pull requests from forks. [tests/e2e/README.md](tests/e2e/README.md) describes the suite.

## How the code is organized

| Path                  | What it is                                                               |
| --------------------- | ------------------------------------------------------------------------ |
| `cmd/ork/`            | The `main` package                                                       |
| `pkg/cmd/root/`       | The root command and its global flags                                    |
| `pkg/cmd/workspace/`  | Every API command; other programs embed this package                     |
| `pkg/cmd/local/`      | `ork local`, a single-machine engine run with Docker Compose             |
| `pkg/mcpoauth/`       | The OAuth flow behind `ork agent vaults credentials create`              |
| `tests/e2e/`          | The exhaustive command suite and direct Managed Agents E2E              |
| `scripts/`            | Release helpers and the license-header check                             |
| `docs/`               | Notes on how the command tree maps onto the engine's API                 |

## Code style

- Go, formatted with `gofmt`. Wrap errors with `%w` and say what failed.
- Never commit secrets. Credentials come from flags or environment variables, and flag defaults for
  secrets stay empty so help text never prints them.
- Files and commit messages are public. Don't write internal hostnames, private repository names,
  customer names, personal paths, credentials or AI session links into them.

## Commits

### Sign your commits (DCO)

Every commit needs a Developer Certificate of Origin sign-off:

```bash
git commit -s -m "fix(local): parse only stdout from compose run"
```

The `-s` flag adds a line such as `Signed-off-by: Your Name <you@example.com>`. The line certifies
that you wrote the change, or otherwise have the right to submit it under the project's license. The
full text is at [developercertificate.org](https://developercertificate.org/).

If you forgot to sign off, fix the last commit with `git commit --amend -s --no-edit`, or a series
with `git rebase --signoff origin/main`, and then force-push your branch.

We don't use a CLA. The DCO sign-off is all we ask.

### Write useful messages

Use [Conventional Commits](https://www.conventionalcommits.org/): a type, an optional scope, and a
short summary in the imperative mood, such as `fix(local): parse only stdout from compose run` or
`feat(agent): stream session events`. Mark a breaking change with `!` after the type or scope, and
explain it in a `BREAKING CHANGE:` footer. In the body, explain why the change is needed and call out
any follow-up work.

Pull requests are squash-merged. The pull request title becomes the commit subject on `main`, so
title your pull request the same way. The squashed commit keeps the `Signed-off-by:` and
`Assisted-by:` trailers of the commits it replaces.

### Say when AI helped

If an AI tool helped meaningfully, add one `Assisted-by:` trailer that names the tool, such as
`Assisted-by: Claude Code`. Don't credit a tool with `Co-authored-by:`, which is for people, and
don't add session links or other trailers that a tool generates. The [AI policy](AI_POLICY.md)
explains what counts.

## Pull requests

1. Fork the repository on GitHub, and add your fork as a remote:
   `git remote add fork https://github.com/<your-username>/orca-cli.git`. Create a branch for your
   change, and push it to `fork`.
2. Keep each pull request to one logical change. Smaller pull requests get reviewed sooner.
3. Fill in the [pull request template](.github/pull_request_template.md): what changed and why,
   compatibility, how you tested it, and AI assistance.
4. Update the README in the same pull request when you change behavior.
5. Make sure CI passes.
6. A maintainer listed in [CODEOWNERS](.github/CODEOWNERS) reviews and approves the change.

If your pull request has been quiet for a while, @-mention one of the maintainers.

## Security issues

Don't report a vulnerability in a public issue, pull request or discussion. Follow
[SECURITY.md](SECURITY.md) instead.

## License

ORK is licensed under the [Apache License 2.0](LICENSE), and so is your contribution. If you copy
code from another project, keep its license header in the file and add the project to
[NOTICE](NOTICE) in the same pull request.
