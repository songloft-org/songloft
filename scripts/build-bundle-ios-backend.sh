#!/usr/bin/env bash
set -euo pipefail

asset=$(cd "${1:?expected a staged iOS FFmpeg asset}" && pwd)
output=${2:?expected the Songloft.xcframework output path}
xcconfig=${3:?expected the Xcode build settings output path}
ldflags=${4:-}
backend=$(cd "$(dirname "$0")/.." && pwd)
staging="$backend/internal/mediaexec/ffmpeg"
archive="$asset/libsongloft_ffmpeg.a"
test -f "$asset/include/libavformat/avformat.h"
test -f "$archive"
link_flags=$(cat "$asset/link-flags.txt")
# This is the part of the native link contract that the app's final link needs.
test "$link_flags" = '-lc++ -framework Security -framework CoreFoundation'
xcrun lipo -verify_arch arm64 "$archive"

if [[ -e "$staging" || -L "$staging" ]]; then
  echo "FFmpeg staging path already exists: $staging" >&2
  exit 1
fi
ln -s "$asset" "$staging"
trap 'unlink "$staging"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
cd "$backend"
gomobile bind -v -target=ios/arm64 -iosversion=15.0 -tags lite,bundle_ffmpeg \
  -ldflags="$ldflags" -o "$output" ./mobile
framework="$output/ios-arm64/Songloft.framework/Songloft"
test -f "$framework"
xcrun --sdk iphoneos libtool -static -arch_only arm64 -o "$framework.merged" "$framework" "$archive"
mv "$framework.merged" "$framework"
printf '%s\n' \
  'SWIFT_ACTIVE_COMPILATION_CONDITIONS = $(inherited) HAS_SONGLOFT_BACKEND' \
  'FRAMEWORK_SEARCH_PATHS = $(inherited) "$(SRCROOT)/Songloft.xcframework/ios-arm64"' \
  "OTHER_LDFLAGS = \$(inherited) -framework Songloft $link_flags" > "$xcconfig"
