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
  gote                            Open quick note (default)
  gote quick | q                  Open quick note

Notes:
  gote recent | r [-n <size>]     List recent notes
  gote ro [-n <size>]             Recent + open mode
  gote rd [-n <size>]             Recent + delete mode
  gote rp [-n <size>]             Recent + pin mode
  gote info | i <note>            Show note metadata
  gote rename | mv | rn <note> -n <new>  Rename a note

Search:
  gote search | s <query> [-n <size>]  Search notes by title
  gote so <query> [-n <size>]     Search + open mode
  gote sd <query> [-n <size>]     Search + delete mode
  gote sp <query> [-n <size>]     Search + pin mode
  gote search -t <tag1> <tag2>    Search by tags

Tags:
  gote tags | ts                  List all tags
  gote tags popular [-n <limit>]  Show most used tags
  gote tags edit                  Edit tags file
  gote tags format                Format tags file
  gote tag | t <note> -t <tags>   Add tags to a note

Pins:
  gote pin | p <note>             Pin a note
  gote pin                        List pinned notes
  gote pinned | pd [-n <size>]    List pinned notes
  gote po [-n <size>]             Pinned + open mode
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