#!/bin/bash
# Install the bundled sounds into ~/Library/Sounds as claude-notify-<group>.aiff,
# where macOS notifications can find them by name.
# Usage: install.sh [group ...]   (default: all groups)
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$HOME/Library/Sounds"

groups=("$@")
[ ${#groups[@]} -eq 0 ] && groups=(needs-you done paused error limit info)

for g in "${groups[@]}"; do
  afconvert -f AIFF -d BEI16 "$HERE/mp3/$g.mp3" "$HOME/Library/Sounds/claude-notify-$g.aiff"
  echo "installed ~/Library/Sounds/claude-notify-$g.aiff"
done
