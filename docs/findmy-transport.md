# Find My request encoding

Generated request builders cover discovery, refresh, commands and setup-token
lookup. Account and cookie state belongs to each call. Setup-token lookup omits
ordinary account query parameters. Initialization and refresh use distinct named
contexts from the canonical [wire contracts](findmy-wire-contracts.md).

The source preserves opaque server-context insertion order. A regression supplies
ordered `z,a,theftLoss` keys; the next refresh must preserve that order while
clearing only top-level `theftLoss` to null. The schema-owned raw context is bound
to the canonical server-context component. Unknown nested JSON, nulls and large
integers remain intact, and stored bytes do not alias caller input.

Transport replay checks complete method, origin, escaped path, ordered queries,
headers, framing and request bytes before returning exact responses. It constructs
requests from scenario inputs and preceding provider replies rather than copying
expected requests. Public [lifecycle replay](findmy-session.md) separately proves
production polling, cache, capability and teardown orchestration.

See [paired replay mechanics](../tests/replay/README.md). Historical coverage and
milestone sign-offs are retained in Git history and the single
[independent review record](independent-review.md).
