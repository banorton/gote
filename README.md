<div align="center">
  <img src="assets/logote.png" alt="Logo" width="180" />
</div>

# gote

A fast and simple CLI note-taking tool. Notes are stored as plain Markdown files, with robust tagging, pinning, trash, and search features.

## Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `gote` | | Open quick note (default) |
| `gote <note>` | | Create or open a note by name |
| `gote <note> -t <template>` | | Create note from template |
| `gote <note> -t` | | Create note, pick template interactively |
| `gote -d <note>` | `--date` | Create note with date prefix (yymmdd) |
| `gote -dt <note>` | `--datetime` | Create note with datetime prefix (yymmdd-hhmmss) |
| `gote -nt <note>` | `--no-timestamp` | Create note without timestamp (bypass config) |
| `gote quick` | `q` | Open quick note |
| `gote quick save <name>` | `qs` | Save quick note as named note |
| `gote recent` | `r` | List recent notes |
| `gote ro` | `recent open` | Recent + open mode |
| `gote rd` | `recent delete` | Recent + delete mode |
| `gote rp` | `recent pin` | Recent + pin mode |
| `gote rv` | `recent view` | Recent + view mode (browser preview) |
| `gote search [query]` | `s` | Search notes by title (prompts if no query) |
| `gote search -t .tag1.tag2` | | Search notes by tags |
| `gote search -w <date>` | `--when` | Search by creation date |
| `gote search -w <date> <date>` | | Search date range (inclusive) |
| `gote search -w <date> --modified` | `-m` | Search by modification date |
| `gote so <query>` | `search open` | Search + open mode |
| `gote sd <query>` | `search delete` | Search + delete mode |
| `gote sp <query>` | `search pin` | Search + pin mode |
| `gote sv <query>` | `search view` | Search + view mode (browser preview) |
| `gote tag` | `t` | List all tags |
| `gote tag .tag1.tag2` | | Filter notes by tags (AND logic) |
| `gote to .tag1.tag2` | `tag open` | Filter + open mode |
| `gote td .tag1.tag2` | `tag delete` | Filter + delete mode |
| `gote tp .tag1.tag2` | `tag pin` | Filter + pin mode |
| `gote tv .tag1.tag2` | `tag view` | Filter + view mode (browser preview) |
| `gote tag popular` | | Show most used tags |
| `gote tag edit` | | Edit tags file |
| `gote tag format` | | Format tags file |
| `gote pin <note>` | `p` | Pin a note |
| `gote pin` | `p` | Interactive pinned menu |
| `gote pinned` | | Interactive pinned menu |
| `gote po` | `pinned open` | Pinned + open mode |
| `gote pv` | `pinned view` | Pinned + view mode (browser preview) |
| `gote pu` | `pinned unpin` | Pinned + unpin mode |
| `gote unpin <note>` | `u`, `up` | Unpin a note |
| `gote delete <note>` | `d`, `del` | Move note to trash |
| `gote trash` | | List trashed notes |
| `gote trash <note>` | | Move note to trash |
| `gote trash empty` | | Permanently delete all trash |
| `gote trash search <query>` | | Search trashed notes |
| `gote recover <note>` | | Restore note from trash |
| `gote rename <note> -n <new>` | `mv`, `rn` | Rename a note |
| `gote info <note>` | `i` | Show note metadata |
| `gote view <note>` | | Open note preview in browser |
| `gote index` | `idx` | Rebuild the note index |
| `gote index edit` | | Edit index file |
| `gote index format` | | Format index file |
| `gote index clear` | | Clear and rebuild index from scratch |
| `gote config` | `c` | Show config |
| `gote config edit` | `ce` | Edit config (uses vim) |
| `gote config format` | | Format config file |
| `gote config help` | | Show config options |
| `gote get` | `g` | Interactive note selection and action |
| `gote template` | `tmpl` | List all templates |
| `gote template <name>` | | Create or edit a template |
| `gote template delete <name>` | | Delete a template |
| `gote help` | `h`, `man` | Show help message |
| `gote -v` | `--version` | Show version |

## Examples

```bash
# Create or open a note
gote mynote

# Create with timestamp prefix
gote -d mynote        # creates "241223 mynote.md"
gote -dt mynote       # creates "241223-152030 mynote.md"
gote -nt mynote       # creates "mynote.md" (no timestamp, even if config enabled)

# Quick note
gote
gote quick
gote qs mynote       # save quick.md as "mynote.md"

# Recent notes
gote recent          # list recent
gote ro              # select to open (shorthand)
gote recent open     # select to open (keyword)
gote rd              # select to delete
gote rp              # select to pin

# Search
gote search          # prompts for query
gote search meeting  # search by title
gote so meeting      # search + open (shorthand)
gote search open meeting  # search + open (keyword)
gote sd meeting      # search + delete
gote sp meeting      # search + pin
gote search -t .work  # search by tag
gote search -t .work.urgent  # search by multiple tags

# Date search (by creation date)
gote search -w 24           # all notes from 2024
gote search -w 2412         # all notes from Dec 2024
gote search -w 241223       # notes from Dec 23, 2024
gote search -w 241223.15    # notes from 3pm hour
gote search -w 2412 2501    # Dec 2024 through Jan 2025
gote search -w 241201 241231  # Dec 1-31, 2024
gote search -w 2412 --modified  # by modification date

# Tags
gote tag             # list all tags
gote t               # list all tags (shorthand)
gote tag popular     # most used tags
gote tag .work       # filter notes by tag (must have "work")
gote tag .work.urgent  # filter notes with ALL tags (AND logic)
gote to .work        # filter + open mode (shorthand)
gote td .work.urgent # filter + delete mode
gote tp .work        # filter + pin mode

# Pins
gote pin mynote      # pin a note
gote pin             # list pinned
gote po              # select to open (shorthand)
gote pinned open     # select to open (keyword)
gote unpin mynote    # unpin

# Trash
gote delete mynote   # move to trash
gote trash           # list trashed notes
gote trash empty     # permanently delete all
gote recover mynote  # restore from trash

# Get (interactive)
gote get             # choose source -> select note -> choose action
gote g               # shorthand

# Templates
gote template           # list templates
gote template meeting   # create/edit meeting template
gote template delete meeting  # delete template
gote standup -t meeting # create note from meeting template
gote standup -t         # create note, pick template interactively

# View (browser preview)
gote view mynote        # open note as HTML in browser
gote rv                 # recent + view mode
gote sv meeting         # search + view mode
gote tv .work           # tag filter + view mode
gote pv                 # pinned + view mode

# Other
gote rename mynote -n project-notes
gote info mynote
gote ce                 # edit config (shortcut)
gote config edit        # edit config
gote help
gote -v
```

## Configuration

Config file is at `~/.gote/config.json`:

```json
{
  "noteDir": "/Users/you/gotes",
  "editor": "vim",
  "fancyUI": false,
  "timestampNotes": "none",
  "defaultPageSize": 10
}
```

| Option | Description |
|--------|-------------|
| `noteDir` | Directory where notes are stored |
| `editor` | Editor to open notes with |
| `fancyUI` | Enable TUI mode with boxes, single-keypress input, and screen refresh |
| `timestampNotes` | Auto-prefix notes: `"none"`, `"date"` (yymmdd), or `"datetime"` (yymmdd-hhmmss) |
| `defaultPageSize` | Number of results to show by default (override with `-n`) |

## Tag Syntax

Tags are specified on the first line of a note, starting with a period and separated by periods:

```
.project.urgent.work
```

The first line **must start with `.`** for tags to be recognized. Lines without a leading period are treated as regular content with no tags.

Tags are automatically indexed and searchable.

## Data Storage

| File | Location | Purpose |
|------|----------|---------|
| Notes | `~/gotes/*.md` | Your markdown notes |
| Index | `~/.gote/index.json` | Note metadata for fast lookup |
| Tags | `~/.gote/tags.json` | Tag index |
| Pins | `~/.gote/pins.json` | Pinned notes |
| Templates | `~/.gote/templates/*.md` | Note templates |
| Trash | `~/.gote/trash/` | Deleted notes |
| Config | `~/.gote/config.json` | User configuration |

## Installation

```bash
go build -o gote ./src
mv gote /usr/local/bin/  # or add to PATH
```

## Requirements

- Go 1.18+
- Unix-like OS (macOS, Linux, WSL)

## License

MIT
