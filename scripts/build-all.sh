#!/usr/bin/env bash
# Cross-compile tickr. Output names use plain-English aliases
# so it's obvious which file to download (e.g. "mac-m" for Apple Silicon).
set -euo pipefail
cd "$(dirname "$0")/.."

LDFLAGS="-s -w"
mkdir -p dist
rm -f dist/*

build() {
  local os=$1 arch=$2 name=$3 ext=${4:-}
  local out="dist/tickr-${name}${ext}"
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 \
    go build -trimpath -ldflags="$LDFLAGS" -o "$out" ./cmd/tickr
  echo "built $out"
}

build darwin  arm64  mac-m
build darwin  amd64  mac-intel
build linux   amd64  linux
build linux   arm64  linux-arm
build windows amd64  windows      .exe
build windows arm64  windows-arm  .exe

echo
ls -lh dist/
