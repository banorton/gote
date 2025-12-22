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
| `gote quick` | `q` | Open quick note |
| `gote recent` | `r` | List recent notes |
| `gote ro` | | Recent + open mode |
| `gote rd` | | Recent + delete mode |
| `gote search <query>` | `s` | Search notes by title |
| `gote search -t <tags>` | | Search notes by tags |
| `gote so <query>` | | Search + open mode |
| `gote sd <query>` | | Search + delete mode |
| `gote tags` | `ts` | List all tags |
| `gote tags popular` | | Show most used tags |
| `gote tag <note> -t <tags>` | `t` | Add tags to a note |
| `gote pin <note>` | `p` | Pin a note |
| `gote pin` | | List pinned notes |
| `gote pinned` | `pd` | List pinned notes |
| `gote po` | | Pinned + open mode |
| `gote unpin <note>` | `u`, `up` | Unpin a note |
| `gote delete <note>` | `d`, `del` | Move note to trash |
| `gote trash` | | List trashed notes |
| `gote trash <note>` | | Move note to trash |
| `gote trash empty` | | Permanently delete all trash |
| `gote trash search <query>` | | Search trashed notes |
| `gote recover <note>` | | Restore note from trash |
| `gote rename <note> -n <new>` | `mv`, `rn` | Rename a note |
| `gote info <note>` | `i` | Show note metadata |
| `gote index` | `idx` | Rebuild the note index |
| `gote config` | `c` | Show config |
| `gote config edit` | | Edit config (uses vim) |
| `gote help` | `h`, `man` | Show help message |

## Examples

```bash
# Create or open a note
gote mynote

# Quick note
gote
gote quick

# Recent notes
gote recent          # list recent
gote ro              # list + select to open
gote rd              # list + select to delete

# Search
gote search meeting  # search by title
gote so meeting      # search + open
gote sd meeting      # search + delete
gote search -t work  # search by tags

# Tags
gote tags            # list all tags
gote tags popular    # most used tags
gote tag mynote -t work urgent  # add tags to note

# Pins
gote pin mynote      # pin a note
gote pin             # list pinned
gote po              # list + select to open
gote unpin mynote    # unpin

# Trash
gote delete mynote   # move to trash
gote trash           # list trashed notes
gote trash empty     # permanently delete all
gote recover mynote  # restore from trash

# Other
gote rename mynote -n project-notes
gote info mynote
gote config edit
gote help
```

## Tag Syntax

Tags are specified on the first line of a note, separated by periods:

```
.project.urgent.work
```

Tags are automatically indexed and searchable.

## Data Storage

| File | Location | Purpose |
|------|----------|---------|
| Notes | `~/gotes/*.md` | Your markdown notes |
| Index | `~/.gote/index.json` | Note metadata for fast lookup |
| Tags | `~/.gote/tags.json` | Tag index |
| Pins | `~/.gote/pins.json` | Pinned notes |
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
