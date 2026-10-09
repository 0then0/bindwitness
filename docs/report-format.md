# JSON reports, schema version 1

All JSON input is bounded to 256 MiB and decoded with unknown fields rejected.
Report writes use a private temporary file in the destination directory, fsync,
close and atomic rename. The destination directory must already exist. Exit 3
is authoritative when writing the mandatory report fails, even if an older file
is present at that path. Inputs and reports are trusted, portable evidence files.

`capture` emits `kind: observation`. `check` emits `kind: check`, wrapping an
observation plus contract evaluation. `compare` emits `kind: compare`, wrapping
both observations and comparison results. Preparation/input/report errors emit
`kind: error`, `outcome: INFRASTRUCTURE_ERROR`, if a report can be saved.

The JSON Schemas in this directory describe version 1. A future incompatible
report change must use a new `schema_version`. Finding IDs and exit codes are
stable within this schema. Human-readable messages and timestamps are not stable.
Unsupported versions are rejected for both standalone observations and the
outer envelope of a saved check report.

## Observation

- `provenance`: tool/backend version, OS release, architecture, command argv,
  working directory, explicit environment overrides, effective `LD_*`/`GLIBC_*`
  profile, loader path/hash/version, timestamps, PID, diagnostics policy and
  trace checksum. Inherited non-linker environment is not exhaustively recorded;
  explicitly configure relevant inputs and use reproducible artifacts/images.
- `workload`: started, completed/reaped, exit code (negative for signal on Go),
  deadline state, bounded stdout/stderr and their truncation flags.
- `capture`: complete, trace truncation flag, independent capture issues.
- `trace`: unmodified loader diagnostic lines without their newline terminators.
  `trace_sha256` hashes each saved line followed by one newline.
- `bindings`: PID, reported `reference` and `provider`, raw `symbol`,
  `trace_version` (empty when absent), both namespace identifiers and one-based
  `trace_line` into `trace`. Non-base/other-PID events remain evidence but cannot
  satisfy selectors or supply contract witnesses.
- `objects`: identities keyed by the original observed name and by declared
  paths, where applicable. Each has `observed_path`, `resolved_path`, `logical_id`,
  SHA-256, ELF class/machine, SONAME/build ID where available, `stability` and
  any `problem`. `logical_id` records the capture mapping; offline checks and
  comparisons independently apply the supplied path/root mappings.

Identity stability values are `pre_post_unchanged`, `post_only`, `changed` and
`ambiguous`. Missing files can retain an identity `problem` without a hash.
Export-definition symbol versions are not collected; `trace_version` must never
be interpreted as such a version. Hashes and build IDs do not prove mapped bytes.

`capture.complete` means stream/parser/scope conditions held, not that all
functions ran. Output truncation of stdout alone does not invalidate bindings;
loss of the mixed stderr stream can lose diagnostics and does invalidate capture.

## Evaluation

`outcome` is the final CI result. `binding_verdict` is PASS, FAIL or UNRESOLVED
for the declared selected bindings and necessary evidence conditions. Infrastructure
errors have unresolved binding verdict. `coverage` gives per-selector observed
counts and requiredness; these are diagnostic event counts, not call counts.
Compare counts the smaller observation count across the two sides; its findings
also disclose unmatched exact versions and unmatched provider observations.

`findings` has stable `id`, a message and optional selector index, witness,
other-side witness and expected logical provider list. Violations sort before
incompleteness so the first actionable witness is easy to find. Both sides'
complete raw evidence and identities remain in comparison reports. Rebuilds of
the same observed logical provider are listed separately in `artifact_changes`.
Compare does not apply contract pins or allowed-provider gates.

The exit precedence is infrastructure error (3), observed violation (1),
incomplete/insufficient evidence (2), then pass (0). A malformed saved envelope or
checksum/event disagreement is an infrastructure input error; a valid capture
containing malformed loader diagnostics is unresolved.

## Finding IDs

Contract/comparison findings:

- `PROVIDER_NOT_ALLOWED`: resolved provider outside the explicit allow list.
- `ARTIFACT_PIN_MISMATCH`: selected stable artifact differs from its exact pin.
- `PROVIDER_CHANGED`: mapped binding provider sets are disjoint across observations.
- `REQUIRED_NOT_OBSERVED`: missing required workload coverage.
- `COMPARE_COVERAGE_GAP`: a selected exact version is absent on one side, or
  unequal provider sets share a provider and cannot establish a change witness.
- `IDENTITY_UNRESOLVED`, `PROVIDER_IDENTITY_AMBIGUOUS`: insufficient identity.
- `CAPTURE_INCOMPLETE`: independent capture conditions did not hold.
- `INVALID_OBSERVATION`, `INVALID_CONTRACT`, `INVALID_MAPPING`: internal evaluator
  guards, also exposed for programmatic evaluation of invalid data.

Capture/parser findings:

- `MALFORMED_BINDING`, `TRACE_LINE_LIMIT`, `UNSUPPORTED_FILES_FORMAT`.
- `UNSUPPORTED_PROCESS_TREE`, `UNSUPPORTED_NAMESPACE`, `UNSUPPORTED_RELOAD`,
  `UNSUPPORTED_EXEC`.
- `NO_LOADER_STARTUP`, `UNTESTED_GLIBC`, `UNVALIDATED_PLATFORM`, `TRACE_TRUNCATED`.
- `WORKLOAD_DEADLINE`, `WORKLOAD_CANCELLED`, `WORKLOAD_SIGNAL`, `STREAM_INCOMPLETE`.
- `LOADER_CHANGED`, `EXECUTABLE_CHANGED`.

`HARNESS_ERROR` is used for CLI infrastructure errors. An unknown future ID
should be preserved by report consumers; gate on the declared outcome/exit code.

## Capture limitations

Supported capture uses one controlled executable lifetime in the base linker
namespace, without unload/reload cycles. Additional diagnostic PIDs, non-base
namespaces, repeated link maps and repeated startup markers are retained and
flagged. Their events cannot supply contract witnesses. Silent forks, detached
children, re-exec with diagnostics removed and reload through different path
spellings are not completely detectable with LD_DEBUG; keep the workload inside
the declared scope.

Selected shared objects must have absolute loader paths. A relative DSO name does
not identify the cwd at loading time, so its identity remains ambiguous. This
also applies when re-evaluating saved reports with claimed relative DSO identities.
The executable's exact original argv[0] can identify its path resolved before launch.

Artifacts must remain immutable throughout the run. Missing or changed identities
cannot supply a successful check or comparison witness. Offline analysis checks
saved evidence, not the current files or cryptographic authenticity of a report.

Capture drains bounded stdout/stderr concurrently, splits loader diagnostics from
stderr, and creates no loader trace files. A trusted workload must not forge
loader lines. Lines resembling diagnostics can be conservatively classified;
unknown relevant binding lines are retained and flagged. An incomplete final
diagnostic or loss of the mixed stderr stream can make capture unresolved.

SIGINT/SIGTERM cancellation and deadline expiry terminate the workload process
group. Cancellation is reported as `WORKLOAD_CANCELLED`, distinct from
`WORKLOAD_DEADLINE`; signal termination is also retained. Output draining has a
one-second WaitDelay. Detached process trees are outside the capture scope.

A binding event is not a call count, call stack, proven dlsym caller or final IFUNC
execution address. DT_NEEDED entries, exported symbols and loaded libraries alone
do not prove a selected binding. The backend follows glibc's documented
[debug categories](https://sourceware.org/glibc/manual/latest/html_node/Dynamic-Linker-Environment-Variables.html),
but LD_DEBUG is not a stable machine protocol. Only the declared tested glibc
formats can produce complete supported capture results.

The release validation targets Linux AMD64 and ARM64 with glibc 2.36 and 2.41.
Other architectures/operating systems produce `UNVALIDATED_PLATFORM` and other
glibc versions produce `UNTESTED_GLIBC`; sufficient binding observations cannot
turn those evidence gaps into PASS. A proven violation still retains FAIL.
Offline evaluation checks the platform recorded in provenance and the saved ELF
class/machine of the executable and observed binding objects, independently of
the machine evaluating the report or historical files. Unused declared objects
do not establish a workload platform. See [current validation status](validation.md).
Source builds and release binaries enable the validated AMD64 and ARM64 matrix.
Live capture also requires executable and loader identities to match the CLI's
architecture and ELF64 class. Compat ELF32/x32 or foreign-machine artifacts
remain unresolved.
