# Bash completion for gote
_gote_completions()
{
    local cur prev opts notes_dir notes
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="delete index tags search recent pin unpin archive view lint config today popular move mv rename help trash recover pack unpack tag info"

    # Try to get notes dir from config
    notes_dir="$HOME/gotes"
    if [ -f "$HOME/.gote/config.json" ]; then
        ndir=$(grep -o '"notesDir"[ ]*:[ ]*"[^"]*"' "$HOME/.gote/config.json" | sed 's/.*: *"//;s/"//')
        if [ -n "$ndir" ]; then
            notes_dir="$ndir"
        fi
    fi

    # List .md files in notes dir
    notes=""
    if [ -d "$notes_dir" ]; then
        notes=$(find "$notes_dir" -type f -name '*.md' -exec basename {} .md \; 2>/dev/null)
    fi

    case "$prev" in
        delete|view|lint|pin|unpin|archive|info|tag|recover)
            COMPREPLY=( $(compgen -W "$notes" -- "$cur") )
            return 0
            ;;
        move|rename)
            COMPREPLY=( $(compgen -W "$notes" -- "$cur") )
            return 0
            ;;
        *)
            ;;
    esac

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W '--sort' -- ${cur}) )
        return 0
    fi

    COMPREPLY=( $(compgen -W "$opts $notes" -- "$cur") )
    return 0
}
complete -F _gote_completions gote
