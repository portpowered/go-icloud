# Reference endpoint occurrence audit

Run `make endpoint-coverage` from the repository root to list portable HTTP
occurrences and missing routes. This Go audit (LIB-13) reads checked-in synthetic
fixtures and requires their source pin to match the draft inventory and
`tools/reference/source.json`. `make check` and CI run its diagnostic mode.

For JSON with each fixture and zero-based exchange index:

```powershell
go run ./tools/endpointcoverage
```

Use `-root` to select a repository directory. Strict occurrence mode returns
failure on a missing inventoried HTTP route or unmatched exchange:

```powershell
go run ./tools/endpointcoverage -summary -require-covered
```

That strict command currently fails. Diagnostic success means the inputs were
audited successfully; final endpoint acceptance remains open.

## Matching and limitations

The audit matches service, method and escaped path, with one path segment for
declared `dsid`, `zone` and `step` placeholders. Unknown/malformed templates,
duplicate entries and ambiguous bindings fail. Dynamic URLs require the same
HTTPS authority and escaped path in an earlier JSON response or caller input.
Album-location prefixes similarly require a referenced location. A future
response cannot supply an earlier request's URL.

This establishes occurrence, not the exact response field/helper supplying a
URL, its query/header/body, or authority-selection policy. The eventual source
and schema gate must prove those bindings. Paired reference replay already
checks complete prepared exchanges, semantic outcomes, order and consumption;
this audit does not replace it. HTTP counts classify statuses 200–399, statuses
at least 400 and other statuses; a 200 response may contain a provider record
error.

Occurrences are synthetic evidence, separate from private captured reads. They
do not prove zero/one/many, pagination, transport, mutation or lifecycle matrices.
The separate TLS/WebSocket inventory entry is excluded. Function and branch
measurement remains `make reference-coverage`.

## Current occurrences and scoped gap

The current audit maps 456 scenarios and 1030 exchanges to 74/75 HTTP routes,
with no unmatched exchange. The sole missing occurrence is physical security-key
verification, explicitly deferred. Strict mode continues to report that gap.

The initial 373-scenario, 804-exchange audit mapped 66/75 routes. Source inspection
and 40 additional scenarios cover Photos container changes, shared lookup and
library operations, and Reminders zone discovery. Shared library asset discovery
also revealed an active shared batch-count route, now included in the inventory.

Reminders database changes is retained under `dormant_definitions`, outside the
active denominator. Inspection of the pinned service, adapter, public exports
and CLI found no call or exposed operation; only Photos delegates to the common
container helper. Reminders zone discovery is exposed by the reference CLI and
remains included. Introducing a Reminders database-change operation requires
restoring its active entry and binding schemas and replay cases (LIB-18).

The inventory is a draft with pending source/schema/model gates. Resolve actual
send sites and active dependencies, correct dormant entries, and bind applicable
functional cases before treating occurrences as complete endpoint coverage.
