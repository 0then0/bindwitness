# Perl bundled/system zlib load order: real integration case

The same real `Compress::Raw::Zlib` extension and system libz, performing the same
compression/decompression round trip, select different symbol providers when
their global load order changes. BindWitness detects the difference even though
application stdout is identical and both workloads exit 0. Upstream prefixing
removes this sensitivity for the selected extension bindings.

This is a **real integration case inspired by upstream issue #8**, not an exact
reproduction of the published AlmaLinux/RHEL Perl build failure or a crash.
There is no custom interposer. The only workload glue is a Perl script using
core DynaLoader and the real extension API.

## Upstream investigation and dependency policy

[Issue #8](https://github.com/pmqs/Compress-Raw-Zlib/issues/8) describes unprefixed
bundled zlib exports competing with OS libz, including an observed extension
`inflate` binding to system libz. Its discussion supplies a version mismatch
during a Perl build on AlmaLinux 8, but no complete load-order reproducer.
That environment brings in system libz transitively through libperl dependencies;
this case explicitly loads it instead. The published mismatch involved zlib
1.2.12 versus 1.2.11; this experiment uses 1.2.12 versus 1.2.13.

[PR #11](https://github.com/pmqs/Compress-Raw-Zlib/pull/11) adds `-DZ_PREFIX` and
changes bundled names to `Perl_crz_*`. It was merged as
`64aea2d3f78946d7df4096eadfa0d7267f4439a5`. Release 2.104 first included the
fix, but its system-zlib build option had a separate prefix problem described
in the same discussion. We use release 2.105, whose `Makefile.PL` enables the
prefix when `BUILD_ZLIB=1` and disables it when `BUILD_ZLIB=0`.

The explicit policy here is **a bundled build must use its own selected zlib
functions**. This implements the isolation intended by the upstream prefix fix.
It also keeps the extension's bundled 1.2.12 headers and runtime implementation
together. A system provider is a violation of this declared bundled-build policy;
it is not evidence that every use of system zlib is wrong or ABI-incompatible.
For a deliberate `BUILD_ZLIB=0` build, a system-provider contract would be appropriate.

## Artifacts and build profile

Pinned CPAN releases from `https://www.cpan.org/authors/id/P/PM/PMQS/`:

- `Compress-Raw-Zlib-2.103.tar.gz`, SHA-256
  `d69d2620ca024dc1b424f7f4228fe1169b2b77416bb30310e890d3be7da47dff`.
- `Compress-Raw-Zlib-2.105.tar.gz`, SHA-256
  `228159574899c56fe616c4dc889bddcf41db6079e095a2d622af96b043bbe7d8`.

Both contain bundled zlib 1.2.12. The
[build script](../scripts/build-perl-zlib-case.sh) verifies checksums before
extraction and uses `BUILD_ZLIB=1 ZLIB_INCLUDE=./zlib-src ZLIB_LIB=./zlib-src`,
`perl Makefile.PL OPTIMIZE=-O2`, `make -j2`, and `make test`. It never installs
the module. A build directory must be new; reusing it fails explicitly.

Run the case in the immutable Bookworm image defined by
[validation.Dockerfile](../scripts/validation.Dockerfile):
`golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437`.
The [stored representative reports](../testdata/reports/bookworm/perl-zlib/README.md)
use Linux ARM64, Debian Bookworm, glibc 2.36,
Perl `5.36.0-7+deb12u3`, system zlib `1:1.2.13.dfsg-1`, GCC 12.2.0,
GNU ld 2.40, Go 1.25.14, Docker LinuxKit kernel 7.0.14 on a macOS ARM64 host.
Each new report records its actual architecture, glibc and loader identity.
The source project's supported capture matrix is described in
[validation.md](validation.md).

Both extension builds use `-DNO_VIZ -DZ_SOLO -DGZIP_OS_CODE=3 -DUSE_PPPORT_H`.
2.105 additionally uses `-DZ_PREFIX -DPerl_crz_BUILD_ZLIB=1`.
Linking uses `-shared -L/usr/local/lib -fstack-protector-strong` without
`-Bsymbolic`, export hiding or a new version script. Full generated Makefiles,
compiler/linker commands, upstream test logs and ELF inspections remain in the
build output. Different source versions produce different extension artifacts;
they are never treated as a same-artifact order comparison.

## Explicit scenarios

[workload.pl](../scripts/perl-zlib-workload.pl) calls
`DynaLoader::dl_load_file(ABSOLUTE_PATH, 0x01)` for both objects, with
`0x01` selecting `RTLD_GLOBAL`. It retains both handles and then loads the Perl
module through its absolute `blib` paths. XSLoader reuses the extension's
existing mapping; there is no unload or second link map.

The four round-trip variants are:

- 2.103 bundled-first: extension, then system libz. Expected own provider, PASS.
- 2.103 system-first: system libz, then extension. Expected policy violation, FAIL.
- 2.105 bundled-first: same order with the prefixed release. Expected PASS.
- 2.105 system-first: reverse order with the prefixed release. Expected PASS.

Within each version the artifact paths, hashes, global flags, payload and operation
are identical; only the two load calls change order. Each capture launches a fresh
Perl process. The payload is a fixed 2,560-byte string containing NUL and byte 1;
the operation compresses it, finishes the stream, decompresses and verifies all
bytes. Stdout contains only `round_trip: true` and the original `payload_hex`.
Module/header/runtime versions are recorded independently in workload stderr.

The profile explicitly sets `PERL_DL_NONLAZY=0`, uses ordinary lazy loading, and
does not set `LD_BIND_NOW`. No other loader profile is inferred or tested here.
The validation script rejects inherited `LD_*`/`GLIBC_*` settings to keep the
documented container experiment explicit. BindWitness still manages only
`LD_DEBUG=bindings,files` and `LD_DEBUG_OUTPUT` as documented in the existing CLI.

## Witness and negative controls

Required, unversioned trace selectors from 2.103 `Zlib.so` are `deflate`,
`inflate`, and `zlibVersion`, with that exact extension as the permitted provider.
For 2.105, the selectors explicitly change to `Perl_crz_deflate`,
`Perl_crz_inflate`, and `Perl_crz_zlibVersion` and the distinct 2.105 object.
The prefix is a symbol-name change, not an ELF trace-version annotation.

The historical ARM64 report records these exact artifacts:

- 2.103 extension SHA-256:
  `6ef590d5e2f41d228d697a90c8f7679a5950ddfd44656851f3819487d21cc329`.
- 2.105 extension SHA-256:
  `c20c253e835a8dce5fc3581987854b9a5f486846ae97abd3473cf3e42772ade8`.
- `/usr/lib/aarch64-linux-gnu/libz.so.1.2.13` SHA-256:
  `ffb1ab496e6eced03ab679075f9f2c415c7728a145cc7f63d614497102d73822`.

The full absolute extension paths are retained in the
[saved reports](../testdata/reports/bookworm/perl-zlib/README.md). In those reports,
the 2.103 bundled-first bindings target that same extension. The system-first
bindings instead target `/usr/lib/aarch64-linux-gnu/libz.so.1.2.13` for all three
selectors. BindWitness returns `PROVIDER_NOT_ALLOWED` with the original raw
trace line. The order comparison returns three `PROVIDER_CHANGED` findings,
with both source lines and no artifact changes.

For example, `2.103-system-first-01.json` trace line 300 is the concrete witness:

```text
       526:	binding file /work/build/validation/perl-zlib-bookworm-arm64/build/Compress-Raw-Zlib-2.103/blib/arch/auto/Compress/Raw/Zlib/Zlib.so [0] to /usr/lib/aarch64-linux-gnu/libz.so.1.2.13 [0]: normal symbol `deflate'
```

The 2.105 prefix is the upstream negative control: both orders bind the selected
prefixed symbols to the 2.105 extension, and their comparison passes. We compare
orders separately within each release. A cross-release comparison of these exact
symbol selectors would have unmatched coverage, not establish that one provider
changed into another.

The 2.103 system-first version metadata changes from header/runtime
`1.2.12/1.2.12` to `1.2.12/1.2.13`, but the complete application stdout is
identical and all four round-trip workloads exit 0. The primary witness is the
binding, not a failed round trip or a reproduced upstream crash.

Additional coverage controls:

- Recheck saved evidence requiring `inflateBack`, which the workload never
  binds: UNRESOLVED / `REQUIRED_NOT_OBSERVED`. Comparing two sides without that
  required symbol remains UNRESOLVED, without a provider-change witness.
- A fresh `load-only` process loads the same objects but omits the round trip.
  It exits 0 with complete capture, yet required `deflate`/`inflate` remain
  uncovered: UNRESOLVED. Comparing it with the round-trip observation returns
  `COMPARE_COVERAGE_GAP`, without `PROVIDER_CHANGED`.
- `capture` of the policy-violating system-first profile returns capture PASS;
  offline `check` of that observation returns binding FAIL. Capture completeness,
  workload exit, binding verdict and overall check outcome are separate fields.
- A missing extension or system libz must stop the workload with a nonzero exit
  and the original `dl_load_file` error, before any application-success output.
  The validator checks both failures for both versions and both load orders.
  These runs have complete capture but UNRESOLVED binding verdict because the
  required calls were not observed. Failed loading must not be mistaken for a
  successful prefix-isolation control.

## Reproduce and inspect

On a same-architecture Docker Linux VM, from the repository root:

```sh
docker build -f scripts/validation.Dockerfile -t bindwitness-perl-zlib .
docker run --rm -v "$PWD:/work" -w /work \
  -e VALIDATION_IMAGE=golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437 \
  bindwitness-perl-zlib sh -c '
    set -eu
    mkdir -p build/perl-zlib
    CGO_ENABLED=0 go build -trimpath -o build/perl-zlib/bindwitness ./cmd/bindwitness
    sh scripts/build-perl-zlib-case.sh build/perl-zlib/build &&
    python3 scripts/perl-zlib-validation.py build/perl-zlib/bindwitness \
      build/perl-zlib/build build/perl-zlib/evidence
  '
```

Choose a new build/output directory for a subsequent complete reproduction.
`--repetitions 10` is the default; `--repetitions 1` is an explicit smoke check.
The script delegates every verdict to the existing CLI and checks expected
results. Its exit 0 means validation expectations held, including the real FAIL
and UNRESOLVED reports. It does not relabel those reports PASS.

Each evidence directory contains the full original reports and configs,
individual CLI logs, a `runs.json` index of commands/exits/outcomes/report links,
`provenance.json`, package and ELF snapshots, the exact copied workload and
`validation-errors.json`. The index summarizes validation commands and results;
the CLI reports use the existing report schema. Source archives, build flags and logs are alongside
it in the build directory. Binding observations retain loader provenance, raw
trace, namespace/PID, exact paths, hashes, available ELF identities and coverage.
Hashes are pinned in each generated contract; paths define distinct logical
objects even if their basenames are both `Zlib.so`.

Replay a selected saved observation, even on macOS with no historical artifacts:

```sh
go build -o build/bindwitness ./cmd/bindwitness
./build/bindwitness check \
  --config testdata/reports/bookworm/perl-zlib/2.103-system-first-01.config.json \
  --observation testdata/reports/bookworm/perl-zlib/2.103-system-first-01.json \
  --report build/perl-zlib-offline.json
```

Expected CLI exit is 1, outcome FAIL. All order comparison configs and individual
reports are also generated, so `compare --config ... --left ... --right ...`
can be rerun offline without inspecting old ELF files.

## Validation and scope

The validator launches 10 fresh processes for each round-trip variant by default,
rechecks every observation offline, compares each pair of orders and compares
the first and last repetitions. It checks hashes and application stdout across
the runs. Coverage controls and failed-load tests run separately from the four
round-trip variants. Upstream `make test` runs before binding validation.

[CI](../.github/workflows/ci.yml) runs this case in native Bookworm AMD64 and ARM64
jobs using the validated release binary. Reports and build provenance are
uploaded even when validation fails. A successful validation job means every
declared expectation was met, including the expected FAIL and UNRESOLVED reports.

This case uses existing `capture`, `check`, offline replay and `compare` commands.
Perl, compilers and the external source packages are validation prerequisites;
they are not dependencies of an installed BindWitness CLI.

The result proves provider sensitivity for the selected observed bindings and
shows the effect of upstream prefixing. It does not establish ABI compatibility,
function execution coverage, call counts, all possible import orders, GPU-bug
reproduction or the original upstream failure's exact conditions. Interpret each
run using its saved environment and the [capture limitations](report-format.md#capture-limitations).
