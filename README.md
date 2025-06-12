# gote

Minimal, fast, and clean command-line note-taking for hackers and power users. All features use only the Go standard library.

## Features

- **Create/Open Notes:** `gote <note_name> [tags...]` — Instantly create or open notes, supports subdirectories.
- **Tagging:** Tag line is always first, lowercased, delimited by ` . `. Set, append, and list tags.
- **Indexing:** `gote index` — Recursively indexes all `.md` files, extracts tags, creation/modification times, and saves metadata to `~/.gote/index.json`. Prompts to salvage info from old index if present.
- **Persistent Metadata:** Index includes `Name`, `Path`, `Tags`, `Created`, `LastModified` (both as Unix timestamps and `yymmdd.hhmmss` strings).
- **Configurable Notes Directory:** `gote config set-dir <path>` — Stores in `~/.gote/config.json`.
- **Search:** `gote search <query>` — Fast, case-insensitive search by title. `gote search --tags <tags...>` for tag search.
- **Tags:** `gote tags` — Lists all tags and their counts.
- **Recent:** `gote recent` — Lists notes by most recently modified.
- **Pinning:** `gote pin <note>`, `gote unpin <note>`, `gote pinned` — Persistent pin list in `~/.gote/pinned.json`.
- **Archiving:** `gote archive <note>` — Moves a note to an `archive/` subdirectory.
- **Syntax-Highlighted Preview:** `gote view <note>` — Prints a colorized preview in the terminal.
- **Linting:** `gote lint <note>` — Checks for empty tag line, missing title, and formatting issues.
- **Subdirectory Support:** All note operations support relative paths and subdirectories.
- **Creation Time Tracking:** Each note has a `.created` file for accurate creation time.
- **Index Auto-Refresh:** Index is rebuilt if files change; `gote index` always rebuilds and saves the index.
- **Access Count Tracking:** Each note tracks how many times it has been opened. `gote popular [N]` shows the most accessed notes with proportional bars.
- **Note Linking:** Use `[[note name]]` to link to other notes. `gote links <note>` shows inbound and outbound links.
- **Move/Rename:** `gote move <old> <new>` (or `gote mv`) moves/renames notes (including subdirs). `gote rename <old> <new>` (or `gote rn`) renames a note within its directory.
- **Reserved Words:** All command names and aliases are reserved and cannot be used as note names.
- **Short Aliases:** All major commands have single-letter aliases (see below).
- **Manual:** `gote help` or `gote h` shows all commands, aliases, and usage.

## Command Aliases

| Command   | Alias | Description                       |
|-----------|-------|-----------------------------------|
| delete    | d     | Delete a note                     |
| index     | i     | Rebuild/search index              |
| tags      | t     | List all tags                     |
| search    | s     | Search notes                      |
| recent    | r     | List recent notes                 |
| pin       | p     | Pin a note                        |
| unpin     | u     | Unpin a note                      |
| archive   | a     | Archive a note                    |
| view      | v     | Preview a note                    |
| lint      | l     | Lint a note                       |
| config    | c     | Config directory                  |
| today     | n     | Daily note                        |
| links     | k     | Show note links                   |
| popular   | x     | Show most accessed notes          |
| move      | mv,m  | Move/rename a note (can change dir)|
| rename    | rn    | Rename a note (same dir only)     |
| help      | h     | Show help/manual                  |

## Usage Examples

- Create/open a note: `gote mynote project . idea`
- Tag a note: `gote tag mynote project . idea`
- Move a note: `gote move mynote subdir/newname`
- Rename a note: `gote rename mynote newname`
- Search: `gote search meeting`
- List tags: `gote tags`
- Show links: `gote links mynote`
- Show most popular: `gote popular 5`

## Data Storage
- All metadata is stored in `~/.gote/` (config, index, pins, access counts).
- Notes are stored in your configured notes directory (default: `~/gotes`).

## Requirements
- Go 1.18+
- No external dependencies (pure Go standard library)

## License
MIT
