# Account wire contracts

`api/external/account.openapi.yaml` describes the five selected account routes:
account devices, family members, member photos, storage usage and plan summary.
`pkg/dependencymodels/account/models.gen.go` is generated with oapi-codegen v2.8.0.
Its canonical definitions live in `api/external/account-models.openapi.yaml`.
The route schema references those definitions; generated request builders in
`internal/accountapi/client.gen.go` import the shared models instead of
duplicating them. The generator runtime is pinned in go.mod.
Run `make generate-api` to regenerate it. Contract tests regenerate the models
and reject drift (SCHEMA-16).

## Evidence

The reference is timlaing/pyicloud at
`e2e44ab875d47dab4475096021da60030f26c35e`, particularly
`pyicloud/services/account.py` and session setup in `pyicloud/base.py`.
Known device, payment-method and storage field names and JSON types were also
checked against private account captures. Only field names and types informed
the contract; private values, cookies and captures are not published
(SCHEMA-08, SCHEMA-12). Captured Accept negotiation includes `*/*`, so the
schema accepts it alongside the synthetic JSON negotiation (SCHEMA-09).

All examples in this schema are invented. The 26 public account replay
scenarios contain 32 synthetic exchanges. The contract suite checks their
JSON response shapes using the actual operation's status and media schema,
checks explicit operation ownership, validates ten schema
examples, and checks known required fields, types and byte-count bounds.
Binary member photos remain uninterpreted bytes, including empty content.
These counts demonstrate wire-contract checks, not SDK semantic replay or
live endpoint coverage (LIB-07).

The generated Go client now executes all 26 synthetic account scenarios against
strict paired replay. Requests originate from portable initial account state
and operation inputs, rather than from the expected request. Result checks bind
device records, family names, storage bytes, plan JSON, and member-photo
status/headers/bytes; provider failures bind the reference status/body/message.
Both route-client and wire-model generation use their checked-in configs in
drift tests. Successful member photos use an adapter that reads and closes raw
response bodies: the generic generated JSON-default parser can mistake binary
success labeled `application/json` for an error object. This is fixture/source
compatibility, not a claim about observed live photo content types.

The generated client tests establish request/response interoperability. A separate
public SDK driver now runs all eleven account-device scenarios, including empty,
one and multiple devices, HTTP 503, and seven HTTP 200 provider-error envelopes.
The same new synthetic cases pass the pinned Python reference with no network.
Public projections come from `api/client-models.openapi.yaml`, independently of
wire types (API-02). Contract tests validate its two invented examples, regenerate
its models, and compile an independent consumer module.

`GetAccountDevices` performs fresh requests with caller-owned `AuthContext`.
Concurrent two-account paired replays bind separate identities, cookies and
responses, then repeat each request to reject shared cache or cookie updates
(API-03, API-13). The caller receives response headers for session updates.
Typed errors retain exact body/headers and causes without displaying private
provider text. Unit controls cover cancellation, timeouts, unknown metadata,
HTTP failure classes, invalid provider bodies and falsy error fields.

Non-generated coverage includes `pkg/icloud` and handwritten `internal` code,
including the transport and binary-photo adapter. Replay: 150/187 statements
(80.2%); unit: 165/187 (88.2%); combined: 171/187 (91.4%). These measurements
are separate (LIB-07). They describe the current account-device SDK and transport
milestone, not coverage of all selected iCloud services. Constructor validation
and generic malformed caller inputs are outside the endpoint replay target.
Run `make sdk-coverage` for the three measurements. `make check` and blocking CI
require at least 80% replay and combined coverage; CI saves separate profiles.
Schema-owned protocol constants have syntax/collision checks (SCHEMA-10).
The remaining four account operations still have generated-client tests only.

## Unknown values and remaining work

Additional provider fields retain raw JSON, including nulls and integers beyond
floating-point precision. The reference does not define concrete subscription
plan fields; plan summaries remain raw JSON. Family properties whose actual
types have not been observed also remain raw JSON rather than inferred types
(SCHEMA-03, SCHEMA-07).

Generated nullable wrappers preserve both named explicit nulls and omitted
values. Media usage entries require `mediaKey`, which the reference constructor
reads unconditionally. Regression checks cover both contracts.

The error object describes currently known JSON fields. Complete authentication
failure variants, non-JSON failures, request framing, source-to-schema endpoint
gates, the remaining public projections and account SDK operations remain work in
progress. This document does not declare complete account schema acceptance.
Other services and physical security-key behavior are outside this account
contract milestone. Generic invalid caller inputs do not define the selected
endpoint replay coverage target.
