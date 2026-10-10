# Reference tooling and evidence

This is contributor material. Customer usage lives in the
[SDK guides](guides/index.mdx). The current implementation status is maintained in
the [completion matrix](completion-matrix.md), not in historical capture receipts.

## Bootstrap

`tools/reference/source.json` pins the Python reference, its historical comparison
source and dependency versions. `setup-reference.ps1` creates clean detached
checkouts under ignored `.reference/` and the locked ignored `.venv/`.
The wrapper rejects a dirty source checkout, revision drift or imports from the
wrong environment. Instrumentation does not modify upstream source.
Reference source licenses remain distinct from this repository's Apache-2.0 license.

For account-owner exploration, run `./icloud.ps1 --help`. Login prompts for a
password and verification code; neither is saved. Private sessions, capture
directories and `result.json` outputs belong under
`%LOCALAPPDATA%/go-icloud/reference`. The recorder does not sanitize arbitrary
personal data; never copy private captures into the repository.

The private capture CLI supports `login` and eight read commands: `status`,
`account`, `devices`, `drive`, `albums`, `photos`, `reminders`, and
`reminder-zones`. Its login helper accepts a two-factor verification code but
does not support legacy two-step completion. `tools/reference/replay.py` replays
captured reads only; it explicitly excludes `login` captures. This workflow does
not expose Photos or Reminders writes, or separate MFA, trust, consent and logout
capture commands. The HTTP recorder does not capture trusted-device bridge TLS/
socket frames or security-key HID traffic.

## Pair format and replay

Each `portos.http-exchange.v1` pair contains source revision, operation, sequence,
method, origin, escaped path, ordered repeated query pairs, headers and exact body
bytes, followed by the response status, headers and body or a typed transport
failure. The ordinary recorder preserves decoded HTTP entities, not transfer
compression/framing bytes. Synthetic `bodyRepresentation: wire` controls exercise
the real content decoder; omitted or `decoded` values preserve recorder semantics.

Interactive secrets use explicit nonempty or verification-code format match rules.
Compressed CRDT documents use an explicit decoded-byte rule; surrounding fields,
valid compression, absence of trailing data and actual request framing remain checked.
See [paired replay mechanics](../tests/replay/README.md).

`ReplayAdapter` matches outbound requests before returning responses, forbids
network fallback, rejects duplicates and requires full consumption. Replays copy
initial private state into a temporary directory and compare complete results or
failure meaning without changing the live session. Stop and join reference monitor
threads before closing their HTTP sessions.

```powershell
.venv/Scripts/python.exe tools/reference/replay.py --all
make reference-coverage
go run ./tools/endpointcoverage -summary
```

Private captured reads, portable synthetic scenarios, document decoding controls
and binary socket transcripts are separate evidence classes (LIB-04, LIB-05,
LIB-07, LIB-12). Paired synthetic results establish pinned-source compatibility;
they do not establish live provider behavior.

Portable paired replay covers the selected SDK Photos, Reminders and
authentication operation families, including writes and authentication flows.
`tools/reference/synthetic.py` executes the HTTP scenarios with network access
forbidden; separate bridge/socket and security-key transcripts exercise framed
traffic. These runners replay fixtures and do not provide additional private
capture commands. Recording support and portable replay coverage therefore have
different scopes (LIB-05, LIB-07, LIB-12).

The [observed read batch](operation-matrix.md) records limited account evidence.
The [endpoint occurrence audit](reference-endpoint-coverage.md) is a diagnostic
over fixtures, not proof of package-resolved send provenance. Final acceptance
requires complete source/model/network gates and independent review.
