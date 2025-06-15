<div align="center">
  <img src="assets/logote.png" alt="Logo" width="180" />
</div>

# gote

Minimal, fast, clean command-line note-taking.

Features include instant note creation, powerful tag system, trash bin, quick notes, metadata tracking, and cross-platform support (macOS, Linux, Windows). All features use only the Go standard library—no dependencies.

## Features

- **Create/Open Notes:** `gote <note name>` — Instantly create or open notes, supports subdirectories and note names with spaces (e.g. `gote new note`).
- **Tagging:** Add tags at creation with `-t` or `--tags` (e.g. `gote new note -t project idea`). Tag line is always first, lowercased, delimited by periods (e.g. `project . idea`). Tags can contain spaces.
- **Quick Note:** Running `gote` with no arguments opens `quick.md` with `.quick` tag and ready to type.
- **Indexing:** `gote index` — Recursively indexes all `.md` files, extracts tags, creation/modification times, and saves metadata to `~/.gote/index.json`.
- **Persistent Metadata:** Index includes `Name`, `Path`, `Tags`, `Created`, `LastModified` (both as Unix timestamps and `yymmdd.hhmmss` strings).
- **Configurable Notes Directory:** `gote config set-dir <path>` — Stores in `~/.gote/config.json`.
- **Configurable Editor:** `gote config set-editor <editor>` — Set your preferred editor.
- **Safe Delete:** `gote config safe-delete [on|off]` — If on, prompts before deleting notes. Default is off.
- **Search:** `gote search <query>` — Fast, case-insensitive search by title. `gote search --tags <tags...>` or `gote search -t <tags...>` for tag search. Output is columnar and adapts to terminal width.
- **Tags:** `gote tags` — Lists all tags and their counts. `gote tags --sort popular` lists by popularity.
- **Recent:** `gote recent` — Lists notes by most recently modified, 20-char columns.
- **Pinning:** `gote pin <note>`, `gote unpin <note>`, `gote pinned` — Persistent pin list in `~/.gote/pinned.json`.
- **Archiving:** `gote archive <note>` — Moves a note to an `archive/` subdirectory.
- **Trash Bin:** `gote delete <note>` moves a note to `~/.gote/trash` instead of deleting. `gote trash` lists trashed notes, `gote recover <note>` restores.
- **Syntax-Highlighted Preview:** `gote view <note>` — Prints a colorized preview in the terminal.
- **Linting:** `gote lint <note>` — Checks for empty tag line, missing title, and formatting issues.
- **Subdirectory Support:** All note operations support relative paths and subdirectories.
- **Creation Time Tracking:** Each note has a `.created` file for accurate creation time.
- **Index Auto-Refresh:** Index is rebuilt if files change; `gote index` always rebuilds and saves the index.
- **Access Count Tracking:** Each note tracks how many times it has been opened. `gote popular [N]` shows the most accessed notes with proportional bars.
- **Move/Rename:** `gote move <old> <new>` (or `gote mv`) moves/renames notes (including subdirs). `gote rename <old> <new>` (or `gote rn`) renames a note within its directory.
- **Pack/Unpack:** `gote pack` zips all notes and metadata. `gote unpack <zipfile> <destdir>` restores them.
- **Reserved Words:** All command names and aliases are reserved and cannot be used as note names.
- **Short Aliases:** All major commands have single-letter aliases (see below).
- **Minimal Output:** All commands print lowercase, minimal status messages (e.g. `deleted <note>`, `pinned <note>`, `error <note>`).
- **Manual:** `gote help` or `gote h` shows all commands, aliases, and usage.

## Command Aliases

| Command   | Alias | Description                       |
|-----------|-------|-----------------------------------|
| delete    | d     | Move note to trash                |
| trash     |       | List trashed notes                |
| recover   |       | Restore note from trash           |
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
| popular   | x     | Show most accessed notes          |
| move      | mv,m  | Move/rename a note (can change dir)|
| rename    | rn    | Rename a note (same dir only)     |
| help      | h     | Show help/manual                  |
| pack      |       | Zip notes and metadata            |
| unpack    |       | Restore notes from a zip          |

## Usage Examples

- Create/open a note: `gote my note`
- Create/open a note with tags: `gote my note -t project idea`
- Tag a note: `gote tag my note project . idea`
- Move a note: `gote move my note subdir/newname`
- Rename a note: `gote rename my note newname`
- Search: `gote search meeting`
- Search by tags: `gote search -t project idea`
- List tags: `gote tags`
- List tags by popularity: `gote tags --sort popular`
- Show most popular: `gote popular 5`
- Trash a note: `gote delete my note`
- List trash: `gote trash`
- Recover from trash: `gote recover my note`
- Pack notes: `gote pack`
- Unpack notes: `gote unpack ~/.gote/notes_pack.zip ./restore_dir`
- Quick note: just run `gote` (no args)
- Enable safe delete: `gote config safe-delete on`

## Tag Syntax
- Tags are separated by periods (with optional spaces), not whitespace.
- Example: `project . idea . meeting notes` → tags: `project`, `idea`, `meeting notes`

## Data Storage
- All metadata is stored in `~/.gote/` (config, index, pins, access counts, trash, packs).
- Notes are stored in your configured notes directory (default: `~/gotes`).

## Building
- To build for macOS:
  `GOOS=darwin GOARCH=amd64 go build -o builds/gote-mac ./main.go`
- To build for Linux (WSL):
  `GOOS=linux GOARCH=amd64 go build -o builds/gote-linux ./main.go`
- To build for Windows:
  `GOOS=windows GOARCH=amd64 go build -o builds/gote-win.exe ./main.go`

## Requirements
- Go 1.18+
- No external dependencies (pure Go standard library)

## License
MIT
