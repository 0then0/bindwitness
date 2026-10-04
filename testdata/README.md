# Test fixtures

This directory is part of BindWitness's reproducible test suite and is kept in
the public repository. Go excludes `testdata` from ordinary package discovery;
its contents are not runtime dependencies of the CLI.

- `native/`: small C sources used to build real executable/shared-object fixtures.
  Build them with `sh scripts/build-fixtures.sh`; the default output is
  `build/fixtures`, outside this directory.
- `traces/`: real glibc 2.36 and 2.41 diagnostic streams with provenance. Parser
  tests and fuzz seeds use these files.
- `reports/`: representative real captures and evaluations with their original
  contracts, runtime versions and file identities. See the
  [saved-report guide](reports/README.md) for offline replay.

Keep fixtures and representative evidence under version control. Generated ELF
binaries, new reports and release archives belong in ignored `build/` and `dist/`.
See the [validation guide](../docs/validation.md) to reproduce the test matrix.
