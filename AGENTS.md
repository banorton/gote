# Working on gote

gote is a single-binary, CGO-free Go CLI for a directory of Markdown notes:
SQLite index, background server, fuzzy/full-text/semantic search, link graph,
native Excalidraw drawings with an embedded local-only web editor. User docs:
`README.md`, `docs/config.md`, `docs/embedding.md`.

## Releasing

Follow `docs/releasing.md`. Update it when a release teaches you something.

## Layout

| Path | What |
|---|---|
| `cmd/gote` | entrypoint; `imports.go` blank-imports every package that self-registers |
| `internal/cli/<area>` | commands; each registers itself with `cli.Register` in `init()` |
| `internal/ops/<area>` | operations; each registers with `ops.Register`/`ops.Handle`; JSON in, JSON out |
| `internal/server`, `internal/client` | Unix-socket HTTP server and the local/remote clients |
| `internal/index`, `internal/store`, `internal/parse` | scanning, SQLite schema, Markdown/frontmatter parsing |
| `internal/excal` | Excalidraw parsing, checks, PNG renderer |
| `internal/web`, `web/` | local web view and editor; `web/dist` is committed |
| `e2e/` | real-binary tests; `web/e2e/` Playwright tests |
| `skill/` | agent skill (`gote/`) and template (`template/`), both test-checked |

## Rules

- `CGO_ENABLED=0` must always build; macOS and Linux only.
- Ops never touch the terminal, editor or browser; that belongs in `internal/cli`.
  Every op must behave the same in-process and through the server.
- Write the failing test first. Tests use temp `XDG_CONFIG_HOME`/`XDG_STATE_HOME`
  with short paths, `server: false` unless they start their own server, and
  must leave no `gote serve` process behind.
- Never touch a real user's config, state or notes from tests or scripts.
- Changing commands, flags, output or frontmatter means updating `README.md`,
  `skill/gote/SKILL.md` (regenerate examples with `skill/capture.sh`) and the
  core section of `skill/template/SKILL.md`; tests fail otherwise.
- Changes to `web/src` need `make web` so the committed bundle matches.
- Conventional Commits. Changes reach `main` only through a PR with green CI.

## Commands

| Task | Command |
|---|---|
| Build | `make build` |
| Go tests | `CGO_ENABLED=0 go test -count=1 ./...` |
| Web bundle | `make web` / `make web-check` |
| Browser e2e | `make e2e` |
| Cross builds | `make cross` |
| Linux, no network | `make test-linux` |
