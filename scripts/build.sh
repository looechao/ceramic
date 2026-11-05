#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
APP="build/Ceramic.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
go build -trimpath -ldflags='-s -w' -o "$APP/Contents/MacOS/Ceramic" .
# Same sizes MyGo bundles: the 16 and 32 px 1x entries are left out because
# IconServices draws their PNG payloads as noise; macOS scales down from 2x.
ICONSET="$(mktemp -d)/Ceramic.iconset"
mkdir -p "$ICONSET"
for entry in 16x16@2x:32 32x32@2x:64 128x128:128 128x128@2x:256 256x256:256 256x256@2x:512 512x512:512 512x512@2x:1024; do
  sips -z "${entry#*:}" "${entry#*:}" resources/icon.png --out "$ICONSET/icon_${entry%:*}.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/Ceramic.icns"
rm -rf "$(dirname "$ICONSET")"
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleName</key><string>Ceramic</string><key>CFBundleDisplayName</key><string>Ceramic</string><key>CFBundleIdentifier</key><string>dev.ceramic.reader</string><key>CFBundleExecutable</key><string>Ceramic</string><key>CFBundleIconFile</key><string>Ceramic</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleShortVersionString</key><string>0.1.0</string><key>CFBundleVersion</key><string>1</string><key>NSHighResolutionCapable</key><true/><key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict></dict></plist>
PLIST
codesign --force --deep --sign - "$APP"
printf 'Built %s\n' "$APP"
