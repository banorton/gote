package cli

import "fmt"

func HelpCommand(args []string) {
	PrintDefaultHelp()
}

func PrintDefaultHelp() {
	fmt.Println(`gote - CLI note-taking tool

Usage:
  gote <note>                     Create or open note
  gote <note> -t [template]       Create from template
  gote -d/-dt/-nt <note>          Date/datetime/no-timestamp prefix
  gote                            Open quick note
  gote quick save <name> | qs     Save quick note as named note

Recent: (gote recent | r)
  gote recent [-n size]           List recent notes
  gote ro/rd/rp/rv                + open/delete/pin/view mode

Search: (gote search | s)
  gote search <query>             Search by title
  gote so/sd/sp/sv <query>        + open/delete/pin/view mode
  gote search -t .tag1.tag2       Search by tags
  gote search -w <date> [date]    Search by date (created)
  gote search -w <date> -m        Search by date (modified)

Tags: (gote tag | t)
  gote tag                        List all tags
  gote tag .tag1.tag2             Filter by tags
  gote to/td/tp/tv .tags          + open/delete/pin/view mode
  gote tag popular                Most used tags

Pins: (gote pin | p)
  gote pin <note>                 Pin a note
  gote pin                        Interactive pinned menu
  gote po/pv/pu                   + open/view/unpin mode
  gote unpin | u <note>           Unpin a note

Trash: (gote delete | d)
  gote delete <note>              Move to trash
  gote trash                      List trash
  gote trash empty                Empty trash
  gote recover <note>             Restore from trash

Other:
  gote get | g                    Interactive select
  gote template | tmpl [name]     List/edit templates
  gote index | idx                Rebuild index
  gote config | c                 Show config
  gote config edit | ce           Edit config
  gote info | i <note>            Note metadata
  gote view <note>                Preview in browser
  gote rename | mv <note> -n <new>  Rename note
  gote help | h                   Show this help
  gote -v                         Show version`)
}