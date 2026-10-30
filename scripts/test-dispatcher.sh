#!/bin/sh
# Dispatcher test: right binary runs with arguments intact (path with a space), missing binary exits 0 silently.
set -eu
SRC="$(cd "$(dirname "$0")/.." && pwd)"
ROOT="$(mktemp -d)/plugin root"
trap 'rm -rf "$(dirname "$ROOT")"' EXIT
mkdir -p "$ROOT/bin/darwin" "$ROOT/bin/linux-amd64" "$ROOT/bin/linux-arm64"
cp "$SRC/bin/claude-notify" "$ROOT/bin/claude-notify"
for d in darwin linux-amd64 linux-arm64; do
  printf '#!/bin/sh\nprintf "%%s\\n" "$0" "$@"\n' > "$ROOT/bin/$d/claude-notify"
  chmod +x "$ROOT/bin/$d/claude-notify"
done
case "$(uname -s)" in
  Darwin) want=darwin ;;
  Linux) case "$(uname -m)" in aarch64|arm64) want=linux-arm64 ;; *) want=linux-amd64 ;; esac ;;
  *) echo "unsupported host"; exit 1 ;;
esac

out="$("$ROOT/bin/claude-notify" hook --event-type "Stop now")"
expected="$ROOT/bin/$want/claude-notify
hook
--event-type
Stop now"
[ "$out" = "$expected" ] || { echo "FAIL: wrong output:"; echo "$out"; exit 1; }

rm "$ROOT/bin/$want/claude-notify"
rc=0
out="$("$ROOT/bin/claude-notify" hook 2>&1)" || rc=$?
[ "$rc" = 0 ] && [ -z "$out" ] || { echo "FAIL: missing binary rc=$rc out=$out"; exit 1; }
echo PASS
