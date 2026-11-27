# gote

A fast note manager for a directory of Markdown files. One static binary, a SQLite index, full-text and optional semantic search, a link graph, Excalidraw drawings, and a JSON interface for agents.

- Notes are plain `.md` files in one directory. The index is a cache; delete it and gote rebuilds it.
- gote never rewrites a file implicitly. Only explicit commands write.
- macOS and Linux, amd64 and arm64, no CGO.

## Install

```
go install github.com/banorton/gote/cmd/gote@latest
```

Or build from a checkout with `make build`.

## Configure

Create `~/.config/gote/config.yaml` (or `$XDG_CONFIG_HOME/gote/config.yaml`):

```yaml
notes_dir: ~/notes
```

Only `notes_dir` is required. Every key is documented in [docs/config.md](docs/config.md). State (index, socket, log) lives in `~/.local/state/gote/`. Templates live in `~/.config/gote/templates/<name>.md`.

## Use

```
gote                      # open the quick note
gote my idea              # open "my idea.md", creating it if missing
gote new my idea          # always create (fails if it exists)
gote new my idea --json   # create without opening the editor: {id,path,created}
gote -                    # reopen the last note
gote search kube          # fuzzy match on titles, paths, aliases
gote search -t kubernetes # full-text search (body, tags, filters)
gote search               # fuzzy finder (ctrl-t: search note contents)
gote tasks                # open "- [ ]" tasks across all notes
gote graph my-idea -d 2   # link neighborhood as a tree
```

Every command takes `--json` for machine-readable output. Exit codes: 0 ok, 1 error, 2 usage. `-` works as `<note>` in any command and means the last opened note. A `<note>` is an id (`projects/gote.md`), a filename stem, an alias, or a path suffix.

A note's id is its path relative to the notes directory. Its title is the frontmatter `title:`, else the filename. Files and directories starting with `.` are ignored (`.trash/`, `.git/`, `.quick.md`).

### Commands

| Command | Aliases | What it does |
|---|---|---|
| `open` | | open a note, creating it if missing (`gote <name>`); with `--json` or piped stdout it skips the editor and prints `{id,path,created}` or the path. Flags: `-t <template>`, `-d` (date prefix), `-dt` (datetime prefix), `-nt` (no prefix) |
| `new` | | create a note and open it; fails if it exists. With `--json` or piped stdout it skips the editor |
| `quick` | `q` | open the quick note (`.quick.md`); `quick save <name>` turns it into a note |
| `quick-save` | `qs` | save the quick note as a note |
| `daily` | `today` | open today's daily note (`daily/<yymmdd>_daily.md`), creating it if missing: frontmatter only (`created`, `tags: [daily]`, `aliases`), empty body, or your `daily` template. `daily -N` (N >= 1) opens the existing note from N days ago; if there is none it prints `no daily note for YYYY-MM-DD` and exits 1 (past days are never created). With `--json` or piped stdout it skips the editor and prints `{id,path,created}` |
| `archive` | | `archive <note>...` adds the `archive` tag (`--json` lists the changed ids) |
| `unarchive` | | `unarchive <note>...` removes it |
| `recent` | `r` | menu of recently opened notes (`--all` includes archived) |
| `get` | `g` | menu of all notes (`--all` includes archived) |
| `pinned` | `p` | menu of pinned notes (`--all` includes archived); with a note name, pins it |
| `pin` | | pin a note |
| `unpin` | `u`, `up` | unpin a note |
| `tag` | `t` | no args: tags with counts. `tag [--all] <tag>...`: menu of notes with all tags. `tag add <tag> <note>...`, `tag rm <tag> <note>...`; `--query <q>` picks targets |
| `delete` | `d`, `del` | move a note to `.trash/` |
| `trash` | | list the trash; `trash empty` empties it |
| `recover` | | restore a note from the trash |
| `rename` | `mv`, `rn` | rename a note and rewrite every inbound link: `rename <note> -n <new>` |
| `duplicate` | `dup`, `cp` | copy a note |
| `info` | `i` | note details: tags, aliases, link counts, size |
| `template` | `tmpl` | `list`, `open <name>`, `rename <name> <new>`, `delete <name>` |
| `search` | `s` | fuzzy search of titles, paths and aliases; no args opens the fuzzy finder. filters like `tag:x` narrow first; `-t` full-text, `-s` semantic, `-n <limit>`, `--all` includes archived, `-l`/`--list` plain list instead of the menu |
| `tasks` | | list open tasks; `--all` includes done tasks and archived notes; a query restricts to matching notes |
| `links` | | outgoing links of a note |
| `backlinks` | | notes linking to a note |
| `graph` | | link neighborhood as a tree: `graph <note> [-d N]` (depth 1-5) |
| `orphans` | | notes with no links in or out (`--all` includes archived) |
| `broken` | | links to missing notes or files (links to attachments that exist are fine) |
| `diagram` | | Excalidraw drawings: `diagram [name]` creates and opens the editor, `diagram open <ref>`, `diagram list [-n N] [--all] [--json]`, `diagram read <ref>`, `diagram check <ref>`, `diagram render <ref> [--json]`, `diagram describe <ref> [<text>...\|-\|--clear]`, `diagram search [-n N] [--all] [-l] [-t\|-s] [<words>...]` |
| `view` | `v` | open a rendered note in the browser; `view --terminal <note>` (`-T`) renders in the terminal |
| `embed` | | compute embeddings for semantic search; `embed status` |
| `config` | `c` | show the config file |
| `config-edit` | `ce` | edit the config file |
| `export` | `exp` | write notes, templates and pins to a tar.gz (default `gote-export.tar.gz`) |
| `import` | `imp` | restore an export into an empty notes directory (`--force` to overwrite) |
| `status` | | index counts |
| `reindex` | `idx` | drop and rebuild the index |
| `serve` | | run the server in the foreground (normally auto-spawned) |
| `server` | | `server status`, `server stop` |
| `help` | `h`, `man` | list commands or show help for one |
| `version` | | print the version |

### Menu shortcuts

A lister letter plus an action letter opens that menu with the action preselected.

| Lister | `o` open | `d` delete | `p` pin | `v` view | `r` rename | `u` unpin |
|---|---|---|---|---|---|---|
| recent | `ro` | `rd` | `rp` | `rv` | `rr` | |
| search (rest of args = fuzzy words) | `so` | `sd` | `sp` | `sv` | `sr` | |
| tag (rest of args = tags) | `to` | `td` | `tp` | `tv` | `tr` | |
| pinned | `po` | `pd` | | `pv` | `pr` | `pu` |

In a menu, items are labeled with the home row (`a s d f g h j k l ;`); `n`/`p` page; Shift-`O D P V R U` or Tab switch the action; `q` or esc quits. Without a terminal, list commands print `id<TAB>title` lines.

### Search

`gote search <words>` fuzzy-matches (fzf scoring) the same text the finder sees: title, path, aliases. Best match first; on a terminal the results open in the home-row menu below, otherwise (or with `-l`/`--list`) they print as `title  (id)`, or `{results:[{id,title,kind,path,score}]}` with `--json`. It never reads note bodies.

Filter tokens work in fuzzy mode: `gote search tag:thread api` keeps the notes tagged `thread`, then fuzzy-matches `api` on titles, paths and aliases. Supported: `tag:`, `field:value`, `kind:`, `created:`, `modified:` and `-token` negation. A quoted phrase has no fuzzy meaning, so it is a literal, case-insensitive substring filter on the title (`"api gat"`); a negated word or phrase (`-gateway`) excludes titles containing it. With only filters (`gote search tag:thread`) there are no words to match: on a terminal it opens the finder over just the matching notes, otherwise (or with `--list`/`--json`) it lists them, most recently modified first. `gote diagram search` works the same way over drawings; with no arguments it opens the finder over all drawings.

`gote search -t <query>` is full-text (FTS5 over title, aliases, tags, body) and ANDs its terms. `-s <query>` is semantic. `-t` and `-s` together is a usage error. `gote tasks <query>` and `gote tag add --query` take the full-text syntax too. On a terminal `-t`/`-s` results open in the home-row menu, each entry a bold title, a dim `path:line` (plus the score for `-s`) and a one-line snippet trimmed to the terminal width with matched terms highlighted. `--list` (or `-l`) prints the same entries as a plain list; piped output (agents) gets the plain list automatically, with no ANSI codes. `--json` is unchanged.

### Archive

A note is archived when its frontmatter `tags` contains `archive` (case-insensitive); there is no other state. `gote archive <note>...` and `gote unarchive <note>...` add and remove the tag, touching only `tags`. Archived notes are hidden from fuzzy search, `-t`, `-s`, the finder, `recent`, `pinned`, `tag`, `get`, `tasks`, `orphans`, `diagram list` and `diagram search`. They are not hidden from link resolution, `links`, `backlinks`, `graph`, `broken`, `info` or opening by name. `--all` on any hiding command includes them, and an explicit `tag:archive` in a query (or `gote tag archive`) asks for them too. (`tasks --all` already meant "include done", so it now means done tasks and archived notes.)

| Token | Meaning |
|---|---|
| `word` | full-text term |
| `"two words"` | phrase |
| `tag:x` | has tag `x` |
| `field:value` | frontmatter field equals value (lists match any element) |
| `kind:note` / `kind:diagram` | only notes / only drawings |
| `created:<d>` | frontmatter `created` in the period |
| `modified:<d>` | file mtime in the period |
| `-token` | negates any token |

Date values follow the `date_format` config key ([docs/config.md](docs/config.md)). With the default `iso` they are ISO 8601 prefixes: `2026`, `2026-03`, `2026-03-14`, `2026-03-14T10`, `2026-03-14T10:30`. With `compact` they are `yymmddhhmmss` prefixes of even length 2-12 (`26`, `2603`, `260314`). Either way: a bare value (in the period), `>d` (after), `<d` (before), `d1..d2` (range). A value that is not valid in the configured format is an `invalid` error naming the token and the format. A query with only filters lists matches newest first.

Dates gote prints (`info`, `diagram list`, `--json` fields such as `created`, `modified`, trash `deleted`, server `started`) are always ISO 8601 with offset. Filenames keep compact stamps (`-d`, `-dt`, `daily/<yymmdd>_daily.md`, `<yymmddhhmmss>_diagram_...`).

### Frontmatter

```yaml
---
tags: [project, go]       # edited by `gote tag`
aliases: [gote-cli]       # extra names for search and link resolution
created: "2026-11-14T20:30:15-05:00"   # written by `new`
title: Display title
related: ["[[other note]]"]   # typed graph edges (quote [[...]] values)
parent: "[[index]]"
semantic: false           # exclude from embedding
---
```

New notes start with `created` (ISO 8601 with offset, quoted), `tags: []` and `aliases: []`. `created` is ISO only (RFC 3339, or a date-only / offset-less local value); anything else, such as an old `yymmddhhmmss`, is ignored and shown as a warning by `gote info`, so such notes never match `created:`; a template's own frontmatter wins for the keys it sets. Other keys are kept as written and searchable as `field:value`. There are no inline `#tags`.

### Links

`[[note]]`, `[[note|alias]]`, `[[note#heading]]`, `![[note]]` (embed) and relative `[text](path.md)` links all count. `rename` rewrites them. Links inside code are ignored.

## Server

The first command spawns a background server on a Unix socket in the state directory. It watches the notes directory and keeps the index current, so later commands start fast. It restarts itself when the gote version changes. `gote server status` and `gote server stop` manage it.

`--no-server` runs one command in-process with no server. Set `server: false` in the config to always do that. If the server cannot start, gote prints one warning and runs in-process.

## Semantic search

Optional. Configure an embedding provider in the `semantic:` config block: the built-in `openai` adapter works with any OpenAI-compatible endpoint (OpenAI, Gemini, Mistral, Together, or a local Ollama / LM Studio / llama.cpp server), and a `command` adapter runs any script you write (Cohere, Voyage, Bedrock, an internal service, a local model). Then run `gote embed --yes` (the first full embed needs confirmation), then `gote search -s <query>`. After that the server keeps embeddings current in the background. Supported providers, key handling, and how to write and test an adapter: [docs/embedding.md](docs/embedding.md).

## Drawings

Drawings are native `.excalidraw` files, indexed like notes: their text is searchable and `[[links]]` in them count. `gote diagram [name]` creates `diagrams/<yymmddhhmmss>_diagram[_<name>].excalidraw` and opens the browser editor (an embedded Excalidraw, saved back to the file). Embed one in a note with `![[<stem>]]`. `diagram list` shows drawings newest first (home-row menu on a terminal, `title  (id)` lines otherwise, `--all` includes archived, `[{id,title,path,modified}]` with `--json`); `diagram read` prints a text summary; `diagram check` prints `ok` or the binding problems it finds.

`gote diagram render <ref>` prints the path of a PNG of the drawing, rendered in pure Go (no browser): clean geometry, not the hand-drawn look, and unsupported element types show as labelled outlines. `--json` gives `{id, path, width, height, fresh}` (`fresh` is true when this call drew it). The PNG is always current: it is re-rendered when the drawing's content differs from the stored render. Where it lives is set by `diagram_png` ([docs/config.md](docs/config.md)): `cache` (default) keeps it under the state dir, `sidecar` writes `<name>.png` next to the drawing, `off` renders only on demand. The server re-renders a drawing in the background after any change; without a server `render` renders on demand. Agents use it to look at a drawing.

A drawing can carry a description, stored in the file as a top-level `"gote": {"description": "..."}` object. It is indexed for full-text and semantic search and shown by `diagram read`. `gote diagram describe <ref> <text...>` sets it, `gote diagram describe <ref> -` reads it from stdin (for long text), `--clear` removes it, and with no text it prints it; `--json` gives `{id, description}`. The browser editor keeps the description when it saves. `gote diagram search <words>...` searches drawings only, with the same modes as `gote search`: fuzzy on title and path by default, with filter tokens narrowing first (menu on a terminal, `title  (id)` lines otherwise, `{results:[{id,title,path,score}]}` with `--json`), `-t` full-text over labels and description with the full query syntax, `-s` semantic, `-n <limit>`. Agents are meant to set a description after creating or editing a drawing so it can be found later.

The web view makes no public network calls: pages are served from `127.0.0.1` only, fonts and scripts are bundled, and a strict Content-Security-Policy makes the browser enforce it. Excalidraw's share links, live collaboration, AI features, library browser and external embeds are disabled.

`gote view <note>` renders a note in the browser with live reload. The web server listens on localhost only and every request needs a per-start token.

## Testing

`go test ./...` covers the Go code. `make e2e` runs the browser end-to-end suite (Playwright, Chromium) against a throwaway vault and server in `web/e2e/`; it builds `./gote`, installs the Playwright Chromium browser if missing, and needs Node and network access on first run only.

## Agent skill

`skill/gote/SKILL.md` teaches an agent to drive gote through `--json`: search, tasks, graph, safe edits and the drawing workflow. Install it for Claude Code:

```
cp -R skill/gote ~/.claude/skills/
```

The examples in the skill are generated from a fixture vault by `skill/capture.sh <gote-binary>`; `go test ./skill` fails when they go stale.

### Custom agent skill

`skill/gote` is a reference skill and works as-is. To build a personalized one (your layout, tags, naming, daily-note workflow, integrations), copy `skill/template/SKILL.md` and have your agent follow `skill/template/BUILD.md`: it interviews you, fills the `CUSTOMIZE` blocks from your vault, and keeps the gote core section current. `go test ./skill` checks the template's command table against `gote help --json`.
