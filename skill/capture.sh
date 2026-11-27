#!/usr/bin/env bash
# Runs every documented example against skill/fixture and prints one block per
# command: a "$ gote ..." line, then the output. SKILL.md blocks must match.
# usage: skill/capture.sh <path-to-gote-binary>
set -euo pipefail

BIN=$(cd "$(dirname "${1:?usage: capture.sh <gote-binary>}")" && pwd)/$(basename "$1")
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

NOTES="$WORK/notes"
cp -R "$HERE/fixture" "$NOTES"
mkdir -p "$WORK/config/gote" "$WORK/state"
printf 'notes_dir: %s\nserver: false\neditor: "true"\n' "$NOTES" > "$WORK/config/gote/config.yaml"

# Fixed mtimes: ordering and `modified:` filters must not depend on when this runs.
t=2611010900
for f in ideas.md design.md lonely.md projects/gote.md \
  diagrams/261101120000_diagram_architecture.excalidraw \
  diagrams/261101130000_diagram_broken.excalidraw; do
  touch -t "$t" "$NOTES/$f"
  t=$((t + 100))
done

export TZ=UTC XDG_CONFIG_HOME="$WORK/config" XDG_STATE_HOME="$WORK/state" EDITOR=true

CMDS=(
  "search --json desi"
  "search -t --json socket"
  "search -t --json tag:go -tag:project"
  "search -t --json created:2026-11"
  "search --json tag:project gote"
  "search -t modified:2026-11 -tag:ideas"
  "tasks"
  "tasks --all --json tag:go"
  "info --json gote"
  "tag --json"
  "links --json gote"
  "backlinks --json ideas"
  "graph gote -d 2"
  "orphans --json"
  "broken --json"
  "search -t --json kind:diagram architecture"
  "diagram list --json"
  "diagram read --json 261101120000_diagram_architecture"
  "diagram check 261101120000_diagram_architecture"
  "diagram check 261101130000_diagram_broken"
  "diagram search --json arch"
  "diagram search -t --json cache"
  "diagram describe 261101120000_diagram_architecture"
  "diagram describe --json 261101130000_diagram_broken Sample with deliberately broken bindings"
  "rename --json ideas backlog"
  "backlinks backlog"
)

for line in "${CMDS[@]}"; do
  echo '%%%'
  echo "\$ gote $line"
  # shellcheck disable=SC2086
  "$BIN" --no-server $line 2>&1 | sed "s|$NOTES|~/notes|g" || true
done
