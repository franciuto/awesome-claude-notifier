#!/bin/bash
# Regenerate notification sounds with ElevenLabs (needs an authenticated `elevenlabs` CLI)
# into sounds/mp3/<group>.mp3. Then run the Release workflow, which builds them into the app.
# Usage: generate.sh [group ...]   (default: all groups)
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$HERE/mp3"

prompt_for() {
  case "$1" in
    needs-you) echo "Short bright two-note rising notification chime, clean glass bell tones, asks for attention, friendly, modern UI sound, no reverb tail" ;;
    done)      echo "Short satisfying task-complete chime, warm marimba three-note ascending resolve, soft and pleasant, modern UI success sound" ;;
    paused)    echo "Single soft muted wooden tick followed by a gentle low bell, calm neutral UI status sound, very short" ;;
    error)     echo "Short soft descending two-note error tone, low muted synth, gentle but clearly negative, modern UI alert, not harsh" ;;
    limit)     echo "Short soft clock tick-tock then a quiet airy chime, waiting or paused state, calm modern UI sound" ;;
    info)      echo "Tiny soft bubble pop notification, very short, subtle modern UI sound" ;;
    *) echo "unknown group: $1" >&2; exit 1 ;;
  esac
}

groups=("$@")
[ ${#groups[@]} -eq 0 ] && groups=(needs-you done paused error limit info)

for g in "${groups[@]}"; do
  body=$(printf '{"text": %s, "duration_seconds": 1.2, "prompt_influence": 0.7}' \
    "$(prompt_for "$g" | /usr/bin/python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().strip()))')")
  elevenlabs text-to-sound-effects convert --json "$body" -o "$HERE/mp3/$g.mp3" -q
  "$HERE/soften.sh" "$HERE/mp3/$g.mp3"
  echo "generated sounds/mp3/$g.mp3"
done
