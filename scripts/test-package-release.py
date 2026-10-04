#!/usr/bin/env python3
"""Check both archive contents and checksum preservation without running binaries."""

import hashlib
import pathlib
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile

repo = pathlib.Path(__file__).resolve().parents[1]
version = re.search(
    r'^const Version = "([0-9.]+)"$',
    (repo / "internal/witness/types.go").read_text(),
    re.MULTILINE,
)[1]


def check_packages(amd64, arm64, out):
    binaries = {"amd64": amd64, "arm64": arm64}
    script = repo / "scripts/package-release.sh"

    def package(arch):
        subprocess.run(
            ["sh", str(script), str(binaries[arch]), arch, str(out)], check=True
        )

    def checksums():
        lines = (out / "SHA256SUMS").read_text().splitlines()
        entries = dict(line.split(None, 1)[::-1] for line in lines)
        assert len(lines) == len(entries), "Duplicate checksum entries"
        for name, digest in entries.items():
            assert hashlib.sha256((out / name).read_bytes()).hexdigest() == digest, name
        return entries

    package("amd64")
    first = checksums()
    package("arm64")
    combined = checksums()
    first_archive = f"bindwitness-{version}-linux-amd64.tar.gz"
    assert combined[first_archive] == first[first_archive], (
        "Second architecture overwrote first checksum"
    )
    package("arm64")
    assert len(checksums()) == 2, "Repackaging must replace its checksum entry"
    expected_files = [
        "LICENSE",
        "README.md",
        "docs/config.schema.json",
        "docs/report.schema.json",
        "docs/compare.schema.json",
        "docs/assets/bindwitness.svg",
    ]
    for arch, binary in binaries.items():
        name = f"bindwitness-{version}-linux-{arch}"
        with tarfile.open(out / (name + ".tar.gz")) as archive:
            assert (
                archive.extractfile(name + "/bindwitness").read() == binary.read_bytes()
            )
            for path in expected_files:
                assert (
                    archive.extractfile(name + "/" + path).read()
                    == (repo / path).read_bytes()
                )
    def output_snapshot():
        return {path.name: path.read_bytes() for path in out.iterdir() if path.is_file()}

    before = output_snapshot()
    wrong = subprocess.run(
        ["sh", str(script), str(amd64), "arm64", str(out)], check=False
    )
    assert wrong.returncode != 0, "Mismatched ELF machine was accepted"
    assert output_snapshot() == before
    # Deliberately altered copies are packaging regression fixtures. They do not
    # replace or certify the native metadata of either supplied release input.
    with tempfile.TemporaryDirectory(prefix="bindwitness-invalid-package-") as stage:
        for case in (
            "stale-version",
            "missing-version",
            "missing-checksum",
            "missing-checksum-entry",
            "invalid-checksum",
            "duplicate-checksum",
            "changed-binary",
        ):
            directory = pathlib.Path(stage) / case
            directory.mkdir()
            binary = directory / amd64.name
            shutil.copyfile(amd64, binary)
            shutil.copyfile(amd64.parent / "version.txt", directory / "version.txt")
            shutil.copyfile(amd64.parent / "SHA256SUMS", directory / "SHA256SUMS")
            if case == "stale-version":
                stale = "0.1.0" if version != "0.1.0" else "0.0.0"
                (directory / "version.txt").write_text(f"bindwitness {stale}\n")
            elif case == "missing-version":
                (directory / "version.txt").unlink()
            elif case == "missing-checksum":
                (directory / "SHA256SUMS").unlink()
            elif case == "missing-checksum-entry":
                digest = hashlib.sha256(binary.read_bytes()).hexdigest()
                (directory / "SHA256SUMS").write_text(f"{digest}  another-binary\n")
            elif case == "invalid-checksum":
                (directory / "SHA256SUMS").write_text(f"{'X' * 64}  {binary.name}\n")
            elif case == "duplicate-checksum":
                checksum = directory / "SHA256SUMS"
                checksum.write_text(checksum.read_text() * 2)
            else:
                with binary.open("ab") as stream:
                    stream.write(b"changed after validation")
            rejected = subprocess.run(
                ["sh", str(script), str(binary), "amd64", str(out)], check=False
            )
            assert rejected.returncode != 0, f"{case} was accepted"
            assert output_snapshot() == before, f"{case} changed release output"
    print(
        "Both release archives, metadata and ELF rejection, and common checksums passed"
    )


if __name__ == "__main__":
    if len(sys.argv) not in (3, 4):
        sys.exit(
            "usage: test-package-release.py AMD64_BINARY ARM64_BINARY [OUTPUT_DIR]"
        )
    amd64, arm64 = (pathlib.Path(value).resolve() for value in sys.argv[1:3])
    if len(sys.argv) == 4:
        check_packages(amd64, arm64, pathlib.Path(sys.argv[3]).resolve())
    else:
        with tempfile.TemporaryDirectory(prefix="bindwitness-packaging-") as output:
            check_packages(amd64, arm64, pathlib.Path(output))
