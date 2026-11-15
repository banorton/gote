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
gote -                    # reopen the last note
gote search kubernetes    # ranked full-text search
gote search               # fuzzy finder (ctrl-t: search note contents)
gote tasks                # open "- [ ]" tasks across all notes
gote graph my-idea -d 2   # link neighborhood as a tree
```

Every command takes `--json` for machine-readable output. Exit codes: 0 ok, 1 error, 2 usage. `-` works as `<note>` in any command and means the last opened note. A `<note>` is an id (`projects/gote.md`), a filename stem, an alias, or a path suffix.

A note's id is its path relative to the notes directory. Its title is the frontmatter `title:`, else the filename. Files and directories starting with `.` are ignored (`.trash/`, `.git/`, `.quick.md`).

### Commands

| Command | Aliases | What it does |
|---|---|---|
| `open` | | open a note, creating it if missing (`gote <name>`). Flags: `-t <template>`, `-d` (date prefix), `-dt` (datetime prefix), `-nt` (no prefix) |
| `new` | | create a note and open it; fails if it exists |
| `quick` | `q` | open the quick note (`.quick.md`); `quick save <name>` turns it into a note |
| `quick-save` | `qs` | save the quick note as a note |
| `today` | | open the daily note (`daily/<yymmdd>_daily.md`); `today -1` is yesterday |
| `recent` | `r` | menu of recently opened notes |
| `get` | `g` | menu of all notes |
| `pinned` | `p` | menu of pinned notes; with a note name, pins it |
| `pin` | | pin a note |
| `unpin` | `u`, `up` | unpin a note |
| `tag` | `t` | no args: tags with counts. `tag <tag>...`: menu of notes with all tags. `tag add <tag> <note>...`, `tag rm <tag> <note>...`; `--query <q>` picks targets |
| `delete` | `d`, `del` | move a note to `.trash/` |
| `trash` | | list the trash; `trash empty` empties it |
| `recover` | | restore a note from the trash |
| `rename` | `mv`, `rn` | rename a note and rewrite every inbound link: `rename <note> -n <new>` |
| `duplicate` | `dup`, `cp` | copy a note |
| `info` | `i` | note details: tags, aliases, link counts, size |
| `template` | `tmpl` | `list`, `open <name>`, `rename <name> <new>`, `delete <name>` |
| `search` | `s` | search notes; no args opens the fuzzy finder. `-n <limit>`, `-s` for semantic |
| `tasks` | | list open tasks; `--all` includes done; a query restricts to matching notes |
| `links` | | outgoing links of a note |
| `backlinks` | | notes linking to a note |
| `graph` | | link neighborhood as a tree: `graph <note> [-d N]` (depth 1-5) |
| `orphans` | | notes with no links in or out |
| `broken` | | unresolved links |
| `diagram` | | Excalidraw drawings: `diagram [name]` creates and opens the editor, `diagram open <ref>`, `diagram read <ref>`, `diagram check <ref>` |
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
| search (rest of args = query) | `so` | `sd` | `sp` | `sv` | `sr` | |
| tag (rest of args = tags) | `to` | `td` | `tp` | `tv` | `tr` | |
| pinned | `po` | `pd` | | `pv` | `pr` | `pu` |

In a menu, items are labeled with the home row (`a s d f g h j k l ;`); `n`/`p` page; Shift-`O D P V R U` or Tab switch the action; `q` or esc quits. Without a terminal, list commands print `id<TAB>title` lines.

### Search

`gote search <query>` ANDs its terms.

| Token | Meaning |
|---|---|
| `word` | full-text term |
| `"two words"` | phrase |
| `tag:x` | has tag `x` |
| `field:value` | frontmatter field equals value (lists match any element) |
| `created:<d>` | frontmatter `created` in the period |
| `modified:<d>` | file mtime in the period |
| `-token` | negates any token |

Dates are `yymmddhhmmss` prefixes of even length 2-12: `26` is 2026, `2603` is March 2026. Forms: `2603`, `>2603` (after), `<2603` (before), `2601..2603` (range). A query with only filters lists matches newest first.

### Frontmatter

```yaml
---
tags: [project, go]       # edited by `gote tag`
aliases: [gote-cli]       # extra names for search and link resolution
created: "261114203015"   # written by `new`
title: Display title
related: [[other note]]   # typed graph edges
parent: [[index]]
semantic: false           # exclude from embedding
---
```

Other keys are kept as written and searchable as `field:value`. There are no inline `#tags`.

### Links

`[[note]]`, `[[note|alias]]`, `[[note#heading]]`, `![[note]]` (embed) and relative `[text](path.md)` links all count. `rename` rewrites them. Links inside code are ignored.

## Server

The first command spawns a background server on a Unix socket in the state directory. It watches the notes directory and keeps the index current, so later commands start fast. It restarts itself when the gote version changes. `gote server status` and `gote server stop` manage it.

`--no-server` runs one command in-process with no server. Set `server: false` in the config to always do that. If the server cannot start, gote prints one warning and runs in-process.

## Semantic search

Optional. Configure an embedding provider (OpenAI-compatible endpoint, or any command) in the `semantic:` config block, run `gote embed --yes` (the first full embed needs confirmation), then `gote search -s <query>`. After that the server keeps embeddings current in the background. Setup, key handling and the command adapter protocol: [docs/embedding.md](docs/embedding.md).

## Drawings

Drawings are native `.excalidraw` files, indexed like notes: their text is searchable and `[[links]]` in them count. `gote diagram [name]` creates `diagrams/<yymmddhhmmss>_diagram[_<name>].excalidraw` and opens the browser editor (an embedded Excalidraw, saved back to the file). Embed one in a note with `![[<stem>]]`. `diagram read` prints a text summary; `diagram check` prints `ok` or the binding problems it finds.

`gote view <note>` renders a note in the browser with live reload. The web server listens on localhost only and every request needs a per-start token.

## Agent skill

`skill/gote/SKILL.md` teaches an agent to drive gote through `--json`: search, tasks, graph, safe edits and the drawing workflow. Install it for Claude Code:

```
cp -R skill/gote ~/.claude/skills/
```

The examples in the skill are generated from a fixture vault by `skill/capture.sh <gote-binary>`; `go test ./skill` fails when they go stale.
