<div align="center">
  <img src="assets/logote.png" alt="Logo" width="180" />
</div>

# gote

A CLI note-taking tool. Notes are plain Markdown files with tagging, pinning, search, and more.

## Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| `gote <note>` | | Create or open note |
| `gote <note> -t [template]` | | Create from template |
| `gote -d/-dt/-nt <note>` | | Date/datetime/no-timestamp prefix |
| `gote quick` | `q` | Open quick note |
| `gote -` | | Open last opened note (also works as alias: `gote view -`) |
| `gote quick save <name>` | `qs` | Save quick note |
| `gote recent` | `r` | Recent notes |
| `gote recent open/delete/pin/view` | `ro/rd/rp/rv` | Recent + mode |
| `gote search <query>` | `s` | Search by title |
| `gote search -t .tag1.tag2` | | Search by tags |
| `gote search -w <date>` | | Search by date |
| `gote tag` | `t` | List tags |
| `gote tag .tag1.tag2` | | Filter by tags |
| `gote tag open/delete/pin/view` | `to/td/tp/tv` | Tag filter + mode |
| `gote pin <note>` | `p` | Pin a note |
| `gote pin` | `p` | Interactive pinned menu |
| `gote pinned open/view/unpin` | `po/pv/pu` | Pinned + mode |
| `gote unpin <note>` | `u` | Unpin a note |
| `gote delete <note>` | `d` | Move to trash |
| `gote trash` | | List trash |
| `gote recover <note>` | | Restore from trash |
| `gote get` | `g` | Interactive select |
| `gote template` | `tmpl` | List templates |
| `gote index` | `idx` | Rebuild index |
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

gote s meeting           # search
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
| Pins | `~/.gote/pins.json` |
| Templates | `~/.gote/templates/*.md` |
| Trash | `~/.gote/trash/` |
| Config | `~/.gote/config.json` |

## Install

```bash
go build -o gote ./src
mv gote /usr/local/bin/
```

## License

MIT
