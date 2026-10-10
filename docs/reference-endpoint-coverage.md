# Reference endpoint occurrence audit

Run `make endpoint-coverage` from the repository root to list portable HTTP
occurrences and missing routes. This Go audit (LIB-13) reads checked-in synthetic
fixtures and requires their source pin to match the draft inventory and
`tools/reference/source.json`. `make endpoint-coverage`, `make check` and CI
run strict occurrence mode with `-summary -require-covered`.

For diagnostic JSON with each fixture and zero-based exchange index:

```powershell
go run ./tools/endpointcoverage
```

Use `-root` to select a repository directory. Strict occurrence mode returns
failure on a missing inventoried HTTP route or unmatched exchange:

```powershell
go run ./tools/endpointcoverage -summary -require-covered
```

Diagnostic success means the inputs were audited successfully. Strict success
means every active inventoried HTTP route has an occurrence; it does not prove
SDK request/result parity or channel and binary behavior.

## Matching and limitations

The audit matches service, method and escaped path, with one path segment for
declared `dsid`, `zone` and `step` placeholders. Unknown/malformed templates,
duplicate entries and ambiguous bindings fail. Dynamic URLs require the same
HTTPS authority and escaped path in an earlier JSON response or caller input.
Album-location prefixes similarly require a referenced location. A future
response cannot supply an earlier request's URL.

This establishes occurrence, not the exact response field/helper supplying a
URL, its query/header/body, or authority-selection policy. The source/model
provenance and socket gates verify their own declared bindings; their success
must be assessed separately from occurrence coverage. Paired reference replay
checks complete prepared exchanges, semantic outcomes, order and consumption;
this audit does not replace it. HTTP counts classify statuses 200–399, statuses
at least 400 and other statuses; a 200 response may contain a provider record
error.

Occurrences are synthetic evidence, separate from private captured reads. They
do not prove zero/one/many, pagination, transport, mutation or lifecycle matrices.
The separate TLS/WebSocket inventory entry is excluded. Function and branch
measurement remains `make reference-coverage`.

## Occurrences and active inventory

Run the audit for current scenario, exchange and route totals. The corpus grows
as reference behavior is ported; fixed totals here would become stale. Physical
security-key verification requires its own explicit boundary and evidence; an
HTTP occurrence alone cannot verify a hardware interaction.

Reminders database changes is retained under `dormant_definitions`, outside the
active denominator. Inspection of the pinned service, adapter, public exports
and CLI found no call or exposed operation; only Photos delegates to the common
container helper. Reminders zone discovery is exposed by the reference CLI and
remains included. Introducing a Reminders database-change operation requires
restoring its active entry and binding schemas and replay cases (LIB-18).

The runtime wire gate independently binds actual send sites, active dependencies
and canonical models. Keep dormant entries justified by source inspection and
bind functional cases before treating occurrences as complete service coverage.
