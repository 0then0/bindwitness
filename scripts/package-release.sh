#!/bin/sh
# Package an already validated Linux ARM64 binary; never publishes remotely.
set -eu
binary=${1:?usage: package-release.sh VALIDATED_BINARY [OUTPUT_DIR]}
out=${2:-dist}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT HUP INT TERM
name=bindwitness-0.1.0-linux-arm64
mkdir -p "$stage/$name"
cp "$binary" "$stage/$name/bindwitness"
cp "$root/LICENSE" "$root/README.md" "$stage/$name/"
cp -R "$root/docs" "$stage/$name/docs"
tar czf "$out/$name.tar.gz" -C "$stage" "$name"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$out" && sha256sum "$name.tar.gz" > SHA256SUMS)
else
  (cd "$out" && shasum -a 256 "$name.tar.gz" > SHA256SUMS)
fi
