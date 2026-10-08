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

## Current gaps

The initial audit maps 373 scenarios and 804 exchanges to 66/75 HTTP routes,
with no unmatched exchange. Nine routes have no portable HTTP occurrence:

- Physical security-key verification, explicitly deferred.
- Photos private database changes.
- Photos shared query, lookup, modify, zone changes and database changes.
- Reminders zone discovery and database changes.

Reminders database changes requires an active-call audit: its common CloudKit
helper exists, but the high-level Reminders adapter does not expose it. Do not
manufacture an operation solely to satisfy an inventory row. Zone discovery is
used by the reference CLI. Photos shared libraries and exposed container
operations remain relevant.

The inventory is a draft with pending source/schema/model gates. Resolve actual
send sites and active dependencies, correct dormant entries, and bind applicable
functional cases before treating occurrences as complete endpoint coverage.
