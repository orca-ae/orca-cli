# Orca Improvement Proposals (OIPs)

An OIP is the design record for a change that other people build on or operate against. ORK follows
the [OIP process of Orca Agent Engine](https://github.com/orca-ae/orca-agent-engine/blob/main/proposals/README.md).
Proposals for the CLI live in this directory and are numbered separately from the engine's.

## When you need an OIP

You need an accepted OIP before the code lands for:

- **A breaking change to the command line.** Removing or renaming a command or flag, changing what a
  flag means, or changing the default output, the shape of `-o json` output, or exit statuses.
- **A change to configuration.** `ORCA_*` environment variables, how credentials are sent, or how
  the deployment URL is resolved.
- **A change to what `ork local` sets up.** Its data directory, the files it writes, its default
  ports, or the services it runs.
- **A breaking change to the Go API of `pkg/cmd/workspace`**, which other programs embed.
- **A new distribution channel,** such as another package manager or image registry.

You don't need an OIP for a new command that mirrors an engine API, a bug fix, refactoring, tests or
documentation. If you're not sure, ask in
[Discussions](https://github.com/orca-ae/orca-cli/discussions/categories/ideas).

## How it works

1. **Start a discussion** under Ideas that describes the problem.
2. **Write the OIP.** Copy the engine's
   [`TEMPLATE.md`](https://github.com/orca-ae/orca-agent-engine/blob/main/proposals/TEMPLATE.md) to
   `proposals/OIP-NNN-short-title.md` here, using the highest existing number plus one, and add a row
   to the index below.
3. **Open a pull request** that adds the file with the status *Proposed*, and link it from the
   discussion. The review happens on the pull request, which carries the `oip` label.
4. **Acceptance.** An OIP is accepted with approvals from two maintainers and no unresolved objection
   from a maintainer. Merging the pull request accepts it.
5. **Keep the status current.** Implementation pull requests link the OIP, and update its status and
   its row in the index as the work lands. The statuses are those of the engine's process.

## Index

| OIP | Title | Status |
| --- | ----- | ------ |
