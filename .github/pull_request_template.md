<!--
Thanks for contributing! Please read CONTRIBUTING.md before opening a pull request.
Keep each pull request to one logical change, and title it the way you would write a commit
subject, for example "fix(local): parse only stdout from compose run".
-->

### What changes and why

<!-- Link the issue or discussion this addresses (for example, "Fixes #123"). Call out changes to
     docs, CI or release tooling. If this change needs an Orca Improvement Proposal (see
     proposals/README.md), link it here. -->

### Compatibility

- [ ] No change to a command's name, arguments or flags, the default or `-o json` output, exit statuses, an `ORCA_*` environment variable, the files and ports of `ork local`, the container image's user or entrypoint, or the Go API of `pkg/cmd/workspace`
- [ ] Changes one of the above. The change is described in _What changes and why_, with its OIP if one is required.

### How I tested it

<!-- The tests you added or changed, and the commands you ran. -->

### AI assistance

<!-- Required when an AI tool generated or substantially rewrote code, tests, documentation or a design in
     this pull request. Autocomplete, spelling and grammar fixes, formatting and mechanical renames don't
     count. See AI_POLICY.md. Choose one: -->

- [ ] No AI assistance
- [ ] AI-assisted. Tool(s): ___ . What it did: ___ . How I verified the result: ___ .

### Checklist

- [ ] Every commit is signed off (`git commit -s`), as described in CONTRIBUTING.md
- [ ] Commits with meaningful AI assistance carry one `Assisted-by:` trailer
- [ ] `gofmt -l .` prints nothing, and `go vet ./...`, `go test ./...` and `scripts/check-license-headers.sh` pass locally
- [ ] A new, removed or moved command has its case in `tests/e2e/commands/commands_test.go`
- [ ] The README is updated if behavior changed
