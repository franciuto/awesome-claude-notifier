#!/bin/bash
# Build the universal "Claude Notify.app" into <repo>/macos with the bell icon and the notification sounds.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$HERE/.."
ICON_PNG="$ROOT/assets/icon/app-icon.png"
APP="$ROOT/macos/Claude Notify.app"
BUILD="$(mktemp -d)"
trap 'rm -rf "$BUILD"' EXIT
VERSION="$(plutil -extract version raw -o - "$ROOT/.claude-plugin/plugin.json")"

mkdir -p "$BUILD/AppIcon.iconset"
for size in 16 32 128 256 512; do
  sips -z $size $size "$ICON_PNG" --out "$BUILD/AppIcon.iconset/icon_${size}x${size}.png" >/dev/null
  sips -z $((size * 2)) $((size * 2)) "$ICON_PNG" --out "$BUILD/AppIcon.iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$BUILD/AppIcon.iconset" -o "$BUILD/AppIcon.icns"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BUILD/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
cp "$HERE/Info.plist" "$APP/Contents/Info.plist"
# The plugin version is the app version; a new version makes ensureApp reinstall it and macOS drop its cached icon
plutil -replace CFBundleVersion -string "$VERSION" "$APP/Contents/Info.plist"
plutil -replace CFBundleShortVersionString -string "$VERSION" "$APP/Contents/Info.plist"

for arch in arm64 x86_64; do
  swiftc -O -target "$arch-apple-macos13" "$HERE/main.swift" -o "$BUILD/helper-$arch"
done
lipo -create "$BUILD/helper-arm64" "$BUILD/helper-x86_64" -output "$APP/Contents/MacOS/claude-notify-helper"

for mp3 in "$ROOT"/sounds/mp3/*.mp3; do
  slug="$(basename "$mp3" .mp3)"
  afconvert -f AIFF -d BEI16 "$mp3" "$APP/Contents/Resources/claude-notify-$slug.aiff"
done
codesign --force --sign - "$APP"

echo "$APP"
