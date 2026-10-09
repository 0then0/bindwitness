#!/bin/sh
# Real CPAN sources, built only in a new disposable directory. No installation.
set -eu
out=${1:?usage: build-perl-zlib-case.sh NEW_OUTPUT_DIR}
if [ -e "$out" ]; then
  echo "Refusing to reuse build directory: $out" >&2
  exit 1
fi
test "$(uname -s)" = Linux || { echo 'Requires Linux/glibc, not a skipped validation' >&2; exit 1; }
mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)
for version in 2.103 2.105; do
  case "$version" in
    2.103) checksum=d69d2620ca024dc1b424f7f4228fe1169b2b77416bb30310e890d3be7da47dff ;;
    2.105) checksum=228159574899c56fe616c4dc889bddcf41db6079e095a2d622af96b043bbe7d8 ;;
  esac
  archive="$out/Compress-Raw-Zlib-$version.tar.gz"
  url="https://www.cpan.org/authors/id/P/PM/PMQS/Compress-Raw-Zlib-$version.tar.gz"
  printf '%s\n' "$url" >"$out/source-$version.url"
  curl -fL --retry 2 --max-time 120 "$url" -o "$archive"
  printf '%s  %s\n' "$checksum" "$archive" | sha256sum -c -
  tar xzf "$archive" -C "$out"
  (
    cd "$out/Compress-Raw-Zlib-$version"
    # Select the vendored source explicitly. 2.105 enables its upstream prefix.
    env BUILD_ZLIB=1 ZLIB_INCLUDE=./zlib-src ZLIB_LIB=./zlib-src \
      perl Makefile.PL OPTIMIZE=-O2 >"$out/configure-$version.log" 2>&1
    make -j2 >"$out/build-$version.log" 2>&1
    make test >"$out/test-$version.log" 2>&1
    cp Makefile "$out/Makefile-$version"
    readelf -h -d -n -Ws blib/arch/auto/Compress/Raw/Zlib/Zlib.so >"$out/elf-$version.txt"
    sha256sum blib/arch/auto/Compress/Raw/Zlib/Zlib.so >"$out/artifact-$version.sha256"
  )
done
perl -V >"$out/perl.txt"
cc --version >"$out/compiler.txt"
ld --version >"$out/linker.txt"
getconf GNU_LIBC_VERSION >"$out/glibc.txt"
uname -a >"$out/kernel.txt"
cat /etc/os-release >"$out/os-release.txt"
printf 'image=%s\nrunner=%s\nBUILD_ZLIB=1\nZLIB_INCLUDE=./zlib-src\nZLIB_LIB=./zlib-src\nOPTIMIZE=-O2\n' \
  "${VALIDATION_IMAGE:-unspecified}" "${VALIDATION_RUNNER:-local}" >"$out/environment.txt"
