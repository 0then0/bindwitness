#!/usr/bin/env python3
import hashlib
import json
import pathlib
import platform
import subprocess
import sys

version = (
    subprocess.check_output(["getconf", "GNU_LIBC_VERSION"], text=True)
    .strip()
    .split()[-1]
)
trace = pathlib.Path("testdata/traces/glibc-" + version + "-echo.trace")
artifacts = {}
for name in [
    "/bin/echo",
    "/lib/ld-linux-aarch64.so.1",
    "/lib/aarch64-linux-gnu/libc.so.6",
]:
    path = pathlib.Path(name).resolve()
    artifacts[name] = {
        "resolved_path": str(path),
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
    }
meta = {
    "source": "unmodified diagnostic stream from real glibc dynamic loader",
    "validation_date": "2026-10-04",
    "command": ["/bin/echo", "probe"],
    "environment_overrides": {"LD_DEBUG": "bindings,files"},
    "glibc": version,
    "architecture": platform.machine(),
    "os_release": pathlib.Path("/etc/os-release").read_text(),
    "base_image": sys.argv[1],
    "trace_sha256": hashlib.sha256(trace.read_bytes()).hexdigest(),
    "artifacts": artifacts,
}
trace.with_suffix(".provenance.json").write_text(json.dumps(meta, indent=2) + "\n")
