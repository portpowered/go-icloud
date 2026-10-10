# Contributor verification

Run `make lint` and `make check` from the root before accepting a change.
The root Makefile owns SDK and standalone CLI formatting, tidy, lint, build,
race tests, schema generation drift, source pin, wire inventory and replay gates.
Use the exact release commit for final checks (LIB-07, LIB-13, LIB-18).
For the CLI, set `GOWORK=off` and use its published SDK dependency; an integration
workspace pass is a separate receipt and does not prove the final public pin.
Current receipts and unresolved gates are in the
[completion matrix](completion-matrix.md).

The portable replay corpus is evaluated independently by the pinned Python
reference and the public Go SDK. Matcher interoperability, endpoint occurrence
counts and Source function coverage each answer a narrower question; none proves
public Go behavior by itself. Run `make endpoint-coverage` for current counts and
paired replay and schema tests for emitted requests and public results.
Custom compiler provenance engines, exhaustive ownership proofs and symbolic lineage
gates are not acceptance requirements; see library standard 4.
See [reference capture](reference-capture.md) and
[paired replay](../tests/replay/README.md) for artifact rules.

Coverage has separate replay, unit and combined profiles. Replay must independently
meet the library threshold; a high combined percentage cannot substitute for
paired request/result coverage. `tools/coverage` excludes generated files from
the published denominator. The CLI is a separate Go module with its own profiles
and checks. The Pages workflow publishes filtered combined HTML and a badge;
it does not waive the replay gate.

`go run ./tools/docsite` examines every exported HTML page, local assets and
anchors, canonical schema documentation links and expected visible content.
Its subprocess controls reject missing destinations, missing anchors, absent
schema targets, fallback guide pages and markers present only inside scripts.
See [documentation publishing](website.md) for renderer inputs and deployment
requirements. Inspect example values, alternatives, required fields and request
snippets in the rendered reference as part of independent review.

Keep routine CI offline and deterministic. Live provider tests are opt-in and
require an explicit account boundary. Preserve synthetic, captured and historical
evidence classifications; never commit private credentials, captures or binaries.
Review verdicts and unresolved acceptance items belong in the
[independent review](independent-review.md), [migration checklist](migration-checklist.md)
and [completion matrix](completion-matrix.md), rather than another checklist here.
