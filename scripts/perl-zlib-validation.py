#!/usr/bin/env python3
"""Issue #8 integration case: existing CLI is the only binding evaluator.

The run index is a validation log, not another evidence format or suite API.
All full schema-v1 reports remain available, including unexpected outcomes.
"""

import argparse
import copy
import hashlib
import json
import os
import platform
import shutil
import subprocess
from pathlib import Path


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("build_root", type=Path)
    parser.add_argument("new_output", type=Path)
    parser.add_argument("--repetitions", type=int, default=10)
    args = parser.parse_args()
    if platform.system() != "Linux":
        parser.error("live validation requires Linux/glibc; no scenarios were run")
    if not 1 <= args.repetitions <= 20:
        parser.error(
            "repetitions must be 1..20 (use at least 10 for milestone evidence)"
        )
    # Do not silently change an inherited loader profile to obtain a witness.
    inherited = [k for k in os.environ if k.startswith(("LD_", "GLIBC_"))]
    if inherited:
        parser.error(
            "run in the documented clean container profile; inherited "
            + ", ".join(inherited)
        )
    binary, build, out = (
        p.resolve() for p in (args.binary, args.build_root, args.new_output)
    )
    out.mkdir(parents=True, exist_ok=False)
    workload = out / "workload.pl"
    shutil.copyfile(Path(__file__).with_name("perl-zlib-workload.pl"), workload)
    perl = Path(shutil.which("perl")).resolve()
    cache = subprocess.check_output(["/sbin/ldconfig", "-p"], text=True)
    candidates = {
        Path(line.split("=>", 1)[1].strip()).resolve()
        for line in cache.splitlines()
        if line.strip().startswith("libz.so.1 ") and "=>" in line
    }
    if len(candidates) != 1:
        raise SystemExit(
            "expected one unambiguous system libz.so.1 in the loader cache"
        )
    libz = candidates.pop()
    modules = {v: build / ("Compress-Raw-Zlib-" + v) for v in ("2.103", "2.105")}
    extensions = {
        v: p / "blib/arch/auto/Compress/Raw/Zlib/Zlib.so" for v, p in modules.items()
    }
    paths = {
        "perl": perl,
        "system-zlib": libz,
        **{"extension-" + v: p for v, p in extensions.items()},
    }
    hashes = {k: sha256(p) for k, p in paths.items()}
    objects = {
        k: {"root": "filesystem", "path": str(p).lstrip("/"), "sha256": hashes[k]}
        for k, p in paths.items()
    }
    roots = {"filesystem": "/"}
    environment = {
        "PATH": os.environ["PATH"],
        "LC_ALL": "C",
        "PERL5LIB": "",
        "PERL5OPT": "",
        "PERL_DL_NONLAZY": "0",
    }
    metadata = {
        "classification": "real integration case, not exact upstream symptom reproduction",
        "build_root": str(build),
        "repetitions": args.repetitions,
        "image": os.environ.get("VALIDATION_IMAGE", "unspecified"),
        "runner": os.environ.get("VALIDATION_RUNNER", "local"),
        "global_load_flag": 1,
        "profile": "lazy; no LD_BIND_NOW override",
        "harness_environment": environment,
        "inputs": {k: {"path": str(p), "sha256": hashes[k]} for k, p in paths.items()},
        "workload_sha256": sha256(workload),
        "binary_sha256": sha256(binary),
    }
    save(out / "provenance.json", metadata)
    for name, command in {
        "packages": ["dpkg-query", "-W", "perl", "zlib1g"],
        "system-zlib-elf": ["readelf", "-h", "-d", "-n", "-Ws", str(libz)],
        "perl-dependencies": ["ldd", str(perl)],
    }.items():
        (out / (name + ".txt")).write_text(subprocess.check_output(command, text=True))

    errors, runs = [], []

    def expect(condition, message):
        if not condition:
            errors.append(message)

    def invoke(name, config, expected, operation="check", extra=(), finding=None):
        config_path, report = out / (name + ".config.json"), out / (name + ".json")
        save(config_path, config)
        command = [
            str(binary),
            operation,
            "--config",
            str(config_path),
            "--report",
            str(report),
            *map(str, extra),
        ]
        result = subprocess.run(
            command, env=environment, text=True, capture_output=True
        )
        (out / (name + ".log")).write_text(result.stdout + result.stderr)
        r = json.loads(report.read_text()) if report.exists() else {}
        outcome = r.get("outcome")
        if operation == "capture" and r.get("kind") == "observation":
            outcome = "PASS" if r["capture"]["complete"] else "UNRESOLVED"
        observation = r if r.get("kind") == "observation" else r.get("observation", {})
        runs.append(
            {
                "name": name,
                "command": command,
                "exit_code": result.returncode,
                "expected_exit": expected,
                "outcome": outcome,
                "report": report.name,
                "binding_verdict": r.get("binding_verdict"),
                "workload_exit": observation.get("workload", {}).get("exit_code"),
                "findings": [f["id"] for f in r.get("findings", [])],
            }
        )
        save(out / "runs.json", runs)
        want = {0: "PASS", 1: "FAIL", 2: "UNRESOLVED"}[expected]
        expect(
            result.returncode == expected and outcome == want,
            f"{name}: got {result.returncode}/{outcome}, expected {expected}/{want}",
        )
        if finding:
            expect(
                any(f["id"] == finding for f in r.get("findings", [])),
                f"{name}: missing {finding}",
            )
        print(
            f"{name}: {outcome} (CLI {result.returncode}, expected {expected})",
            flush=True,
        )
        return report, r

    configs, reports, stdouts = {}, {}, []
    expected_payload = (b"BindWitness: real Perl zlib round trip\x00\x01" * 64).hex()
    for v in modules:
        reference = "extension-" + v
        prefix = "" if v == "2.103" else "Perl_crz_"
        selectors = [
            {
                "reference": reference,
                "symbol": prefix + s,
                "trace_version": "",
                "providers": [reference],
                "required": True,
            }
            for s in ("deflate", "inflate", "zlibVersion")
        ]
        for order in ("bundled-first", "system-first"):
            key = v + "-" + order
            c = {
                "schema_version": 1,
                "command": [
                    str(perl),
                    str(workload),
                    order,
                    str(extensions[v]),
                    str(libz),
                    str(modules[v]),
                ],
                "working_directory": str(out),
                "environment": environment,
                "deadline": "10s",
                "limits": {
                    "stdout_bytes": 16384,
                    "stderr_bytes": 16384,
                    "trace_bytes": 2097152,
                },
                "roots": roots,
                "objects": objects,
                "selectors": selectors,
            }
            configs[key] = c
            expected = 1 if v == "2.103" and order == "system-first" else 0
            reports[key] = []
            for repeat in range(1, args.repetitions + 1):
                name = key + f"-{repeat:02d}"
                report, r = invoke(
                    name,
                    c,
                    expected,
                    finding="PROVIDER_NOT_ALLOWED" if expected else None,
                )
                reports[key].append(report)
                # Offline replay must preserve the complete result, not just exit status.
                _, offline = invoke(
                    name + "-offline", c, expected, extra=["--observation", report]
                )
                for field in ("outcome", "binding_verdict", "coverage", "findings"):
                    expect(
                        offline.get(field) == r.get(field),
                        f"{name}: offline {field} differs",
                    )
                o = r.get("observation")
                if o is None:
                    errors.append(name + ": no observation")
                    continue
                w = o["workload"]
                expect(
                    w["exit_code"] == 0 and w["completed"], name + ": workload failed"
                )
                expect(
                    o["capture"]["complete"] and not o["capture"]["issues"],
                    name + ": incomplete capture",
                )
                expect(
                    all(x["observed"] > 0 for x in r["coverage"])
                    and len(r["coverage"]) == 3,
                    name + ": incomplete selected coverage",
                )
                expect(
                    r["binding_verdict"] == ("FAIL" if expected else "PASS"),
                    name + ": wrong binding verdict",
                )
                expect(
                    json.loads(w["stdout"])
                    == {"round_trip": True, "payload_hex": expected_payload},
                    name + ": application result differs",
                )
                stdouts.append(w["stdout"])
                runtime = json.loads(w["stderr"])
                expect(
                    str(runtime["module_version"]) == v
                    and runtime["zlib_compile"] == "1.2.12",
                    name + ": unexpected module/header",
                )
                expect(
                    runtime["zlib_runtime"] == ("1.2.13" if expected else "1.2.12"),
                    name + ": unexpected runtime zlib",
                )
                actual_provider = libz if expected else extensions[v]
                for s in selectors:
                    events = [
                        b
                        for b in o["bindings"]
                        if b["reference"] == str(extensions[v])
                        and b["symbol"] == s["symbol"]
                    ]
                    expect(
                        bool(events)
                        and all(b["provider"] == str(actual_provider) for b in events),
                        name + ": unexpected selected provider",
                    )
                    for b in events:
                        expect(
                            b["provider"] in o["trace"][b["trace_line"] - 1],
                            name + ": missing raw witness",
                        )
                for logical, p in paths.items():
                    identity = o["objects"].get(str(p), {})
                    expect(
                        identity.get("sha256") == hashes[logical]
                        and identity.get("resolved_path") == str(p)
                        and identity.get("stability") == "pre_post_unchanged",
                        name + ": artifact identity drift: " + logical,
                    )

    expect(
        len(set(stdouts)) == 1,
        "application stdout differs between orders/versions/repetitions",
    )
    for v in modules:
        c = configs[v + "-bundled-first"]
        cc = {
            "schema_version": 1,
            "left_roots": roots,
            "right_roots": roots,
            "objects": objects,
            "selectors": copy.deepcopy(c["selectors"]),
        }
        for s in cc["selectors"]:
            s["providers"] = ["extension-" + v, "system-zlib"]
        for repeat, (left, right) in enumerate(
            zip(reports[v + "-bundled-first"], reports[v + "-system-first"]), 1
        ):
            _, r = invoke(
                v + f"-order-compare-{repeat:02d}",
                cc,
                1 if v == "2.103" else 0,
                "compare",
                ["--left", left, "--right", right],
                "PROVIDER_CHANGED" if v == "2.103" else None,
            )
            expect(
                not r.get("artifact_changes"),
                v + ": order comparison changed artifacts",
            )
        for order in ("bundled-first", "system-first"):
            history = reports[v + "-" + order]
            invoke(
                v + "-" + order + "-repeat-compare",
                cc,
                0,
                "compare",
                ["--left", history[0], "--right", history[-1]],
            )

    positive = reports["2.103-bundled-first"][0]
    c = copy.deepcopy(configs["2.103-bundled-first"])
    c["selectors"] = [
        {
            "reference": "extension-2.103",
            "symbol": "inflateBack",
            "trace_version": "",
            "providers": ["extension-2.103"],
            "required": True,
        }
    ]
    missing, r = invoke(
        "missing-required",
        c,
        2,
        extra=["--observation", positive],
        finding="REQUIRED_NOT_OBSERVED",
    )
    expect(
        r.get("binding_verdict") == "UNRESOLVED",
        "missing coverage became a binding PASS",
    )
    cc = {
        "schema_version": 1,
        "left_roots": roots,
        "right_roots": roots,
        "objects": objects,
        "selectors": c["selectors"],
    }
    _, r = invoke(
        "missing-compare",
        cc,
        2,
        "compare",
        ["--left", positive, "--right", missing],
        "REQUIRED_NOT_OBSERVED",
    )
    expect(
        not any(f["id"] == "PROVIDER_CHANGED" for f in r.get("findings", [])),
        "missing coverage became a change witness",
    )
    # A genuinely unexercised workload side, not edited or removed raw evidence.
    idle = copy.deepcopy(configs["2.103-bundled-first"])
    idle["command"].append("load-only")
    idle_report, r = invoke("load-only", idle, 2, finding="REQUIRED_NOT_OBSERVED")
    expect(
        r.get("observation", {}).get("workload", {}).get("exit_code") == 0,
        "load-only workload failed",
    )
    expect(
        r.get("binding_verdict") == "UNRESOLVED",
        "unexercised workload became a binding PASS",
    )
    cc["selectors"] = idle["selectors"]
    _, r = invoke(
        "load-only-compare",
        cc,
        2,
        "compare",
        ["--left", positive, "--right", idle_report],
        "COMPARE_COVERAGE_GAP",
    )
    expect(
        not any(f["id"] == "PROVIDER_CHANGED" for f in r.get("findings", [])),
        "unexercised workload became a change witness",
    )
    # A failure of either mandatory load must stop the application, including
    # the prefixed control where bindings alone cannot prove system libz loaded.
    for key, original in configs.items():
        for object_name, argument in (("extension", 3), ("system-zlib", 4)):
            c = copy.deepcopy(original)
            absent = out / ("absent-" + object_name + ".so")
            c["command"][argument] = str(absent)
            _, r = invoke(
                key + "-missing-" + object_name,
                c,
                2,
                finding="REQUIRED_NOT_OBSERVED",
            )
            o = r.get("observation", {})
            w = o.get("workload", {})
            expect(
                w.get("completed") and w.get("exit_code") not in (None, 0),
                key + ": mandatory load failure reported successful workload",
            )
            expect(
                w.get("stdout") == "",
                key + ": load failure printed application success",
            )
            expect(
                "dl_load_file(" + str(absent) + "):" in w.get("stderr", ""),
                key + ": missing original loader error",
            )
            expect(
                o.get("capture", {}).get("complete") and not o["capture"]["issues"],
                key + ": load failure lost capture evidence",
            )
    # capture saves evidence without evaluating even the known violated contract.
    c = configs["2.103-system-first"]
    captured, _ = invoke("capture-system-first", c, 0, "capture")
    invoke(
        "capture-offline-check",
        c,
        1,
        extra=["--observation", captured],
        finding="PROVIDER_NOT_ALLOWED",
    )
    for logical, p in paths.items():
        expect(
            sha256(p) == hashes[logical],
            "artifact changed across scenarios: " + logical,
        )
    save(out / "validation-errors.json", errors)
    if errors:
        raise SystemExit("\n".join(errors))
    print(
        f"Perl/zlib integration validation passed: {4 * args.repetitions + 10} fresh processes; full reports in {out}"
    )


if __name__ == "__main__":
    main()
