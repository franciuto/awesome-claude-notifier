#!/bin/bash
# Make freshly generated sounds gentle: a 100 ms fade-in instead of a hard attack, then the
# loudness capped at -26 LUFS (quieter sounds keep their level) and peaks limited to -9 dBFS.
# Run it once per new file (generate.sh does): the fade adds up if applied twice. Needs ffmpeg.
# Usage: soften.sh file.mp3 ...
set -euo pipefail

loudness() {
  ffmpeg -hide_banner -nostats -i "$1" -af ebur128 -f null - 2>&1 | awk '/Summary/{s=1} s&&/I:/{print $2; exit}'
}

for f in "$@"; do
  tmp="$(mktemp -d)"
  ffmpeg -hide_banner -loglevel error -y -i "$f" -af "afade=t=in:st=0:d=0.1:curve=qsin" -c:a pcm_s16le "$tmp/fade.wav"
  # Back to the original loudness, or -26 LUFS if it was louder, since the fade takes some energy away
  gain=$(awk -v orig="$(loudness "$f")" -v faded="$(loudness "$tmp/fade.wav")" \
    'BEGIN { target = (orig < -26) ? orig : -26; print target - faded }')
  ffmpeg -hide_banner -loglevel error -y -i "$tmp/fade.wav" \
    -af "volume=${gain}dB,alimiter=limit=0.35:attack=5:release=50:level=false" -c:a libmp3lame -q:a 2 "$tmp/out.mp3"
  mv "$tmp/out.mp3" "$f"
  rm -rf "$tmp"
  echo "softened $f"
done
