#!/bin/sh
# Run inside scripts/validation.Dockerfile, with /work mounted from this repository.
set -eu
out=${1:-build/validation/local}
expected_arch=${2:?usage: validate.sh OUTPUT_DIR EXPECTED_ARCH EXPECTED_GLIBC [EXPECTED_GO]}
expected_glibc=${3:?usage: validate.sh OUTPUT_DIR EXPECTED_ARCH EXPECTED_GLIBC [EXPECTED_GO]}
expected_go=${4:-1.25.14}
# Candidate capture support is published only after the whole native matrix passes.
capture_flags='-X bindwitness/internal/witness.enableAMD64Capture=true'
mkdir -p "$out"
out=$(CDPATH= cd -- "$out" && pwd)
getconf GNU_LIBC_VERSION >"$out/glibc.txt"
uname -a >"$out/kernel.txt"
uname -m >"$out/architecture.txt"
cat /etc/os-release >"$out/os-release.txt"
go version >"$out/go.txt"
go env GOHOSTOS GOHOSTARCH GOOS GOARCH GOVERSION >"$out/go-environment.txt"
cc --version >"$out/cc.txt"
case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo 'Validation requires native Linux AMD64 or ARM64' >&2; exit 1 ;;
esac
glibc=$(getconf GNU_LIBC_VERSION)
go_version=$(go env GOVERSION)
printf 'architecture=%s\nglibc=%s\ngo=%s\nexpected_architecture=%s\nexpected_glibc=%s\nexpected_go=%s\nimage=%s\nrunner=%s\ncapture_profile=native-matrix-candidate\n' \
  "$arch" "$glibc" "$go_version" "$expected_arch" "$expected_glibc" "$expected_go" \
  "${VALIDATION_IMAGE:-local}" "${VALIDATION_RUNNER:-local}" >"$out/environment.txt"
test "$(uname -s)" = Linux
test "$arch" = "$expected_arch"
case "$expected_glibc" in
  2.36|2.41) ;;
  *) echo 'Validation only covers glibc 2.36 and 2.41' >&2; exit 1 ;;
esac
test "$glibc" = "glibc $expected_glibc"
test "$go_version" = "go$expected_go"
test "$(go env GOHOSTOS)" = linux
test "$(go env GOOS)" = linux
test "$(go env GOHOSTARCH)" = "$expected_arch"
test "$(go env GOARCH)" = "$expected_arch"
go test -race -ldflags="$capture_flags" ./...
go vet ./...
# Bound by execution count to avoid campaign-deadline shutdown races in Go fuzzing.
go test -ldflags="$capture_flags" ./internal/witness -run='^$' -fuzz=FuzzParseTrace -fuzztime=5000x -parallel=2
GOBIN="$out/installed" CGO_ENABLED=0 go install -trimpath -ldflags="$capture_flags" ./cmd/bindwitness
binary="$out/installed/bindwitness"
"$binary" --version
python3 scripts/native-validation.py "$binary" "$out/native"
env LD_LIBRARY_PATH=/opt/cpython/lib PYTHONHOME=/opt/cpython /opt/cpython/bin/python3 scripts/zlib-validation.py "$binary" "$out/zlib"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w $capture_flags" -o "$out/release/bindwitness" ./cmd/bindwitness
"$out/release/bindwitness" --version >"$out/release/version.txt"
cat "$out/release/version.txt"
readelf -h "$out/release/bindwitness" >"$out/release/elf.txt"
python3 scripts/native-validation.py "$out/release/bindwitness" "$out/release-smoke"
env LD_LIBRARY_PATH=/opt/cpython/lib PYTHONHOME=/opt/cpython /opt/cpython/bin/python3 scripts/zlib-validation.py "$out/release/bindwitness" "$out/release-zlib"
cp "$out/environment.txt" "$out/release/environment.txt"
(cd "$out/release" && sha256sum bindwitness > SHA256SUMS)
