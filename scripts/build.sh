#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
APP="build/Sprinkle.app"
mkdir -p "$APP/Contents/MacOS"
go build -trimpath -ldflags='-s -w' -o "$APP/Contents/MacOS/Sprinkle" .
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleName</key><string>Sprinkle</string><key>CFBundleDisplayName</key><string>Sprinkle</string><key>CFBundleIdentifier</key><string>dev.sprinkle.reader</string><key>CFBundleExecutable</key><string>Sprinkle</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleShortVersionString</key><string>0.1.0</string><key>CFBundleVersion</key><string>1</string><key>NSHighResolutionCapable</key><true/><key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict></dict></plist>
PLIST
codesign --force --deep --sign - "$APP"
printf 'Built %s\n' "$APP"
