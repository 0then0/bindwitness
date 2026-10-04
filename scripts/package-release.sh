#!/bin/sh
# Package an already validated Linux binary; never builds or publishes remotely.
set -eu
binary=${1:?usage: package-release.sh VALIDATED_BINARY ARCH [OUTPUT_DIR]}
arch=${2:?usage: package-release.sh VALIDATED_BINARY ARCH [OUTPUT_DIR]}
out=${3:-dist}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$root/internal/witness/types.go")
case "$version" in
  ''|*[!0-9.]*) echo 'Invalid CLI version' >&2; exit 1 ;;
esac
case "$arch" in
  amd64) machine=3e00 ;;
  arm64) machine=b700 ;;
  *) echo 'Release architecture must be amd64 or arm64' >&2; exit 1 ;;
esac
test -f "$binary"
metadata=$(dirname -- "$binary")
if [ ! -f "$metadata/version.txt" ] || [ ! -f "$metadata/SHA256SUMS" ]; then
  echo 'Validated binary requires adjacent version.txt and SHA256SUMS' >&2
  exit 1
fi
if [ "$(cat "$metadata/version.txt")" != "bindwitness $version" ]; then
  echo "Validated binary version does not match source version $version" >&2
  exit 1
fi
expected_hash=$(BINARY_NAME="$(basename -- "$binary")" awk \
  'substr($0, 67) == ENVIRON["BINARY_NAME"] && (substr($0, 65, 2) == "  " || substr($0, 65, 2) == " *") {print substr($0, 1, 64)}' \
  "$metadata/SHA256SUMS")
case "$expected_hash" in
  ''|*[!0-9a-f]*) echo 'Missing or invalid validated binary checksum' >&2; exit 1 ;;
esac
test "${#expected_hash}" -eq 64
if command -v sha256sum >/dev/null 2>&1; then
  actual_hash=$(sha256sum <"$binary" | awk '{print $1}')
else
  actual_hash=$(shasum -a 256 <"$binary" | awk '{print $1}')
fi
if [ "$actual_hash" != "$expected_hash" ]; then
  echo 'Validated binary checksum does not match' >&2
  exit 1
fi
# ELF64, little-endian, current ELF version and the requested e_machine.
# Reading the header works on either host architecture without executing it.
header=$(od -An -tx1 -N7 "$binary" | tr -d ' \n')
test "$header" = 7f454c46020101
actual_machine=$(od -An -tx1 -j18 -N2 "$binary" | tr -d ' \n')
if [ "$actual_machine" != "$machine" ]; then
  echo "Binary ELF machine does not match linux/$arch" >&2
  exit 1
fi
mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT HUP INT TERM
name=bindwitness-$version-linux-$arch
mkdir -p "$stage/$name"
cp "$binary" "$stage/$name/bindwitness"
cp "$root/LICENSE" "$root/README.md" "$stage/$name/"
cp -R "$root/docs" "$stage/$name/docs"
tar czf "$out/$name.tar.gz" -C "$stage" "$name"
mkdir "$stage/extracted"
tar xzf "$out/$name.tar.gz" -C "$stage/extracted"
cmp "$binary" "$stage/extracted/$name/bindwitness"
# Replace this archive's entry while preserving entries for other architectures.
checksums="$out/.SHA256SUMS.$$"
if [ -f "$out/SHA256SUMS" ]; then
  awk -v archive="$name.tar.gz" '$2 != archive' "$out/SHA256SUMS" >"$checksums"
else
  : >"$checksums"
fi
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$out" && sha256sum "$name.tar.gz") >>"$checksums"
else
  (cd "$out" && shasum -a 256 "$name.tar.gz") >>"$checksums"
fi
LC_ALL=C sort -k2 "$checksums" >"$stage/SHA256SUMS"
mv "$stage/SHA256SUMS" "$out/SHA256SUMS"
rm "$checksums"
