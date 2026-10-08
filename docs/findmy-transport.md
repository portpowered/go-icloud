# Find My transport replay

The internal transport now executes the seven selected Find My routes through
schema-generated request builders. Account and header state belongs to each call;
the reusable client retains no credentials or cookie jar. Initialization and
refresh use their distinct generated contexts. Erase-token lookup uses the setup
origin and sends no account query parameters. Device commands are sent once and
return acknowledgement evidence without claiming physical completion (API-11).

Thirty-seven portable implementation-derived scenarios contain 70 paired
exchanges. The Go transport driver constructs command values from scenario
inputs and refresh contexts from actual preceding replies. It verifies exact
status, response headers, body bytes and cookie scope, and checks typed read/token
projections against the complete received JSON. The existing matcher binds each
request's origin, path, method, query, headers, framing and body and requires
complete consumption. There is no network fallback (LIB-05).

These are transport checks. The driver follows the fixture's sequence of route
calls; it does not implement or verify production session orchestration. Local
capability refusals, missing-token guards, family polling, cache retention and
monitor lifecycle still require the public SDK driver. Public SDK semantic
coverage remains 0/37 Find My scenarios; the transport numerator does not replace
it. The account and Drive SDK replay inventories remain 34/34 and 79/79.

## Ordered provider context

Independent review found that the canonical model's map serialization sorts
opaque server-context keys. The pinned Python source preserves their insertion
order when sending the next refresh. A new strict Source and Go regression gives
the provider context ordered `z,a,theftLoss`; refresh must keep that order while
setting `theftLoss` to null.

The schema-owned `FindMyRefreshContext` raw JSON type is bound to the canonical
`FindMyServerContext` schema. The transport retains both typed response fields
and the ordered context from received bytes. It clears only the generated
top-level `theftLoss` key before encoding a refresh. Caller context and response
bytes do not alias that stored context. Unknown nested JSON, nulls and large
integers remain intact (SCHEMA-07, SCHEMA-10). No fixture matcher was relaxed.

## Evidence and limits

All fixtures remain synthetic and implementation-derived from timlaing/pyicloud
commit `e2e44ab875d47dab4475096021da60030f26c35e`. The new ordered-context case
passes the actual pinned Source with networking forbidden. Unit controls cover
invalid discovery/token replies, missing token fields, explicit null session
tokens, request cancellation, caller header ownership and context ownership.
No live command was performed or authorized; live exploration remains read-only.

The full source/model/runtime acceptance gates, native Go authentication, public
Find My session, Photos and reminders ports, live Go integration, documentation
publication and package release remain open. Scope remains the selected endpoints
and their reachable behavior rather than unrelated provider services or generic
caller-input validation.

Final `make lint` and `make check` pass, including 67 Python test methods and
Go race/contracts/drift checks. Separate handwritten SDK/internal coverage is
1219/1457 replay statements (83.7%), 780/1457 unit (53.5%) and 1293/1457 combined
(88.7%). Fresh reference measurement remains 576/905 entered functions,
3195/5385 body statements and 973/2076 branch exits. These diagnostics do not
establish full functional completeness; the selected-function inventory and
public SDK scenario parity remain required acceptance evidence (LIB-07).
