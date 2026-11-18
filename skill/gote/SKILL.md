---
name: gote
description: Use when the user asks about their notes or vault - find, search, read, list, tag, rename, link, backlink, or summarize notes; list open tasks or TODOs across notes; inspect the link graph (orphans, broken links); or read, create, or edit an Excalidraw drawing/diagram stored in the vault. Drives the gote CLI with --json.
---

# gote

gote indexes a directory of Markdown notes and `.excalidraw` drawings (the vault, `notes_dir` in `~/.config/gote/config.yaml`). Notes are plain files; the index is a cache. Use gote to search, query structure (tags, tasks, links), and make link-safe changes. Read note bodies straight from disk.

## Rules

- Always pass `--json` when you parse the output. Exit codes: 0 ok, 1 error, 2 usage. On error with `--json`, stderr is `{"error":{"code","msg"}}`; codes are `not_found`, `invalid`, `conflict`, `internal`.
- Put `--no-server` first when you want a one-shot in-process run (no background server). Otherwise the first call auto-spawns a server that keeps the index fresh.
- Create notes with `gote new <name> --json` (daily note: `gote today --json`); with `--json` they never launch the editor or record a visit, and return `{id,path,created}`. Then edit `path` directly.
- Never run commands that open a menu or a pager unless asked: `template open`, `config edit`, `view`, `search` without `--json`, `recent`, `get`, `pinned`, `tag <t>` and the two-letter menu aliases (`ro`, `sv`, ...). Use `search --json`, `tag`, `tasks`, `info`, `links`, `backlinks`, `graph`, `orphans`, `broken`, `diagram list --json`, `diagram search --json`, `diagram read|check|describe` instead.
- `<note>` arguments accept an id (`projects/gote.md`), a filename stem (`gote`), an alias, or a path suffix. Ambiguous names resolve to the shortest path. `-` is the last opened note.
- Every result carries `id`: the vault-relative path. Absolute `path` is in `search` and `info` results.

## Editing safely

- Notes are plain files. To change content, edit the file directly; gote reindexes on the next command (or within a second when the server runs). Do not edit `.trash/`.
- Rename only with `gote rename <note> -n <new>`: it rewrites every inbound link (`[[x]]`, `[[x|alias]]`, `[[x#h]]`, `![[x]]`, `[t](x.md)`). A plain `mv` breaks links.
- Delete with `gote delete <note>`: the file moves to `.trash/`, restore with `gote recover <name>`. Do not `rm` notes.
- Tags live in the frontmatter `tags:` list. Change them with `gote tag add <tag> <note...>` and `gote tag rm <tag> <note...>` (or `--query <q>` to pick targets); they edit only that field. Do not invent `#inline` tags: gote does not read them.
- Writes check the file mtime; a concurrent change fails with `conflict` and nothing is written. Re-read and retry.
- Frontmatter gote reads: `tags`, `aliases`, `created` (`"yymmddhhmmss"`), `title`, `related`, `parent` (typed link edges), `semantic: false` (skip embedding). Other keys are preserved and searchable as `key:value`.

## Search

`gote search [-n N] <words...>` is a fuzzy match (fzf scoring) over note titles, paths and aliases, best first. It never reads note bodies and takes no filters; a filter token (`tag:`, `kind:`, `created:`, `-word`, quotes) is an `invalid` error that points at `-t`. Result: `id`, `title`, `kind`, `path` (absolute), `score`.

```console
$ gote search --json desi
{
  "results": [
    {
      "id": "design.md",
      "title": "design",
      "kind": "note",
      "path": "~/notes/design.md",
      "score": 114
    },
    {
      "id": "ideas.md",
      "title": "ideas",
      "kind": "note",
      "path": "~/notes/ideas.md",
      "score": 71
    },
    {
      "id": "diagrams/261101130000_diagram_broken.excalidraw",
      "title": "261101130000_diagram_broken",
      "kind": "diagram",
      "path": "~/notes/diagrams/261101130000_diagram_broken.excalidraw",
      "score": 38
    },
    {
      "id": "diagrams/261101120000_diagram_architecture.excalidraw",
      "title": "261101120000_diagram_architecture",
      "kind": "diagram",
      "path": "~/notes/diagrams/261101120000_diagram_architecture.excalidraw",
      "score": 32
    }
  ]
}
```

`gote search -t [-n N] <query...>` is full-text search (title, aliases, tags, body) with filters. Terms are ANDed. `-s` instead of `-t` is semantic search (needs the `semantic:` config block; otherwise `invalid`). `-t` and `-s` together is a usage error. Tokens for `-t`:

| Token | Meaning |
|---|---|
| `word` | full-text term (title, aliases, tags, body) |
| `"two words"` | phrase |
| `tag:x` | has tag `x` |
| `field:value` | frontmatter field equals value (case-insensitive; lists match any element) |
| `kind:note` / `kind:diagram` | only notes / only drawings (anything else is `invalid`) |
| `created:<d>` | frontmatter `created` in the period |
| `modified:<d>` | file mtime in the period |
| `-token` | negate any token, e.g. `-tag:draft` |

Dates `<d>` are `yymmddhhmmss` prefixes of even length 2-12: `26` = 2026, `2603` = March 2026, `260314` = 14 March 2026. Forms: `2603` (inside the period), `>2603` (after it), `<2603` (before it), `2601..2603` (both periods inclusive). An odd length is a usage error. A query of only filters lists matching notes by mtime, newest first. Query tokens that start with `-` stay part of the query; put flags (`-n`, `-t`, `-s`) first, or use `--` to end flags.

Full-text result: `id`, `title`, `kind` (`note` or `diagram`), `path` (absolute), `snippet`, `line` (first matching body line, 0 if none), `score`.

```console
$ gote search -t --json socket
{
  "results": [
    {
      "id": "design.md",
      "title": "design",
      "kind": "note",
      "path": "~/notes/design.md",
      "snippet": "\n# Design\n\nClient talks to server over a Unix socket. Implements [[gote-cli]].\n",
      "line": 8,
      "score": 1.3155919755628585
    }
  ]
}
```

```console
$ gote search -t --json tag:go -tag:project
{
  "results": [
    {
      "id": "design.md",
      "title": "design",
      "kind": "note",
      "path": "~/notes/design.md",
      "snippet": "",
      "line": 0,
      "score": 0
    }
  ]
}
```

```console
$ gote search -t --json kind:diagram architecture
{
  "results": [
    {
      "id": "diagrams/261101120000_diagram_architecture.excalidraw",
      "title": "261101120000_diagram_architecture",
      "kind": "diagram",
      "path": "~/notes/diagrams/261101120000_diagram_architecture.excalidraw",
      "snippet": "Client\nServer\nrequest\nCache\nsee [[Design Doc]]\nRequest path from client through…",
      "line": 0,
      "score": 1.1462076785359614
    }
  ]
}
```

```console
$ gote search -t --json created:2611
{
  "results": [
    {
      "id": "ideas.md",
      "title": "ideas",
      "kind": "note",
      "path": "~/notes/ideas.md",
      "snippet": "",
      "line": 0,
      "score": 0
    }
  ]
}
```

Without `--json`, a result is `title  (id)` and an indented snippet:

```console
$ gote search -t modified:2611 -tag:ideas
261101130000_diagram_broken  (diagrams/261101130000_diagram_broken.excalidraw)
261101120000_diagram_architecture  (diagrams/261101120000_diagram_architecture.excalidraw)
Gote  (projects/gote.md)
lonely  (lonely.md)
design  (design.md)
```

## Tasks

`gote tasks [--all] [<query>...]` lists open `- [ ]` items across notes, newest note first, then by line. `--all` adds done items (`- [x]`). A query restricts to notes matching it (same syntax as `search -t`). Text output is `title (id:line)  [ ] text`.

```console
$ gote tasks
Gote (projects/gote.md:17)  [ ] write the release notes
Gote (projects/gote.md:19)  [ ] link [[roadmap]]
ideas (ideas.md:10)  [ ] try local embeddings
```

```console
$ gote tasks --all --json tag:go
[
  {
    "id": "projects/gote.md",
    "title": "Gote",
    "line": 17,
    "text": "write the release notes",
    "done": false
  },
  {
    "id": "projects/gote.md",
    "title": "Gote",
    "line": 18,
    "text": "tag v1.0.0",
    "done": true
  },
  {
    "id": "projects/gote.md",
    "title": "Gote",
    "line": 19,
    "text": "link [[roadmap]]",
    "done": false
  }
]
```

## Notes and tags

`gote info <note>` gives metadata and counts; `gote tag` lists every tag with counts.

```console
$ gote info --json gote
{
  "id": "projects/gote.md",
  "title": "Gote",
  "kind": "note",
  "path": "~/notes/projects/gote.md",
  "created": "",
  "modified": "261101120000",
  "size": 301,
  "tags": [
    "go",
    "project"
  ],
  "aliases": [
    "gote-cli"
  ],
  "links": 5,
  "backlinks": 2,
  "pinned": false
}
```

```console
$ gote tag --json
[
  {
    "tag": "go",
    "count": 2
  },
  {
    "tag": "ideas",
    "count": 1
  },
  {
    "tag": "project",
    "count": 1
  }
]
```

## Graph

Edge kinds: `wiki`, `embed` (`![[x]]`), `md` (`[t](x.md)`), `related`, `parent`. A link whose target does not exist has `id: null`. Drawings are nodes too.

- `gote links <note>`: outgoing edges.
- `gote backlinks <note>`: incoming edges.
- `gote graph <note> [-d N]`: neighborhood tree, depth 1-5 (default 1). `--json` gives nested `{id,title,dir,kind,seen,children}`; `dir` is `out` or `in`; `seen` marks a node already expanded elsewhere.
- `gote orphans`: notes with no links in or out.
- `gote broken`: every unresolved link with its source and line.

```console
$ gote links --json gote
[
  {
    "id": "ideas.md",
    "raw": "ideas",
    "title": "ideas",
    "kind": "related",
    "line": 0
  },
  {
    "id": "ideas.md",
    "raw": "ideas",
    "title": "ideas",
    "kind": "wiki",
    "line": 11
  },
  {
    "id": "design.md",
    "raw": "design",
    "title": "design",
    "kind": "wiki",
    "line": 11
  },
  {
    "id": "diagrams/261101120000_diagram_architecture.excalidraw",
    "raw": "261101120000_diagram_architecture",
    "title": "261101120000_diagram_architecture",
    "kind": "embed",
    "line": 13
  },
  {
    "id": null,
    "raw": "roadmap",
    "title": "roadmap",
    "kind": "wiki",
    "line": 19
  }
]
```

```console
$ gote backlinks --json ideas
[
  {
    "id": "projects/gote.md",
    "title": "Gote",
    "kind": "related",
    "line": 0
  },
  {
    "id": "projects/gote.md",
    "title": "Gote",
    "kind": "wiki",
    "line": 11
  }
]
```

```console
$ gote graph gote -d 2
Gote
├── → ideas [related]
├── → design
│   ├── ← 261101120000_diagram_architecture (seen)
│   └── ← 261101130000_diagram_broken
└── → 261101120000_diagram_architecture [embed]
    └── → design (seen)
```

```console
$ gote orphans --json
[
  {
    "id": "lonely.md",
    "title": "lonely",
    "kind": "note"
  }
]
```

```console
$ gote broken --json
[
  {
    "src": "diagrams/261101120000_diagram_architecture.excalidraw",
    "raw": "server notes",
    "line": 0
  },
  {
    "src": "diagrams/261101130000_diagram_broken.excalidraw",
    "raw": "server notes",
    "line": 0
  },
  {
    "src": "projects/gote.md",
    "raw": "roadmap",
    "line": 19
  }
]
```

## Drawings

Drawings are native Excalidraw JSON files named `<diagram_dir>/<yymmddhhmmss>_diagram[_<name>].excalidraw`. They are indexed: their text is searchable and `[[links]]` inside them count as edges. Embed one in a note with `![[<stem>]]`.

Workflow to understand or change a drawing:

1. `gote diagram read --json <ref>` gives a summary: `shapes` (with their label), standalone `texts`, and `edges` (arrows as `from` -> `to` with a label; an unbound end is `null`). Read this first; do not parse the raw JSON to answer questions.
2. To change it, edit the `.excalidraw` JSON file directly (the path is in the read result).
3. Run `gote diagram check <ref>`. It must print `ok`. Exit 1 lists the broken invariants.
4. After creating a drawing or substantially editing one, set or refresh its description: `gote diagram describe <ref> -` with a short summary of what it shows (read from stdin, so long text is fine). The description is stored in the file as a top-level `"gote": {"description": "..."}` object, indexed for `-t` and `-s` search, and shown by `diagram read`. Write it so a later search finds the drawing: name the system, the main components and the purpose.

Find drawings with `gote diagram search`, which only returns drawings: fuzzy over title and path by default, `-t` full-text over labels and description (full query syntax: tags, dates, field filters), `-s` semantic, `-n <limit>`. Always pass `--json`: without it a terminal gets a menu.

Description commands: `gote diagram describe <ref>` prints it, `gote diagram describe <ref> <text...>` sets it, `gote diagram describe <ref> -` reads it from stdin, `gote diagram describe <ref> --clear` removes it; `--json` gives `{id, description}`. A write after the file changed since it was indexed fails with `conflict`; re-read and retry.

When you edit the `.excalidraw` JSON directly, or rewrite it with a script, keep every unknown top-level key, above all `"gote"`: the description lives there. Load the whole object, change `elements`, write it back; never rebuild the file from only `type`, `elements`, `appState`, `files`.

```console
$ gote diagram list --json
[
  {
    "id": "diagrams/261101130000_diagram_broken.excalidraw",
    "title": "261101130000_diagram_broken",
    "path": "~/notes/diagrams/261101130000_diagram_broken.excalidraw",
    "modified": "261101140000"
  },
  {
    "id": "diagrams/261101120000_diagram_architecture.excalidraw",
    "title": "261101120000_diagram_architecture",
    "path": "~/notes/diagrams/261101120000_diagram_architecture.excalidraw",
    "modified": "261101130000"
  }
]
```

```console
$ gote diagram read --json 261101120000_diagram_architecture
{
  "id": "diagrams/261101120000_diagram_architecture.excalidraw",
  "path": "~/notes/diagrams/261101120000_diagram_architecture.excalidraw",
  "summary": {
    "description": "Request path from client through server to cache",
    "shapes": [
      {
        "id": "rect-a",
        "type": "rectangle",
        "label": "Client"
      },
      {
        "id": "rect-b",
        "type": "rectangle",
        "label": "Server"
      },
      {
        "id": "ell-c",
        "type": "ellipse",
        "label": "Cache"
      }
    ],
    "texts": [
      {
        "id": "note-1",
        "text": "see [[Design Doc]]"
      }
    ],
    "edges": [
      {
        "id": "arrow-1",
        "from": "Client",
        "to": "Server",
        "label": "request"
      },
      {
        "id": "arrow-2",
        "from": "Server",
        "to": null,
        "label": ""
      }
    ]
  }
}
```

Check result for a good and a bad drawing:

```console
$ gote diagram check 261101120000_diagram_architecture
ok
```

```console
$ gote diagram check 261101130000_diagram_broken
arrow-1: endBinding target "rect-b" does not list this element in boundElements
gote: invalid: 1 binding issue(s) in diagrams/261101130000_diagram_broken.excalidraw
```

Binding invariants (both directions must agree, ignoring elements with `isDeleted: true`):

- An arrow's `startBinding.elementId` / `endBinding.elementId` points to an existing element, and that element's `boundElements` contains `{"id": "<arrow id>", "type": "arrow"}`.
- A shape's `boundElements` entry `{"id", "type"}` points to an existing element that points back: an arrow via its binding, a text via `containerId`.
- A text's `containerId` names a container whose `boundElements` lists `{"id": "<text id>", "type": "text"}`. A container that lists a text must be that text's `containerId`.
- Element ids are unique across the file.
- Deleting an element: set `"isDeleted": true` and remove its references from other elements' `boundElements` and bindings.
- Every element you edit or add gets its `version` incremented, a new random integer `versionNonce`, and `updated` set to the current time in milliseconds. A new element needs all the fields of its type; copy an existing sibling and change the ids.

```console
$ gote diagram search --json arch
{
  "results": [
    {
      "id": "diagrams/261101120000_diagram_architecture.excalidraw",
      "title": "261101120000_diagram_architecture",
      "path": "~/notes/diagrams/261101120000_diagram_architecture.excalidraw",
      "score": 104
    }
  ]
}
```

```console
$ gote diagram search -t --json cache
{
  "results": [
    {
      "id": "diagrams/261101120000_diagram_architecture.excalidraw",
      "title": "261101120000_diagram_architecture",
      "kind": "diagram",
      "path": "~/notes/diagrams/261101120000_diagram_architecture.excalidraw",
      "snippet": "Client\nServer\nrequest\nCache\nsee [[Design Doc]]\nRequest path from client through…",
      "line": 6,
      "score": 0.7880574676250016
    },
    {
      "id": "diagrams/261101130000_diagram_broken.excalidraw",
      "title": "261101130000_diagram_broken",
      "kind": "diagram",
      "path": "~/notes/diagrams/261101130000_diagram_broken.excalidraw",
      "snippet": "Client\nServer\nrequest\nCache\nsee [[Design Doc]]",
      "line": 304,
      "score": 0.7007111965828218
    }
  ]
}
```

```console
$ gote diagram describe 261101120000_diagram_architecture
Request path from client through server to cache
```

```console
$ gote diagram describe --json 261101130000_diagram_broken Sample with deliberately broken bindings
{
  "id": "diagrams/261101130000_diagram_broken.excalidraw",
  "description": "Sample with deliberately broken bindings"
}
```

`gote diagram list` and `gote diagram search` without `--json` open a menu on a terminal; always pass `--json`. `gote diagram <name>` and `gote diagram open <ref>` open the browser editor: for the user, not for you.

## Other commands

- `gote rename <note> -n <new>` rewrites inbound links and returns the changed notes in `rewritten`.

```console
$ gote rename --json ideas backlog
{
  "id": "backlog.md",
  "path": "~/notes/backlog.md",
  "rewritten": [
    "projects/gote.md"
  ]
}
```

```console
$ gote backlinks backlog
projects/gote.md [related]
projects/gote.md :11
```

- `gote delete <note>`, `gote recover <name>`, `gote trash`, `gote duplicate <note> <new>`, `gote pin|unpin <note>`.
- `gote status` shows index counts; `gote reindex` rebuilds the index (use after bulk edits outside the vault, or if results look stale).
- `gote embed` computes embeddings for semantic search; `gote embed status` shows progress. The first full embed (and a model change) is refused without `--yes`, which spends API budget; ask before passing it. Once a vault is embedded, the server keeps it current in the background.
- `gote help --json` lists every command with its usage.
