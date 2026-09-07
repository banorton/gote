<div align="center">
  <img src="assets/logote.png" alt="Logo" width="180" />
</div>

# gote

A CLI note-taking tool written in Go. Notes are plain Markdown files with tagging, pinning, search, and more.

Everything gote knows lives in two places: your notes, which are ordinary `.md` files in a
directory you pick, and `~/.gote`, which holds the index and nothing you would miss. Delete
gote tomorrow and you still have a folder of Markdown.

## Why another one

There are a lot of note CLIs. What is specific to this one:

- **Tags are the first line of the file**, period-delimited (`.project.urgent.work`). No
  YAML frontmatter, no database — a note stays readable and greppable in any editor.
- **Search is built in, not shelled out.** BM25 ranking with Snowball stemming, compiled
  into the binary. No ripgrep, fzf or external index required.
- **One static binary, no runtime.** Nothing to install alongside it.
- **Two-letter commands compose a lister with an action.** `ro` is recent + open, `sv` is
  search + view, `tp` is tag-filter + pin.

It is a single-user tool for people who already live in a terminal. There is no sync, no
mobile app, no web UI, and no plugin system.

## What it looks like

```console
$ gote r
[a] gopl
[s] standup
[d] index_format
[f] search_design
[g] sourdough

(1/1)────────────────────────
[q]uit
[o]pen (default) [v]iew [d]elete [r]ename [c]opy [p]in [i]nfo
: a

$ gote s bm25
[a] search_design
[s] standup

(1/1)────────────────────────
[q]uit
[o]pen (default) [v]iew [d]elete [r]ename [c]opy [p]in [i]nfo
: v

$ gote t
design (2)
meeting (1)
personal (1)
reading (1)
recipe (1)
work (3)
```

Selection keys are the home row — `a s d f g` — so picking the third result never
means reaching for a number key or an arrow.

A recorded walkthrough can be regenerated with `vhs scripts/demo.tape`.

## Install

Download a binary from [releases](https://github.com/banorton/gote/releases):

```bash
# macOS (Apple Silicon); swap for -mac-amd64, -linux-amd64, -linux-arm64, -win.exe
curl -L -o gote https://github.com/banorton/gote/releases/latest/download/gote-mac-arm64
chmod +x gote
mv gote /usr/local/bin/
```

With Go installed:

```bash
go install github.com/banorton/gote@latest
```

From source:

```bash
git clone https://github.com/banorton/gote && cd gote
go build -o gote . && mv gote /usr/local/bin/
```

## Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| `gote <note>` | | Create or open note |
| `gote <note> -t [template]` | | Create from template |
| `gote -d/-dt/-nt <note>` | | Date/datetime/no-timestamp prefix |
| `gote quick` | `q` | Open quick note |
| `gote -` | | Open last opened note |
| `gote view -` / `gote info -` / etc. | | Use `-` as last note alias in any command |
| `gote quick save <name>` | `qs` | Save quick note |
| `gote recent` | `r` | Recent notes |
| `gote recent open/delete/pin/view` | `ro/rd/rp/rv` | Recent + mode |
| `gote search <query>` | `s` | Search titles + content |
| `gote search --title <query>` | | Search by title only |
| `gote search -t .tag1.tag2` | | Search by tags |
| `gote search -w <date>` | | Search by date |
| `gote tag` | `t` | List tags |
| `gote tag .tag1.tag2` | | Filter by tags |
| `gote tag open/delete/pin/view` | `to/td/tp/tv` | Tag filter + mode |
| `gote pin <note>` | `p` | Pin a note |
| `gote pin` | `p` | Interactive pinned menu |
| `gote pinned open/delete/view/unpin` | `po/pd/pv/pu` | Pinned + mode |
| `gote unpin <note>` | `u` | Unpin a note |
| `gote delete <note>` | `d` | Move to trash |
| `gote trash` | | List trash |
| `gote recover <note>` | | Restore from trash |
| `gote get` | `g` | Interactive select |
| `gote template` | `tmpl` | List templates |
| `gote index` | `idx` | Rebuild index (includes FTS) |
| `gote index fts` | | Rebuild FTS index only |
| `gote config` | `c` | Show config |
| `gote config edit` | `ce` | Edit config |
| `gote info <note>` | `i` | Note metadata |
| `gote view <note>` | `v` | Preview in browser |
| `gote rename <note> -n <new>` | `mv` | Rename note |
| `gote help` | `h` | Show help |
| `gote -v` | | Show version |

## Examples

```bash
gote mynote              # create/open note
gote -d mynote           # with date prefix
gote mynote -t meeting   # from template

gote r                   # recent notes
gote ro                  # recent + open mode

gote s meeting           # search titles + content
gote s --title meeting   # title-only search
gote s -t .work          # search by tag
gote s -w 2412           # notes from Dec 2024
gote s -w 2412 2501      # date range

gote t .work.urgent      # filter by tags
gote p                   # pinned menu
gote g                   # interactive select
```

## Configuration

Config at `~/.gote/config.json`:

```json
{
  "noteDir": "/path/to/notes",
  "editor": "vim",
  "fancyUI": false,
  "timestampNotes": "none",
  "defaultPageSize": 10
}
```

| Option | Description |
|--------|-------------|
| `noteDir` | Notes directory |
| `editor` | Editor command |
| `fancyUI` | TUI mode with boxes and screen refresh |
| `timestampNotes` | `"none"`, `"date"`, or `"datetime"` |
| `defaultPageSize` | Results per page |

## Tags

First line of note, period-separated:

```
.project.urgent.work
```

## Data

| File | Location |
|------|----------|
| Notes | `~/gotes/*.md` |
| Index | `~/.gote/index.json` |
| Tags | `~/.gote/tags.json` |
| FTS Index | `~/.gote/fts.json` |
| Pins | `~/.gote/pins.json` |
| Templates | `~/.gote/templates/*.md` |
| Trash | `~/.gote/trash/` |
| Config | `~/.gote/config.json` |

## License

MIT
