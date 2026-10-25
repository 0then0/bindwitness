# Security policy

## Supported versions

Security fixes target the latest published release. Older versions do not have
separate maintenance branches; upgrade to the latest release before checking
whether a problem is still present. Releases are listed on
[GitHub Releases](https://github.com/0then0/bindwitness/releases).

## Reporting a vulnerability

Email the maintainer privately at
[zheltov_pe@mail.ru](mailto:zheltov_pe@mail.ru), with the subject
`BindWitness security report`. Do not open a public issue or include exploit
information in a public pull request before coordinated disclosure.

Include the affected version or commit, operating system, architecture, glibc
version, reproduction steps, impact and a minimal example if safe to share.
Remove credentials and unrelated confidential data. Agree on a suitable transfer
method before sending sensitive workload artifacts. The maintainer will discuss
reproduction, remediation and disclosure with the reporter; response and fix
schedules depend on availability and the issue's impact.

## Trust boundaries

BindWitness executes the command in its configuration with the invoking user's
permissions. It is not a sandbox. Run only trusted workloads and configurations,
and use isolated test environments for applications that should not access your
normal files or credentials. Workloads must not forge loader diagnostics.

Saved observations and reports are trusted evidence inputs, not authenticated
attestations. Artifact hashes describe files inspected on disk; they do not
prove mapped process bytes or rule out every replacement-and-restoration race.
A PASS applies only to the declared, observed binding scope and does not establish
general ABI compatibility or application safety. See the
[report format and capture limitations](docs/report-format.md).

Loader traces and reports can include absolute paths, arguments, environment
values and workload stdout/stderr. Inspect them before sharing or uploading them
to public CI artifacts. For a public reproduction, capture a sanitized workload
rather than modifying raw evidence and presenting it as an original observation.

Download release assets from the project's GitHub Releases and verify the
selected archive against `SHA256SUMS` before extraction. Checksums detect content
mismatches; they are not an independent signature or authentication channel.
