# Real Perl bundled/system zlib integration evidence

These reports contain ARM64 / glibc 2.36 evidence captured on 2026-10-09. This is an integration
case inspired by Compress::Raw::Zlib issue #8, not the exact AlmaLinux failure.
See the [case study](../../../../docs/perl-zlib-case-study.md) for upstream sources,
archive checksums, build flags, reproduction commands and limits.

- `2.103-bundled-first-01`: PASS, own `deflate`, `inflate`, `zlibVersion` provider.
- `2.103-system-first-01`: FAIL / `PROVIDER_NOT_ALLOWED`, system libz provider.
- `2.105-system-first-01`: PASS, prefixed symbols use the fixed extension.
- `2.103-order-compare-01`: FAIL / `PROVIDER_CHANGED`, same artifact identities.
- `load-only-compare`: UNRESOLVED / `COMPARE_COVERAGE_GAP`, no change witness.

Each JSON report has the original schema-v1 contract beside it. Comparison
reports include both full observations. Absolute paths, hashes, timestamps,
PIDs and raw diagnostics are preserved; no upstream ELF binaries are committed.
The existing `TestRepresentativeReports` replays these reports offline, including
on hosts without the historical files. `provenance.txt` records build/image and
artifact metadata; the remaining text snapshots record the compiler, linker,
Perl configuration and explicit build profile.
