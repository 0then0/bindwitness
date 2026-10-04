# Testing and validation

Use this guide to run the test suite, reproduce binding witnesses and package a
Linux AMD64 or ARM64 binary. Go 1.25 or newer and a C compiler are needed for native tests.
CPython and zlib are validation workloads, not dependencies of the CLI.

## Tested environments

The v0.1.1 matrix has four native jobs using the same validation scripts:

- Linux AMD64, Debian 12 Bookworm, glibc 2.36, Go 1.25.14: native verification pending.
- Linux AMD64, Debian 13 Trixie, glibc 2.41, Go 1.25.14: native verification pending.
- Linux ARM64, Debian 12 Bookworm, glibc 2.36, GCC 12.2.0, Go 1.25.14: local native checks passed.
- Linux ARM64, Debian 13 Trixie, glibc 2.41, GCC 14.2.0, Go 1.25.14: local native checks passed.

AMD64 jobs use `ubuntu-24.04`; ARM64 jobs use `ubuntu-24.04-arm`. These are native
[GitHub-hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
The workload executes inside a pinned Debian container; the runner's architecture
does not establish the workload's glibc version. Native Ubuntu 24.04 userspace
is outside this matrix. Other architecture/glibc combinations remain unresolved.

Local Linux ARM64 validation uses Docker Desktop on a macOS ARM64 host, with
two userspaces on the same ARM64 VM kernel. No AMD64 native runner is available
in this development session; neither a cross-build nor QEMU establishes native
validation. macOS ARM64 checks offline analysis only. Publishing v0.1.1 requires
the entire native four-job GitHub Actions matrix to pass.

Ordinary source builds leave AMD64 capture unresolved. To exercise the proposed
AMD64 combinations, `validate.sh` explicitly builds test/install/release
candidates with `-X bindwitness/internal/witness.enableAMD64Capture=true`, on
both architectures so released offline tools can evaluate either platform.
This build setting enables testing; it is not a validation result. Candidates
must not be published until all four native jobs succeed. No runtime environment
override automatically enables this setting or changes the binding profile.

[scripts/validation.Dockerfile](../scripts/validation.Dockerfile) pins the images:

- Bookworm: `golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437`.
- Trixie: `golang:1.25-trixie@sha256:2c4c60ef415fbfa5e90300722293bef36c5e63fae17570ce18f580af933dbd73`.
- CPython: `python:3.11-slim-bookworm@sha256:2333bd330d12de02514770b3585cad313644316047cdee24a7acfdece6de6efb`.

The Dockerfile places CPython under `/opt/cpython`. Debian's default interpreter
may have a built-in zlib module; this validation explicitly uses CPython's shared
zlib extension. Saved raw trace provenance includes image references, file hashes
and runtime details. Each validation run also writes OS, compiler, Go and kernel
snapshots alongside its reports.

All three existing pinned digests were inspected in the registry and are OCI
multiarchitecture indexes containing `linux/amd64` and `linux/arm64` manifests.
No pins or component versions were changed. Recheck the immutable indexes with
`docker buildx imagetools inspect --raw IMAGE_REFERENCE`; selecting a runner alone
is not a check that an arbitrary digest supports both architectures.

## Verification of this working tree

On 2026-10-05, both ARM64 combinations passed in native Docker Desktop Linux
containers on an ARM64 VM, using the unchanged pinned images and Go 1.25.14:

- `go test -race`, `go vet` and parser fuzzing with 5000 executions per userspace.
- Native lazy and explicit eager profiles: correct provider PASS, wrong provider
  FAIL/`PROVIDER_NOT_ALLOWED`, missing required binding UNRESOLVED. All eager
  cases preserve `result=12`, workload exit 0 and explicit linker provenance.
- Offline evaluation, provider comparison, argv[0], deadlines, SIGINT/SIGTERM,
  relative DSO identities, ambiguous aliases and partial provider coverage.
- Saved legacy ELF32/x32 and foreign-machine identities remain UNRESOLVED;
  proven wrong-provider and provider-change witnesses retain FAIL. Unused
  declared objects and out-of-scope bindings do not establish the platform.
- Installed and stripped release candidates, each with native and CPython/zlib
  checks. CPython 3.11.17 round trips pass with zlib 1.2.13 on Bookworm and 1.3.1
  on Trixie; wrong-provider and uncovered selectors retain their expected results.
- Bookworm's two upstream zlib 1.3.1 roots both pass. Different logical providers
  yield `PROVIDER_CHANGED`; the rebuilt same logical provider yields PASS with
  a separate `artifact_changes` entry.

Local evidence after the review fixes is retained in
`build/validation/v011-repair-bookworm-arm64/` and
`build/validation/v011-repair-trixie-arm64/`: `run.log`, environment snapshots,
configs, JSON reports and `release/` candidates. Both userspaces passed the full
suite again, including the eager missing-coverage and saved ELF regressions.
Bookworm's final release binary also passed the two-root comparison using the
existing upstream builds in `build/validation/v011-bookworm-arm64/zlib-roots/`.

The native Bookworm ARM64 candidate was packaged and checked for ELF machine,
extracted-byte equality, contents and checksum. Both ARM64 release candidates
currently have SHA-256
`0c67906e0affc21879d69b7146c9e2a3c2769d419956e6403fba37322501b282`.
The dual-archive packaging regression passed using a cross-built AMD64 ELF
strictly for archive/header checks; files in `build/packaging-only/` are not
validated release assets. This does not establish AMD64 runtime support.
Stale or missing version metadata, missing/malformed/duplicate checksum entries,
changed binaries and mismatched ELF machines were rejected before modifying
existing archives or their checksum file.

Default and candidate portable Go race tests and vet also passed on macOS ARM64
with its existing Go 1.27.1. YAML parsing, embedded shell syntax and diff whitespace
checks passed. Validation rejected mismatched expected architecture, glibc and Go
versions before running the suite. No dependencies were installed or upgraded.

At the time of these local checks, GitHub Actions had not run these changes.
Native AMD64 execution, the four GitHub-hosted jobs and the consumer example's
release download were pending in this local snapshot. Publication requires
successful `Linux amd64 / bookworm`, `Linux amd64 / trixie`,
`Linux arm64 / bookworm` and `Linux arm64 / trixie` in the `Linux integration`
workflow on this revision, using its `workflow_dispatch` or normal push/PR trigger
after authorized Git operations. The release job then consumes only that run's
successful native artifacts. A source push does not publish v0.1.1; publication
requires that gate and a separately authorized version tag.

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

For quick candidate checks in either target Linux userspace, run
`go test -ldflags='-X bindwitness/internal/witness.enableAMD64Capture=true' ./...`
and `go vet ./...`. Ordinary `go test ./...` on ARM64 checks the default profile;
on AMD64 its PASS-expecting capture integration tests require the candidate flag.
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

With separate authorization, pushing `v0.1.1` runs the Linux matrix and publishes
a GitHub release only after all four native environments pass, including both
two-root zlib jobs. The release job downloads each validated Bookworm binary
from that same workflow run into a separate directory. It checks CLI/source/tag
versions, ELF machine, archive checksums and extracted binary equality, and
publishes both archives and the common `SHA256SUMS`. It never rebuilds a binary.
Release notes are maintained in [releases/v0.1.1.md](releases/v0.1.1.md).

The [AMD64 consumer example](../examples/github-actions-amd64.yml) is runnable
after publication. It downloads the AMD64 archive and common checksum file,
verifies exactly the selected archive, uses the existing explicit native contract
with `required: true`, creates `build/report/` before capture, preserves the JSON
report using `if: always()`, and fails CI on every nonzero BindWitness exit code.
It requires neither a GPU nor another project. To change the workload profile,
edit the contract's explicit command, environment, objects and required selectors.

Short fuzz runs and the declared integration matrix do not establish exhaustive
parser coverage, ABI compatibility or behavior on unexecuted workload paths.
See [report limitations](report-format.md#capture-limitations) before interpreting
results outside the tested scope.
