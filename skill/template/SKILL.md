---
name: <your-skill-name>
description: <when your agent should use it — e.g. "my notes", "journal", "project log">. Drives the gote CLI with --json.
---

<!--
Template for a personal gote skill. Copy to ~/.claude/skills/<your-skill-name>/SKILL.md,
fill in every CUSTOMIZE block, then delete the guidance comments. See BUILD.md.
Do not edit the GOTE-CORE section by hand; regenerate it from upstream (Keep in sync, below).
-->

# <your-skill-name>

<!-- CUSTOMIZE: layout
Say where things live in the vault and where new notes go. Name the real directories.
Example:
  Vault: ~/notes. Daily notes in daily/, projects in projects/<name>/index.md,
  reference material in ref/, everything else at the root. New notes with no obvious
  home go to inbox/.
-->

<!-- CUSTOMIZE: note-types
List each kind of note, its frontmatter, and the template it uses (gote templates live in
~/.config/gote/templates/<name>.md; `gote new -t <name>` applies one, a template's own
frontmatter wins over the default stub). Example:
  | Type | Template | Frontmatter | Body sections |
  |---|---|---|---|
  | project | project | tags: [project], parent: "[[projects]]" | Goal, Log, Tasks |
  | meeting | meeting | tags: [meeting] | Attendees, Notes, Actions |
-->

<!-- CUSTOMIZE: tags
The tag vocabulary and tagging rules. Tags are the `tags:` frontmatter list only. Example:
  Allowed: project, meeting, idea, ref, person, draft. Lowercase, singular, no spaces.
  Every note gets exactly one type tag. Add `draft` to unfinished notes; remove it when done.
  Run `gote tag --json` before inventing a tag; reuse an existing one when it fits.
-->

<!-- CUSTOMIZE: naming
Filename and title conventions. Example:
  Lowercase kebab-case filenames; `title:` only when it differs from the filename.
  Meeting notes: <yymmdd>_<topic>.md. People: firstname-lastname.md.
  Add an alias for any common abbreviation: `aliases: [k8s]`.
-->

<!-- CUSTOMIZE: daily
How the daily note works. Example:
  Daily note is daily/<yymmdd>_daily.md (the default gote path; `gote today --json` creates it
  in an editor, so create or edit the file directly). Sections: Plan, Log, Tasks.
  When asked "what did I do yesterday", read yesterday's daily note and the open tasks in it.
  Carry unfinished `- [ ]` items forward by copying, never by deleting the original.
-->

<!-- CUSTOMIZE: create-vs-append
Decide when to create a new note, append to an existing one, or edit in place. Example:
  Append dated entries to a project's Log section. Create a new note only for a new topic
  that will be linked from at least one hub. Edit existing prose only when asked.
  Before creating, run `gote search --json <title words>` to avoid duplicates.
-->

<!-- CUSTOMIZE: linking
Linking conventions. Example:
  Every project note sets `parent: "[[projects]]"`. People are always linked as [[person]]
  on first mention per note. Use `related:` for peer notes. Check `gote orphans --json`
  after creating notes; an orphan needs a link from a hub.
-->

<!-- CUSTOMIZE: permissions
What the agent may do without asking and what needs confirmation. Example:
  Without asking: search, read, create notes in inbox/, add tags, append to Log sections.
  Ask first: rename, delete, edit existing prose, change another note's frontmatter,
  anything outside notes_dir, bulk changes touching more than 5 notes.
-->

<!-- CUSTOMIZE: integrations
Other tools and how notes map to them. Example:
  Issues: note frontmatter `issue: owner/repo#12` maps to a GitHub issue; use `gh` to read
  it, write results back into the note. Calendar: each meeting note has `event: <id>`.
  Notes are the source of truth; external tools are read-mostly unless stated here.
-->

<!-- CUSTOMIZE: reporting
How to report back to the user. Example:
  Terse. Cite notes as `id:line`. After changes, list files touched, one per line.
  Summaries are bullets, newest first, no more than 10 items.
-->

<!-- GOTE-CORE:START — generated from skill/template/SKILL.md upstream. Do not edit by hand. -->
## gote core

### What gote is

Single-binary CLI over a directory of Markdown notes and `.excalidraw` drawings (the vault, `notes_dir` in `~/.config/gote/config.yaml`). Files are the source of truth; the SQLite index is a cache gote rebuilds (`gote reindex`). The first command auto-spawns a background server that keeps the index fresh; `--no-server` (put it first) runs one command in-process; `server: false` in config makes that the default. Read note bodies straight from disk. Use gote to search, query structure (tags, tasks, links) and make link-safe changes.

### Calling gote

- Always pass `--json` and parse it. Exit codes: 0 ok, 1 error, 2 usage. With `--json`, errors are `{"error":{"code","msg"}}` on stderr; `code` is `not_found`, `invalid`, `conflict` or `internal`.
- Every result carries `id`, the vault-relative path (`projects/gote.md`). Absolute `path` appears in `search`, `info`, `new`, `open`, `today`.
- Never run commands that open a menu, a pager or a browser (`template open`, `config-edit`, `view`, menu and two-letter menu aliases, `diagram` without `read`/`check`). `new`, `open`, `quick` and `today` are safe with `--json` (or when stdout is not a terminal): they create the note if needed but do not launch the editor or record a visit.
- Create notes with `gote new <name> --json` (daily note: `gote today --json`), not by writing files by hand: gote applies the stub, template and prefix. Then edit the returned `path` with normal file tools.
- Resolving `<note>`: an id (`projects/gote.md`), a filename stem (`gote`), an alias, or a path suffix, case-insensitive. Ambiguous names resolve to the shortest path. `-` is the last opened note. An unknown ref is `not_found`.

### Commands

Menu commands print `id<TAB>title` lines without a terminal and are interactive otherwise; avoid them.

| Command | Aliases | What it does / key flags |
|---|---|---|
| `search` | `s` | fuzzy titles/paths/aliases; `-t` full-text with filters; `-s` semantic; `-n N` limit; no words opens a finder (avoid) |
| `tasks` | | open `- [ ]` items; `--all` includes done; optional query restricts notes |
| `info` | `i` | note metadata: tags, aliases, link counts, size, pinned |
| `tag` | `t` | no args: all tags with counts. `tag add <tag> <note...>`, `tag rm <tag> <note...>`, `--query <q>` picks targets. `tag <tag>...` is a menu (avoid) |
| `links` | | outgoing edges of a note |
| `backlinks` | | incoming edges of a note |
| `graph` | | link neighborhood tree: `graph <note> [-d N]`, depth 1-5 |
| `orphans` | | notes with no links in or out |
| `broken` | | unresolved links with source and line |
| `rename` | `mv` `rn` | `rename <note> -n <new>`; rewrites every inbound link |
| `delete` | `d` `del` | move to `.trash/` |
| `recover` | | restore from trash: `recover <name>` |
| `trash` | | list trash; `trash empty` is permanent (ask first) |
| `duplicate` | `dup` `cp` | `duplicate <note> <new>` |
| `pin` | | pin a note |
| `unpin` | `u` `up` | unpin a note |
| `pinned` | `p` | menu of pins; with a note name, pins it |
| `diagram` | | `diagram list [-n N] [--json]` drawings newest first (menu on a terminal: use `--json`), `diagram read <ref>` summary, `diagram check <ref>` validate; bare/`open` start the browser editor (user only) |
| `template` | `tmpl` | `template list`; `open`, `rename`, `delete` act on `~/.config/gote/templates/` |
| `status` | | index counts (`notes`, `links`, `unresolved`) |
| `reindex` | `idx` | drop and rebuild the index; use after bulk edits outside gote or stale results |
| `embed` | | compute embeddings for semantic search; `embed status`. See safety |
| `export` | `exp` | write notes, templates and pins to a tar.gz |
| `import` | `imp` | restore an export into an empty notes dir; `--force` overwrites (ask first) |
| `config` | `c` | show the config file |
| `config-edit` | `ce` | opens an editor (avoid) |
| `server` | | `server status`, `server stop` |
| `serve` | | run the server in the foreground (never; auto-spawned) |
| `help` | `h` `man` | `help --json` lists every command with usage |
| `version` | | print the version |
| `open` | | open or create a note; with `--json` returns `{id,path,created}` and skips the editor: `-t <template>`, `-d`/`-dt`/`-nt` name prefix |
| `new` | | create a note, `--json` returns `{id,path,created}` without opening the editor; fails with `conflict` if it exists |
| `quick` | `q` | quick note `.quick.md`; `--json` returns `{path}` without the editor; `quick save <name>` |
| `quick-save` | `qs` | save the quick note as a note |
| `today` | | daily note `daily/<yymmdd>_daily.md`; `--json` creates it if missing, returns `{id,path,created}`, no editor; `today -1` yesterday |
| `recent` `get` | `r` `g` | menus of recent / all notes (avoid) |
| `view` | `v` | rendered note in the browser (avoid); `-T` renders in the terminal |
| `ro` `rd` `rp` `rv` `rr` `so` `sd` `sp` `sv` `sr` `to` `td` `tp` `tr` `tv` `po` `pd` `pr` `pu` `pv` | | lister+action menu shortcuts (recent, search, tag, pinned; open, delete, pin, view, rename, unpin). Interactive; avoid |

### Search

`gote search --json [-n N] <words>`: fuzzy (fzf scoring) over titles, paths and aliases, best first. Never reads bodies; filter tokens (`tag:`, `kind:`, `created:`, `-word`, quotes) are an `invalid` error pointing at `-t`. Result: `{results:[{id,title,kind,path,score}]}`.

`gote search -t --json [-n N] <query>`: full-text over title, aliases, tags, body; terms ANDed. Result adds `snippet` and `line`. `-s` is semantic (needs a `semantic:` config block, else `invalid`); `-t` with `-s` is a usage error. Put flags before query tokens that start with `-`, or use `--`. `tasks <query>` and `tag add --query` take the same syntax.

| Token | Meaning |
|---|---|
| `word` | full-text term |
| `"two words"` | phrase |
| `tag:x` | has tag `x` |
| `field:value` | frontmatter field equals value; lists match any element |
| `kind:note` / `kind:diagram` | only notes / only drawings (anything else is `invalid`) |
| `created:<d>` | frontmatter `created` in the period |
| `modified:<d>` | file mtime in the period |
| `-token` | negates any token, e.g. `-tag:draft` |

Dates `<d>` are `yymmddhhmmss` prefixes of even length 2-12 (`26` is 2026, `2603` March 2026, `260314` one day). Forms: `2603`, `>2603` after, `<2603` before, `2601..2603` inclusive range. Odd length is a usage error. Filters only: lists matches by mtime, newest first.

### Frontmatter

```yaml
---
tags: [project, go]            # edit with `gote tag add|rm`
aliases: [gote-cli]            # extra names for search and link resolution
created: "261114203015"        # quoted yymmddhhmmss
title: Display title           # else the filename stem
related: ["[[other note]]"]    # typed graph edge; quote [[...]]
parent: "[[index]]"            # typed graph edge
semantic: false                # exclude from embedding
---
```

| Field | Notes |
|---|---|
| `tags` | list; the only tags gote reads (no inline `#tags`) |
| `aliases` | list; resolve like filenames |
| `created` | quoted `yymmddhhmmss` string; filter with `created:` |
| `title` | display title |
| `related`, `parent` | `[[...]]` values must be quoted YAML strings |
| `semantic: false` | note is never embedded |

A new note stub is `created`, `tags: []`, `aliases: []` (`gote new` writes it). There is no `modified` field: use `modified:` (file mtime). Other keys are preserved and searchable as `field:value`. A template's frontmatter wins for keys it sets.

### Links

`[[note]]`, `[[note|alias]]`, `[[note#heading]]`, `![[note]]` (embed) and relative `[text](path.md)` all count, plus `related`/`parent`. Links inside code are ignored. Edge kinds in `links`/`backlinks`: `wiki`, `embed`, `md`, `related`, `parent`; a link to a missing note has `id: null`. `rename` rewrites all of them.

### Safety rules

- Edit note files directly with normal file tools; gote reindexes on the next command. Never touch `.trash/`.
- Never rename or move a note with `mv`; use `gote rename <note> -n <new>` (it rewrites inbound links and returns them in `rewritten`).
- Never `rm` a note; use `gote delete` (recover with `gote recover <name>`).
- Never pass `embed --yes` without asking the user: the first full embed and any model change spend API budget.
- A write that races a concurrent change fails with `conflict` and writes nothing; re-read and retry.
- Ask before `trash empty` and `import --force`.

### Drawings

Native Excalidraw JSON, `<diagram_dir>/<yymmddhhmmss>_diagram[_<name>].excalidraw`; text is searchable and `[[links]]` inside count as edges. Embed with `![[<stem>]]`. Workflow:

1. `gote diagram read --json <ref>`: `summary` with `shapes` (id, type, label), `texts`, `edges` (`from` -> `to`, label; unbound end is `null`). Read this first; do not parse raw JSON to answer questions.
2. Edit the `.excalidraw` file directly (path is in the result).
3. `gote diagram check <ref>` must print `ok`. Exit 1 lists broken invariants.

Binding invariants (ignore elements with `isDeleted: true`):

- An arrow's `startBinding.elementId` / `endBinding.elementId` names an existing element whose `boundElements` contains `{"id": "<arrow id>", "type": "arrow"}`.
- A shape's `boundElements` entry points to an existing element that points back (arrow via binding, text via `containerId`).
- A text's `containerId` names a container whose `boundElements` lists `{"id": "<text id>", "type": "text"}`.
- Element ids are unique.
- To delete an element, set `"isDeleted": true` and remove references to it from other elements' `boundElements` and bindings.
- Every element you edit or add gets `version` incremented, a new random `versionNonce`, and `updated` set to now in ms. A new element needs all fields of its type: copy a sibling and change the ids.
<!-- GOTE-CORE:END -->

## Keep in sync

The core section above is generated from upstream: `skill/template/SKILL.md` in the gote repository, checked against `gote help --json` by `go test ./skill`. When gote updates, replace everything between the GOTE-CORE markers with the upstream version and leave the CUSTOMIZE blocks alone. Compare `gote version` and `gote help --json` with the command table.
