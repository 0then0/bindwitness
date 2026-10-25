# Contributing to BindWitness

BindWitness checks selected runtime ELF symbol bindings against explicit
contracts. Changes should preserve the distinction between a proven violation,
missing evidence, capture completeness and the workload's exit status.

## Before opening an issue

Search existing issues and read the [README](README.md),
[configuration guide](docs/configuration.md) and
[capture limitations](docs/report-format.md#capture-limitations).
Use the issue templates for reproducible bugs and feature proposals.
Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
Participation follows the [code of conduct](CODE_OF_CONDUCT.md).

## Development setup

Use Go 1.25 or newer. The CLI has no third-party Go dependencies. A C compiler
is needed for native fixtures; Docker provides the pinned Linux environments.
Python 3 and Perl are used by the integration validation scripts, not by the CLI.

```sh
git clone https://github.com/0then0/bindwitness.git
cd bindwitness
mkdir -p build
CGO_ENABLED=0 go build -trimpath -o build/bindwitness ./cmd/bindwitness
go test ./...
go vet ./...
```

On macOS, these commands cover portable parsing and offline evaluation. Native
capture must be tested on Linux AMD64 or ARM64 with supported glibc. Follow the
[validation guide](docs/validation.md) for the pinned Bookworm/Trixie containers,
race tests, bounded fuzzing, native fixtures and external zlib cases.

## Making a change

- Keep pull requests focused. Explain the observed problem and resulting behavior.
- Format Go code with `gofmt`. Follow `.editorconfig` for other source files.
- Add regression tests for changed behavior, including relevant failure paths.
  Documentation-only changes do not need implementation-mirroring tests.
- Keep CLI help, documentation, examples and schemas consistent with public changes.
  Discuss incompatible contract or schema changes before implementing them.
- Keep generated binaries and new validation runs under ignored `build/` or `dist/`.
  Keep `testdata` source fixtures and representative evidence in version control.
- Do not reformat saved traces or reports. When adding real evidence, retain its
  provenance and raw diagnostics, and document the reproduction steps.
- Do not include credentials, private paths or confidential workload output in
  issues, pull requests or new public fixtures.

## Validation and pull requests

Run `go test ./...` and `go vet ./...` for Go changes, plus the affected validation
scripts for capture or integration changes. Check formatting with
`gofmt -l cmd internal` and whitespace with `git diff --check`.
State which checks ran, their results and any environment limitations in the PR.
Expected negative integration cases print FAIL or UNRESOLVED; their validation
scripts exit zero when those expected outcomes are observed.

CI checks formatting and runs the native AMD64/ARM64 Bookworm/Trixie matrix.
Reports are retained as workflow artifacts, including when validation fails.
The PR template asks for the behavior change, verification and compatibility
impact. Commit messages use Conventional Commits in English, for example
`fix: retain loader error on failed library load` or `docs: clarify offline replay`.

## Releases

Maintainers update the CLI version, version test, release notes and versioned
examples together. Document user-visible changes in [CHANGELOG.md](CHANGELOG.md).
The existing version-tag workflow validates all native environments, packages
the validated Bookworm binaries and publishes both architectures with
`SHA256SUMS`. See [release packaging](docs/validation.md#package-a-release).
