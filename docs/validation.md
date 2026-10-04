# Testing and validation

Use this guide to run the test suite, reproduce binding witnesses and package a
Linux ARM64 binary. Go 1.25 or newer and a C compiler are needed for native tests.
CPython and zlib are validation workloads, not dependencies of the CLI.

## Tested environments

The validation matrix uses Linux ARM64 with two glibc versions:

- Debian 12 Bookworm: glibc 2.36, GCC 12.2.0, Go 1.25.14.
- Debian 13 Trixie: glibc 2.41, GCC 14.2.0, Go 1.25.14.

Local Linux validation runs in Docker Desktop on an ARM64 host. These are two
userspaces on the same VM kernel. macOS ARM64 is used for offline analysis;
native macOS capture is not supported. Other architectures and glibc versions
are not covered by this matrix.

[scripts/validation.Dockerfile](../scripts/validation.Dockerfile) pins the images:

- Bookworm: `golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437`.
- Trixie: `golang:1.25-trixie@sha256:2c4c60ef415fbfa5e90300722293bef36c5e63fae17570ce18f580af933dbd73`.
- CPython: `python:3.11-slim-bookworm@sha256:2333bd330d12de02514770b3585cad313644316047cdee24a7acfdece6de6efb`.

The Dockerfile places CPython under `/opt/cpython`. Debian's default interpreter
may have a built-in zlib module; this validation explicitly uses CPython's shared
zlib extension. Saved raw trace provenance includes image references, file hashes
and runtime details. Each validation run also writes OS, compiler, Go and kernel
snapshots alongside its reports.

## Run the full Linux checks

From an ARM64 source checkout with Docker available:

```sh
docker build -f scripts/validation.Dockerfile -t bindwitness-validation:bookworm .
docker run --rm -v "$PWD:/work" -w /work \
  bindwitness-validation:bookworm sh scripts/validate.sh build/validation/bookworm

docker build -f scripts/validation.Dockerfile \
  --build-arg BASE=golang:1.25-trixie@sha256:2c4c60ef415fbfa5e90300722293bef36c5e63fae17570ce18f580af933dbd73 \
  -t bindwitness-validation:trixie .
docker run --rm -v "$PWD:/work" -w /work \
  bindwitness-validation:trixie sh scripts/validate.sh build/validation/trixie
```

[scripts/validate.sh](../scripts/validate.sh) runs:

```sh
go test -race ./...
go vet ./...
go test ./internal/witness -run='^$' -fuzz=FuzzParseTrace -fuzztime=5000x -parallel=2
```

It installs a CLI with `CGO_ENABLED=0`, runs native and CPython/zlib cases, then
builds and exercises a stripped release binary. Reports and configurations are
written to the output directory. Expected negative cases print FAIL or
UNRESOLVED; their validation scripts exit 0 when the expected result is obtained.
[GitHub Actions](../.github/workflows/ci.yml) runs the same matrix and retains
JSON reports and the release binary as artifacts.

For quick checks on supported Linux, run `go test ./...` and `go vet ./...`.
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

The ordinary fixture profile uses lazy binding. An explicit eager profile changes
coverage semantics; dedicated `LD_BIND_NOW` integration coverage is not included.

## CPython and zlib

The external case performs compression/decompression of a short fixed payload
with real CPython 3.11.17 and its shared zlib extension. It checks the extension's
`deflate` and `inflate` bindings. Runtime zlib is 1.2.13 on Bookworm and 1.3.1 on
Trixie. The correct provider passes, an intentionally wrong provider fails, and
an uncovered required `inflateBack` returns UNRESOLVED.

To compare two upstream zlib 1.3.1 builds on Bookworm:

```sh
docker run --rm -v "$PWD:/work" -w /work \
  bindwitness-validation:bookworm sh -c '
    sh scripts/build-zlib-roots.sh /work/build/zlib-roots
    env LD_LIBRARY_PATH=/opt/cpython/lib PYTHONHOME=/opt/cpython \
      /opt/cpython/bin/python3 scripts/zlib-validation.py \
      build/validation/bookworm/installed/bindwitness build/zlib-comparison \
      build/zlib-roots/one build/zlib-roots/two
  '
```

The script downloads the [upstream archive](https://zlib.net/fossils/zlib-1.3.1.tar.gz),
checks SHA-256 `9a93b2b7dfdac77ceba5a558a580e74667dd6fede4585b91eefb60f03b72df23`,
and builds `-O0` and `-O2` installations. Both round trips pass. Explicit
`LD_LIBRARY_PATH` profiles select different logical providers, producing a compare
witness. Mapping both builds to one logical provider instead yields PASS with
`artifact_changes`. This is an integration/configuration case, not a reproduction
of an upstream bug or evidence of external adoption.

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
sh scripts/package-release.sh build/validation/bookworm/release/bindwitness
```

This creates `dist/bindwitness-0.1.0-linux-arm64.tar.gz` and `dist/SHA256SUMS`.
The archive contains the binary, Apache-2.0 license, README, icon, documentation
and JSON schemas. It does not include development fixtures. Verify the checksum
before extracting a distributed archive, then run `./bindwitness --version` and
check your own contract. Packaging does not publish a GitHub release.

Pushing a version tag such as `v0.1.0` runs the Linux matrix and publishes a
GitHub release only after both environments pass. The release job packages the
validated Bookworm binary from that same workflow run, checks the CLI version
against the tag, verifies the archive checksum and binary contents, and uploads
the archive and `SHA256SUMS` before publishing the draft. The tag must match the
version declared by the CLI.

Short fuzz runs and the declared integration matrix do not establish exhaustive
parser coverage, ABI compatibility or behavior on unexecuted workload paths.
See [report limitations](report-format.md#capture-limitations) before interpreting
results outside the tested scope.
