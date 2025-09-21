package cli

import "fmt"

func HelpCommand(args []string) {
	PrintDefaultHelp()
}

func PrintDefaultHelp() {
	fmt.Println(`gote: A simple, extensible CLI note-taking tool

Usage:
  gote <note name>                Create or open a note
  gote                            (no args) runs quick note
  gote quick | q                  Create/open a quick note
  gote recent | r                 Show recently modified notes
  gote index | idx                Rebuild the note index
  gote tags | ts                  List all tags
  gote tag | t <tag>              Show notes with a tag
  gote config | c                 Edit or show config
  gote search | s <query>         Search notes by title
  gote search -t <tag> ...        Search notes by tags
  gote search trash <query>       Search trashed notes
  gote pin | p <note>             Pin a note
  gote pins | pinned | pd         List pinned notes
  gote unpin | u <note>           Unpin a note
  gote delete | d <note>          Move note to trash
  gote recover <note>             Restore note from trash
  gote rename <note> -n <new>     Rename a note
  gote info | i <note>            Show note metadata
  gote help | h                   Show this help message

Example:
  gote
  gote mynote
  gote quick
  gote search project
  gote pin mynote
  gote rename mynote -n project-notes
`)
}