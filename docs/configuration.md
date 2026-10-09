# Configuration

BindWitness accepts strict JSON with `schema_version: 1`. Unknown fields,
unsupported versions, invalid object mappings and nonpositive limits are rejected.
The schemas are [config.schema.json](config.schema.json) for capture/check and
[compare.schema.json](compare.schema.json) for comparison.

## Capture and check

In a source checkout, start from [examples/native.json](../examples/native.json).
A contract contains:

- `command`: a nonempty argv array. No shell is added. argv[0] is preserved,
  and executable lookup uses the effective PATH and working directory.
- `working_directory`: the initial directory, relative to the configuration
  file or absolute. If omitted, it is the configuration file's directory.
- `environment`: optional overrides applied over the inherited environment.
- `deadline`: a positive Go duration such as `5s` or `500ms`.
- `limits`: positive `stdout_bytes`, `stderr_bytes` and `trace_bytes`, each at
  most 33554432 bytes (32 MiB).
- `roots`: named artifact directories, relative to the configuration file or absolute.
- `objects`: logical IDs, each with a `root` key and a relative `path` inside
  that root. An optional lowercase `sha256` pins the exact artifact.
- `selectors`: exact bindings to check.

For example, a selector requiring an unversioned binding is:

```json
{
  "reference": "consumer",
  "symbol": "shared_value",
  "trace_version": "",
  "providers": ["a"],
  "required": true
}
```

`reference` and `providers` name declared logical objects. `symbol` is the exact
raw name, including C++ mangling when applicable. `required` defaults to false;
set it to true for every binding that the smoke workload must exercise. An
optional selector still checks any matching observations it receives.

`trace_version` has three meanings:

- Omitted: accept any version reported in the loader diagnostic.
- `""`: accept only a diagnostic with no version.
- A nonempty string: accept only that exact diagnostic version.

This field describes the loader's trace, not the version of an exported definition.

## Shared-object paths

Use absolute paths when your workload loads selected shared objects, including
arguments to `dlopen`. Configuration roots locate artifacts for inspection; they
do not rewrite command arguments or the application's library search profile.
Relative `working_directory`, root and object paths in the configuration are
supported, but a relative DSO name in loader evidence cannot establish identity:
LD_DEBUG does not record the cwd at the moment of loading. Such selected evidence
returns UNRESOLVED instead of guessing a path.

The executable is the exception: BindWitness resolves it before launch, so its
original relative argv[0] can identify that known file. Keep artifacts immutable
throughout the run. Symlink aliases that map one file to multiple logical IDs are
ambiguous. Basename, SONAME and identical bytes do not establish logical identity.

## Environment and process control

Configure inputs relevant to your smoke workload explicitly. Provenance records
overrides and the effective `LD_*`/`GLIBC_*` profile, not the entire inherited
environment. Avoid putting secrets in overrides that will be retained in reports.

BindWitness replaces inherited `LD_DEBUG` with `bindings,files`, removes
`LD_DEBUG_OUTPUT`, and records their previous values. Overrides of these two
managed variables are rejected. It does not automatically change `LD_BIND_NOW`,
preload, library paths or plugin loading flags. Explicit linker overrides are
retained in provenance. For example, `"environment": {"LD_BIND_NOW": "1"}`
explicitly requests eager binding. Required coverage means that a matching
binding diagnostic was observed. With either eager or lazy binding, this is not
proof that the function executed, nor a call count. The native validation suite
checks both profiles without automatically enabling eager binding.

Stdin is closed. Output is drained concurrently and bounded; excess bytes are
discarded. The mixed stderr budget is `trace_bytes + stderr_bytes`, then each
component's own limit is applied. Losing diagnostics makes capture incomplete.
The workload process group is terminated on deadline, SIGINT or SIGTERM, and
remaining processes in that group are cleaned up after the run. This is not a
sandbox or a supervisor for detached process trees.

## Comparison

Start from [examples/compare.json](../examples/compare.json). Comparison declares
`left_roots`, `right_roots`, `objects` and `selectors`, with no workload command.
Pair corresponding artifact roots under the same root key. The object's relative
path must be the same under the paired roots in schema version 1.

Use one logical ID when two artifacts represent the same provider, even after a
rebuild. Compare records differing hashes in `artifact_changes`. Use different
logical IDs when library selection changes the provider you want to distinguish.
Every observed comparison provider needs an explicit mapping.

Selectors use the same fields as check. In compare, `providers` lists candidate
logical objects rather than imposing an allow list; artifact pins are not enforced.
Events are grouped by exact reference/symbol/trace-version, even if the version
selector is omitted. Order and event count do not establish matching calls.

Equal observed provider sets agree within the declared scope. Disjoint sets
supply a provider-change witness. If unequal sets share a provider, the extra
observations may reflect coverage, so the result is UNRESOLVED. A version observed
on only one side is also a coverage gap. Independent capture or identity issues
remain in the result; insufficient evidence cannot supply a change witness.

## Using saved evidence

Offline check/compare read saved identities and raw evidence without inspecting
historical artifact paths. Use an observation or a check report with supported
schema version 1. BindWitness checks the trace checksum and reparses its events;
damaged evidence or an unsupported report envelope is an infrastructure error.
A complete trace does not establish coverage of functions the workload never used.
