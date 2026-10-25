# Changelog

User-visible changes are grouped by release. Detailed release notes are under
[docs/releases](docs/releases). Version tags identify the source for each release.

## Unreleased

- Add repository editing and Git attributes, contributor and security policies,
  issue forms and a pull request template.
- Add a Go formatting CI gate and bounded workflow runtimes.
- Include contributor and security policies and the changelog in release archives.

## [0.1.2](https://github.com/0then0/bindwitness/releases/tag/v0.1.2)

- Add a reproducible bundled/system zlib case using Compress::Raw::Zlib 2.103
  and 2.105, full representative reports and offline replay instructions.
- Stop the validation workload on mandatory library-load failures and cover both
  libraries, releases and load orders with eight regression scenarios.
- Run the case on native Bookworm AMD64 and ARM64; archive complete build evidence
  in a format accepted by GitHub Actions artifacts.
- Update the public documentation. CLI behavior and schema version 1 are unchanged.

See [v0.1.2 release notes](docs/releases/v0.1.2.md).

## [0.1.1](https://github.com/0then0/bindwitness/releases/tag/v0.1.1)

- Add validated Linux AMD64 capture alongside ARM64 on glibc 2.36 and 2.41.
- Publish native AMD64 and ARM64 archives from the same validated workflow run.
- Add explicit eager-binding checks and an AMD64 consumer workflow example.
- Apply platform checks to saved executable and observed binding identities.

See [v0.1.1 release notes](docs/releases/v0.1.1.md).

## [0.1.0](https://github.com/0then0/bindwitness/releases/tag/v0.1.0)

- Introduce runtime ELF binding capture on Linux ARM64 with glibc 2.36 and 2.41.
- Add explicit provider contracts, offline checking and observation comparison
  with schema version 1, raw loader evidence and artifact identities.
- Add native fixture and CPython/zlib validation, and validated ARM64 archives.
