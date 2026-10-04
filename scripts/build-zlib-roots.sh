#!/bin/sh
set -eu
out=${1:?usage: build-zlib-roots.sh ABSOLUTE_OUTPUT_DIR [source.tar.gz]}
mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)
archive=${2:-"$out/zlib-1.3.1.tar.gz"}
if [ ! -f "$archive" ]; then
  curl -fL --max-time 120 https://zlib.net/fossils/zlib-1.3.1.tar.gz -o "$archive"
fi
printf '%s  %s\n' 9a93b2b7dfdac77ceba5a558a580e74667dd6fede4585b91eefb60f03b72df23 "$archive" | sha256sum -c -
for mode in one two; do
  mkdir -p "$out/src-$mode"
  tar xzf "$archive" -C "$out/src-$mode" --strip-components=1
  (
    cd "$out/src-$mode"
    flags=-O0
    if [ "$mode" = two ]; then flags=-O2; fi
    CFLAGS="$flags" ./configure --prefix="$out/$mode" >"$out/configure-$mode.log"
    make -j2 >"$out/build-$mode.log"
    make install >"$out/install-$mode.log"
  )
done
printf '%s\n' 'Built upstream zlib 1.3.1 in one/ and two/'
