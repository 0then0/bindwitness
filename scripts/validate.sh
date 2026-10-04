#!/bin/sh
# Run inside scripts/validation.Dockerfile, with /work mounted from this repository.
set -eu
out=${1:-build/validation/local}
mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)
getconf GNU_LIBC_VERSION >"$out/glibc.txt"
uname -a >"$out/kernel.txt"
cat /etc/os-release >"$out/os-release.txt"
go version >"$out/go.txt"
cc --version >"$out/cc.txt"
go test -race ./...
go vet ./...
# Bound by execution count to avoid campaign-deadline shutdown races in Go fuzzing.
go test ./internal/witness -run='^$' -fuzz=FuzzParseTrace -fuzztime=5000x -parallel=2
GOBIN="$out/installed" CGO_ENABLED=0 go install -trimpath ./cmd/bindwitness
binary="$out/installed/bindwitness"
"$binary" --version
python3 scripts/native-validation.py "$binary" "$out/native"
env LD_LIBRARY_PATH=/opt/cpython/lib PYTHONHOME=/opt/cpython /opt/cpython/bin/python3 scripts/zlib-validation.py "$binary" "$out/zlib"
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$out/release/bindwitness" ./cmd/bindwitness
"$out/release/bindwitness" --version
python3 scripts/native-validation.py "$out/release/bindwitness" "$out/release-smoke"
