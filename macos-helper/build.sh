#!/bin/bash
# Build "Claude Notify.app" into ~/Applications with the bell icon.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ICON_PNG="$HERE/../assets/icon/app-icon.png"
APP="$HOME/Applications/Claude Notify.app"
BUILD="$(mktemp -d)"

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
# New build number each time, so macOS drops its cached icon for the old one
plutil -replace CFBundleVersion -string "$(date +%Y%m%d%H%M%S)" "$APP/Contents/Info.plist"
swiftc -O "$HERE/main.swift" -o "$APP/Contents/MacOS/claude-notify-helper"
codesign --force --sign - "$APP"

rm -rf "$BUILD"
echo "$APP"
