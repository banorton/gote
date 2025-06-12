# Zsh completion for note names for the gote CLI
gote_note_completion() {
  local notes_dir
  if [[ -f "$HOME/.gote/config.json" ]]; then
    notes_dir=$(grep -o '"notesDir": *"[^"]*"' "$HOME/.gote/config.json" | cut -d'"' -f4)
  fi
  if [[ -z "$notes_dir" ]]; then
    notes_dir="$HOME/gotes"
  fi
  local -a files
  files=(${(f)"$(find "$notes_dir" -type f -name '*.md' 2>/dev/null | sed "s|$notes_dir/||;s|\.md$||")"})
  _describe 'note' files
}

compdef gote_note_completion gote
