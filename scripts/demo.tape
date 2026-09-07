# Records assets/demo.gif. Requires vhs (https://github.com/charmbracelet/vhs):
#   brew install vhs && vhs scripts/demo.tape
#
# Run against a scratch note directory so a recording never touches real notes:
#   HOME=$(mktemp -d) vhs scripts/demo.tape

Output assets/demo.gif

Set FontSize 18
Set Width 1000
Set Height 620
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "gote standup -t meeting"    Sleep 500ms  Enter  Sleep 2s
Type ":q"                          Enter        Sleep 1s

Type "gote r"                      Sleep 500ms  Enter  Sleep 2s
Type "q"                           Sleep 1s

Type "gote s bm25"                 Sleep 500ms  Enter  Sleep 2s
Type "q"                           Sleep 1s

Type "gote t .work"                Sleep 500ms  Enter  Sleep 2s
Type "q"                           Sleep 1s

Type "gote p"                      Sleep 500ms  Enter  Sleep 2s
Type "q"                           Sleep 2s
