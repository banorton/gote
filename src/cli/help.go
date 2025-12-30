package cli

import "fmt"

func HelpCommand(args []string) {
	PrintDefaultHelp()
}

func NotImplementedCommand(name string) {
	fmt.Printf("%s: not implemented\n", name)
}

func PrintDefaultHelp() {
	fmt.Println(`gote: A simple CLI note-taking tool

Usage:
  gote <note name>                Create or open a note
  gote -d <note>                  Create note with date prefix (yymmdd)
  gote -dt <note>                 Create note with datetime prefix (yymmdd-hhmmss)
  gote -nt <note>                 Create note without timestamp (bypass config)
  gote                            Open quick note (default)
  gote quick | q                  Open quick note
  gote quick save | qs <name>     Save quick note as named note

Notes:
  gote recent | r [-n <size>]     List recent notes
  gote ro | recent open           Recent + open mode
  gote rd | recent delete         Recent + delete mode
  gote rp | recent pin            Recent + pin mode
  gote info | i <note>            Show note metadata
  gote rename | mv | rn <note> -n <new>  Rename a note

Search:
  gote search | s <query> [-n <size>]  Search notes by title
  gote so | search open <query>   Search + open mode
  gote sd | search delete <query> Search + delete mode
  gote sp | search pin <query>    Search + pin mode
  gote search -t .tag1.tag2       Search by tags
  gote search -w <date> [<date>]  Search by date (created)
  gote search -w <date> --modified  Search by date (modified)

  Date formats: yy, yymm, yymmdd, yymmdd.hh, yymmdd.hhmm, yymmdd.hhmmss
  Examples: -w 24 (year), -w 2412 (month), -w 241223 (day), -w 2412 2501 (range)

Tags:
  gote tag | t                    List all tags
  gote tag | t .tag1.tag2         Filter notes by tags (AND logic)
  gote to | tag open .tag1.tag2   Filter + open mode
  gote td | tag delete .tag1.tag2 Filter + delete mode
  gote tp | tag pin .tag1.tag2    Filter + pin mode
  gote tag popular [-n <limit>]   Show most used tags
  gote tag edit                   Edit tags file
  gote tag format                 Format tags file

Pins:
  gote pin | p <note>             Pin a note
  gote pin                        List pinned notes
  gote pinned | pd [-n <size>]    List pinned notes
  gote po | pinned open           Pinned + open mode
  gote unpin | u | up <note>      Unpin a note

Trash:
  gote delete | d | del <note>    Move note to trash
  gote trash                      List trashed notes
  gote trash <note>               Move note to trash
  gote trash empty                Permanently delete all trash
  gote trash search <query>       Search trashed notes
  gote recover <note>             Restore note from trash

Index:
  gote index | idx                Rebuild the note index
  gote index edit                 Edit index file
  gote index format               Format index file
  gote index clear                Clear and rebuild index from scratch

Config:
  gote config | c                 Show config
  gote config edit                Edit config (uses vim)
  gote config format              Format config file
  gote config help                Show config options

  gote help | h | man             Show this help message
  gote -v | --version             Show version`)
}