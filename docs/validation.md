# Testing and validation

Use this guide to run the test suite, reproduce binding witnesses and package a
Linux AMD64 or ARM64 binary. Go 1.25 or newer and a C compiler are needed for native tests.
CPython and zlib are validation workloads, not dependencies of the CLI.

## Tested environments

Native capture is validated in four environments:

- Linux AMD64, Debian 12 Bookworm, glibc 2.36, Go 1.25.14: native GitHub Actions checks passed.
- Linux AMD64, Debian 13 Trixie, glibc 2.41, Go 1.25.14: native GitHub Actions checks passed.
- Linux ARM64, Debian 12 Bookworm, glibc 2.36, GCC 12.2.0, Go 1.25.14: native GitHub Actions and local checks passed.
- Linux ARM64, Debian 13 Trixie, glibc 2.41, GCC 14.2.0, Go 1.25.14: native GitHub Actions and local checks passed.

[Release run 37238043266](https://github.com/0then0/bindwitness/actions/runs/37238043266)
passed all four native jobs and the release job for tag `v0.1.1`, revision
`df37776646c45e572c2facddf631e1ff56b5d339`. Checks included race tests, vet,
bounded parser fuzzing, native lazy/eager and offline cases, installed/release
binaries, CPython/zlib and both Bookworm two-root zlib jobs. The
[published release](https://github.com/0then0/bindwitness/releases/tag/v0.1.1)
contains the validated AMD64 and ARM64 binaries from that run, packaged without
rebuilding, and a common `SHA256SUMS`.

AMD64 jobs use `ubuntu-24.04`; ARM64 jobs use `ubuntu-24.04-arm`. These are native
[GitHub-hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
The workload executes inside a pinned Debian container; the runner's architecture
does not establish the workload's glibc version. Native Ubuntu 24.04 userspace
is outside this matrix. Other architecture/glibc combinations remain unresolved.

Docker Desktop on an ARM64 Mac can run native ARM64 validation in its Linux VM.
Running a foreign architecture under QEMU or cross-compiling checks different
properties and does not establish native compatibility. Tests running directly
on macOS cover portable parsing and offline evaluation only.

Source builds enable both validated architectures. `validate.sh` also sets
`-X bindwitness/internal/witness.enableAMD64Capture=true` explicitly to record
the build profile. This setting does not replace native execution or change the
workload's loader environment.

[scripts/validation.Dockerfile](../scripts/validation.Dockerfile) pins the images:

- Bookworm: `golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437`.
- Trixie: `golang:1.25-trixie@sha256:2c4c60ef415fbfa5e90300722293bef36c5e63fae17570ce18f580af933dbd73`.
- CPython: `python:3.11-slim-bookworm@sha256:2333bd330d12de02514770b3585cad313644316047cdee24a7acfdece6de6efb`.

The Dockerfile places CPython under `/opt/cpython`. Debian's default interpreter
may have a built-in zlib module; this validation explicitly uses CPython's shared
zlib extension. Saved raw trace provenance includes image references, file hashes
and runtime details. Each validation run also writes OS, compiler, Go and kernel
snapshots alongside its reports.

All three pins provide AMD64 and ARM64 manifests. When changing an image pin,
check its architecture manifests with
`docker buildx imagetools inspect --raw IMAGE_REFERENCE` and rerun the matrix.

## Run the full Linux checks

From a native AMD64 or ARM64 Linux host, or a same-architecture Docker Linux VM:

```sh
case $(uname -m) in
  x86_64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) exit 1 ;;
esac
docker build -f scripts/validation.Dockerfile -t bindwitness-validation:bookworm .
docker run --rm -v "$PWD:/work" -w /work \
  bindwitness-validation:bookworm sh scripts/validate.sh \
  "build/validation/bookworm-$arch" "$arch" 2.36

docker build -f scripts/validation.Dockerfile \
  --build-arg BASE=golang:1.25-trixie@sha256:2c4c60ef415fbfa5e90300722293bef36c5e63fae17570ce18f580af933dbd73 \
  -t bindwitness-validation:trixie .
docker run --rm -v "$PWD:/work" -w /work \
  bindwitness-validation:trixie sh scripts/validate.sh \
  "build/validation/trixie-$arch" "$arch" 2.41
```

The script records and checks the actual architecture, loader userspace and
Go version before running tests. CI also checks the host and Docker daemon
architecture against the matrix, so accidentally running the other architecture
through emulation is not accepted.
Architecture-specific output directories prevent evidence and binaries from
overwriting each other. Each job uploads `reports-<debian>-<architecture>`.

[scripts/validate.sh](../scripts/validate.sh) runs:

```sh
capture_flags='-X bindwitness/internal/witness.enableAMD64Capture=true'
go test -race -ldflags="$capture_flags" ./...
go vet ./...
go test -ldflags="$capture_flags" ./internal/witness -run='^$' -fuzz=FuzzParseTrace -fuzztime=5000x -parallel=2
```

It installs a CLI with `CGO_ENABLED=0`, runs native and CPython/zlib cases, then
builds and exercises a stripped release binary. Reports and configurations are
written to the output directory. Expected negative cases print FAIL or
UNRESOLVED; their validation scripts exit 0 when the expected result is obtained.
[GitHub Actions](../.github/workflows/ci.yml) runs the same matrix and retains
JSON reports and the release binary as artifacts.

For quick checks in either target Linux userspace, run `go test ./...` and
`go vet ./...`. The default profile matches the explicit validation profile.
On macOS, Go skips Linux-only integration tests; parser, evaluator, schema-version
and saved-report checks still run. Use the Linux matrix to check capture behavior.

## Native regression cases

[testdata/native](../testdata/native) contains C source fixtures compiled into
disposable ELF binaries by [build-fixtures.sh](../scripts/build-fixtures.sh).
The suite checks:

- Normal executable and plugin bindings; reversing plugin order changes provider
  even though both implementations produce `result=12` and exit 0.
- Separate symbol names, equal basename/SONAME and explicit symbol versions.
- Required coverage, bounded output, malformed diagnostics and trace loss.
- Unsupported namespaces, reload, extra traced processes and ambiguous aliases.
- Artifact mutation, deadline handling and independent nonzero workload exits.
- Preserved relative/PATH argv[0] and SIGINT/SIGTERM workload cleanup.
- A relative DSO loaded after chdir: identity remains unresolved instead of being
  assigned to a different file in the initial directory.
- The same workload observed fully and with a truncated trace, using an explicit
  `LD_BIND_NOT=1`/`LD_DYNAMIC_WEAK=1` profile: missing provider observations remain
  a coverage gap, while a distinct positive mismatch still remains FAIL.

The ordinary fixture profile retains lazy binding. Explicit `LD_BIND_NOW=1`
cases require the correct provider to PASS and the wrong provider to FAIL with
`PROVIDER_NOT_ALLOWED`, while both print `result=12` and exit 0. The Go integration
test and installed/release binary checks verify the explicit override in both
provenance environment maps and a selected event's raw trace line. Required
coverage means a binding diagnostic was observed, not that a function ran.
The eager missing-required case also returns UNRESOLVED/`REQUIRED_NOT_OBSERVED`
with the same output and successful workload exit.

## CPython and zlib

The external case performs compression/decompression of a short fixed payload
with real CPython 3.11.17 and its shared zlib extension. It checks the extension's
`deflate` and `inflate` bindings. Runtime zlib is 1.2.13 on Bookworm and 1.3.1 on
Trixie. The correct provider passes, an intentionally wrong provider fails, and
an uncovered required `inflateBack` returns UNRESOLVED.

To compare two upstream zlib 1.3.1 builds on Bookworm, with `arch` set as above:

```sh
docker run --rm -v "$PWD:/work" -w /work \
  -e VALIDATION_ARCH="$arch" bindwitness-validation:bookworm sh -c '
    sh scripts/build-zlib-roots.sh "/work/build/zlib-roots-$VALIDATION_ARCH"
    env LD_LIBRARY_PATH=/opt/cpython/lib PYTHONHOME=/opt/cpython \
      /opt/cpython/bin/python3 scripts/zlib-validation.py \
      "build/validation/bookworm-$VALIDATION_ARCH/installed/bindwitness" \
      "build/zlib-comparison-$VALIDATION_ARCH" \
      "build/zlib-roots-$VALIDATION_ARCH/one" "build/zlib-roots-$VALIDATION_ARCH/two"
  '
```

The script downloads the [upstream archive](https://zlib.net/fossils/zlib-1.3.1.tar.gz),
checks SHA-256 `9a93b2b7dfdac77ceba5a558a580e74667dd6fede4585b91eefb60f03b72df23`,
and builds `-O0` and `-O2` installations. Both round trips pass. Explicit
`LD_LIBRARY_PATH` profiles select different logical providers, producing a compare
witness. Mapping both builds to one logical provider instead yields PASS with
`artifact_changes`. This is an integration/configuration case, not a reproduction
of an upstream bug or evidence of external adoption.

CI keeps this two-root check on Bookworm for both AMD64 and ARM64.

## Perl bundled/system zlib load order

The [real integration case](perl-zlib-case-study.md) builds pinned CPAN
Compress::Raw::Zlib 2.103 and 2.105 with bundled zlib 1.2.12 and loads each
alongside Bookworm's system libz. Reversing global load order changes 2.103's
observed providers while preserving a successful round trip and identical stdout.
The upstream `Perl_crz_*` prefix in 2.105 is the negative control. This is not
an exact reproduction of the original AlmaLinux/RHEL failure.

`build-perl-zlib-case.sh` and `perl-zlib-validation.py` use the existing CLI and
reports, run 10 repetitions per explicit variant, replay every check offline,
compare orders, check missing coverage without inferring provider changes, and
require failed mandatory loads to stop the application with the original error.
CI runs this external workload only in the two Bookworm jobs and preserves its
artifacts through the always-upload step. Stored representative reports use
ARM64 / glibc 2.36; each newly captured report records its actual platform.
See the case study for the four scenarios, expected outcomes and reproduction
commands.

## Saved evidence

[testdata/reports](../testdata/reports/README.md) contains representative reports
and their contracts. [testdata/traces](../testdata/traces) contains raw loader
traces and provenance. `TestRepresentativeReports` recomputes saved verdicts;
parser tests and fuzz seeds use the actual raw traces. Absolute paths and PIDs
are retained as historical evidence. Replaying reports is offline; reproducing
live captures requires rebuilding fixtures and using new configurations.

The files in `testdata` are necessary source fixtures, not disposable build
output. Go excludes that directory from ordinary package discovery. Keep it in
the repository; new run outputs belong in ignored `build/` or `dist/`.

## Package a release

After successful validation, from the source checkout:

```sh
sh scripts/package-release.sh build/validation/bookworm-amd64/release/bindwitness amd64
sh scripts/package-release.sh build/validation/bookworm-arm64/release/bindwitness arm64
(cd dist && sha256sum --check SHA256SUMS)
```

This creates `dist/bindwitness-0.1.1-linux-amd64.tar.gz`,
`dist/bindwitness-0.1.1-linux-arm64.tar.gz` and a common `dist/SHA256SUMS`.
Packaging derives the version from the CLI source and requires the adjacent
`version.txt` and binary `SHA256SUMS` written by native validation. It checks the
recorded version and input hash before creating output, without executing a
foreign-architecture binary. It also verifies the input ELF machine against the
supplied architecture and compares the extracted binary to the input.
Packaging another architecture preserves the first archive's checksum.
The archive contains the binary, Apache-2.0 license, README, icon, documentation
and JSON schemas. It does not include development fixtures. Verify the checksum
before extracting a distributed archive, then run `./bindwitness --version` and
check your own contract. Packaging does not publish a GitHub release.

The release workflow runs for version tags and publishes only after all four
native environments pass, including the Bookworm external integration cases.
The release job downloads each validated Bookworm binary
from that same workflow run into a separate directory. It checks CLI/source/tag
versions, ELF machine, archive checksums and extracted binary equality, and
publishes both archives and the common `SHA256SUMS`. It never rebuilds a binary.
Release notes are maintained in [releases/v0.1.1.md](releases/v0.1.1.md).

The [AMD64 consumer example](../examples/github-actions-amd64.yml) downloads the
published v0.1.1 AMD64 archive and common checksum file,
verifies exactly the selected archive, uses the existing explicit native contract
with `required: true`, creates `build/report/` before capture, preserves the JSON
report using `if: always()`, and fails CI on every nonzero BindWitness exit code.
It requires neither a GPU nor another project. To change the workload profile,
edit the contract's explicit command, environment, objects and required selectors.

Short fuzz runs and the declared integration matrix do not establish exhaustive
parser coverage, ABI compatibility or behavior on unexecuted workload paths.
See [report limitations](report-format.md#capture-limitations) before interpreting
results outside the tested scope.
