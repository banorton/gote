<div align="center">
  <img src="assets/logote.png" alt="Logo" width="180" />
</div>

# gote

A fast and simple CLI note-taking tool. Notes are stored as plain Markdown files, with robust tagging, pinning, trash, and search features.

## Commands

| Command   | Aliases           | Description                                 |
|-----------|-------------------|---------------------------------------------|
| (no args) |                   | Open/create a quick note                    |
| quick     | q                 | Open/create a quick note                    |
| <note>    |                   | Create or open a note by name               |
| help      | h                 | Show help message                           |
| recent    | r                 | Show recently modified notes                |
| index     | idx               | Rebuild the note index                      |
| tags      | ts                | List all tags                               |
| tag       | t                 | Show notes with a tag                       |
| config    | c                 | Edit or show config                         |
| search    | s                 | Search notes by title                       |
| search -t | s -t              | Search notes by tags                        |
| pin       | p                 | Pin a note                                  |
| pins      | pinned, pd        | List pinned notes                           |
| unpin     | u, up             | Unpin a note                                |
| delete    | d, del, trash     | Move note to trash                          |
| recover   |                   | Restore note from trash                     |
| rename    | mv, rn            | Rename a note                               |
| info      | i                 | Show note metadata                          |

## Examples

```
# Create or open a note
$ gote mynote

# Quick note (no args or 'quick')
$ gote
$ gote quick

# Show recent notes
$ gote recent

# List all tags
$ gote tags

# Show notes with a tag
$ gote tag project

# Search notes by title
$ gote search meeting

# Search notes by tags
$ gote search -t project urgent

# Search trashed notes
$ gote search trash old

# Pin and unpin notes
$ gote pin mynote
$ gote unpin mynote

# List pinned notes
$ gote pins

# Move note to trash
$ gote delete mynote

# Recover a trashed note
$ gote recover mynote

# Rename a note
$ gote rename mynote -n project-notes

# Show note metadata
$ gote info mynote

# Show help
$ gote help
```

## Tag Syntax
- Tags are specified on the first line of a note, separated by periods, e.g.:
  ```
  .project.urgent.personal
  ```
- Tags are automatically indexed and searchable.

## Data Storage
- Notes: Markdown files in your notes directory (default: `~/gotes`)
- Index: `.gote/index.json` (metadata for fast lookup)
- Tags: `.gote/tags.json` (tag metadata)
- Pins: `.gote/pins.json` (set of pinned notes)
- Trash: `.gote/trash/` (trashed notes)
- Config: `.gote/config.json` (user config)

## Requirements
- Go 1.18+
- Unix-like OS (macOS, Linux, WSL recommended)

## License
MIT
