# Releasing gote

How to cut a release. Keep this file current: when a release teaches you
something (a new check, a new trap), add it here in the same PR as the fix.

## Versioning

- Tags are `vMAJOR.MINOR.PATCH` and are created from `main` only.
- Patch: bug fixes, docs, test or CI changes.
- Minor: new features. Breaking changes also ship as a minor release with a
  **Breaking changes** section in the release notes. gote stays on major
  version 1 on purpose: a `v2` tag would force the module path to
  `github.com/banorton/gote/v2`, and gote exposes no importable packages
  (everything is under `internal/`), so the version number protects no API
  consumer.
- Never move or reuse a tag after its release exists. If a release is broken,
  fix it on `main` and cut the next patch.

## 1. Land the changes

Every change reaches `main` through a pull request:

```sh
git switch -c <branch> origin/main
# commit (Conventional Commits), then
git push -u origin <branch>
gh pr create --base main --head <branch> --title "..." --body "..."
```

CI (`.github/workflows/ci.yml`) runs on the PR: Go tests on ubuntu and macos,
the race detector, the offline Linux container run, cross builds, the web
bundle check and the Playwright e2e suite. Merge only when every check is
green:

```sh
gh pr checks <number>
gh pr merge <number> --merge --delete-branch
```

Local equivalents, useful before pushing:

| Check | Command |
|---|---|
| Go tests | `CGO_ENABLED=0 go test -count=1 ./...` (`-short` skips the slow ones) |
| Committed web bundle matches source | `make web-check` |
| Browser e2e | `make e2e` |
| Cross builds | `make cross` |
| Linux, no network (Docker) | `make test-linux` |

## 2. Tag

```sh
git switch main
git fetch origin
git merge --ff-only origin/main
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"   # never tag an unsynced main
git status --short                                               # must be empty
git tag -a vX.Y.Z -m "gote vX.Y.Z"
git push origin vX.Y.Z
```

## 3. Let CI publish

Pushing a `v*` tag runs the full CI pipeline. When every test job passes, the
`release` job builds `gote-{darwin,linux}-{amd64,arm64}` with the tag as the
version, writes `SHA256SUMS`, and creates the GitHub release with generated
notes.

```sh
gh run list --branch vX.Y.Z --limit 1
gh run watch <run-id>
```

If a test job fails, no release is created. Fix the problem on `main`
through a PR, then tag the next patch version.

## 4. Verify

```sh
gh release view vX.Y.Z --json assets -q '.assets[].name'
d=$(mktemp -d)
gh release download vX.Y.Z -p "gote-$(go env GOOS)-$(go env GOARCH)" -p SHA256SUMS -D "$d"
(cd "$d" && shasum -a 256 -c --ignore-missing SHA256SUMS && chmod +x gote-* && ./gote-* --version)
```

The binary must print `gote vX.Y.Z`.

## 5. Release notes

The generated notes list merged PRs. Add a short summary at the top, and a
**Breaking changes** section whenever behavior, stored data or command output
changed incompatibly (what changed, what users must do):

```sh
gh release view vX.Y.Z --json body -q .body > notes.md
# edit notes.md
gh release edit vX.Y.Z --notes-file notes.md
```

## 6. After the release

- Install from source: `git pull && make build`. Release binaries:
  `gote update`. A running `gote serve` restarts itself on the next command
  (version handshake).
- If commands, flags, output or frontmatter changed, the docs already had to
  match before merge (the README, `skill/gote/SKILL.md` and
  `skill/template/SKILL.md` are test-checked). Agent skills built from the
  template outside this repo may need the same update.

## Traps

- `web/dist` is committed. Any change under `web/src` needs `make web`; CI
  fails if the bundle doesn't match the source.
- `skill/gote/SKILL.md` examples are compared against real output. Regenerate
  them with `skill/capture.sh <gote-binary>` (it pins `TZ=UTC` and fixture
  mtimes) and paste the changed blocks.
- Timing budgets in `e2e/` triple when `CI` is set; `make test-linux` passes
  `CI` into the container.
- Unix socket paths are limited to about 104 bytes. Tests keep state in short
  `/tmp` directories.
- A killed server can linger as a zombie in containers. Tests treat zombies as
  dead (`e2e/main_test.go`).
- `go install github.com/banorton/gote/cmd/gote@vX.Y.Z` builds report version
  `dev` (no ldflags). Release binaries and `make build` report the tag.
