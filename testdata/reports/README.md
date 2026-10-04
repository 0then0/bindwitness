# Representative reports

These JSON files come from real Linux validation runs. Each check/compare report
has its original configuration beside it. Absolute paths, timestamps, addresses
and PIDs are retained as provenance, not portable live-run settings. No ELF
binaries are stored here.

`bookworm/` uses glibc 2.36 and `trixie/` uses glibc 2.41. Compiler, Go, OS-release
and kernel snapshots accompany each environment. Image references and reproduction
commands are documented in the [validation guide](../../docs/validation.md).

The reports cover native positive, provider mismatch, uncovered and comparison
cases; real CPython/zlib contracts; and two upstream zlib builds compared as both
distinct providers and artifacts of one logical provider. Comparison reports
include both complete observations, with raw traces and identities.

## Replay a report

From the repository root, with a locally built CLI:

```sh
mkdir -p build
./build/bindwitness check \
  --config testdata/reports/bookworm/native/native-positive.config.json \
  --observation testdata/reports/bookworm/native/native-positive.json \
  --report build/replayed.json
```

Offline analysis uses saved evidence without accessing historical Linux paths.
The expected result is PASS. Replaying a mismatch report with its contract should
return FAIL; an uncovered report should return UNRESOLVED.

To reproduce a live capture, build fixtures and generate a new configuration
using the validation scripts. Historical configurations should only be used
with their saved observations outside the original fixture setup.
`TestRepresentativeReports` re-evaluates every saved report against its contract
or comparison mapping and checks that the recorded outcome agrees.
