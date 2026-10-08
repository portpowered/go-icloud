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

All examples in this schema are invented. The 34 public account replay
scenarios contain 40 synthetic exchanges. The contract suite checks their
JSON response shapes using the actual operation's status and media schema,
checks explicit operation ownership, validates ten schema
examples, and checks known required fields, types and byte-count bounds.
Binary member photos remain uninterpreted bytes, including empty content.
These counts demonstrate wire-contract checks, not SDK semantic replay or
live endpoint coverage (LIB-07).

The generated Go client now executes all 34 synthetic account scenarios against
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
wire types (API-02). Contract tests validate its ten invented examples, regenerate
its models, and compile an independent consumer module.

`GetAccountDevices` performs fresh requests with caller-owned `AuthContext`.
Concurrent two-account paired replays bind separate identities, cookies and
responses, then repeat each request to reject shared cache or cookie updates
(API-03, API-13). The caller receives response headers for session updates.
Typed errors retain exact body/headers and causes without displaying private
provider text. Unit controls cover cancellation, timeouts, unknown metadata,
HTTP failure classes, invalid provider bodies and falsy error fields.

All 34 account scenarios now execute the public SDK as well as the generated
client. Family/photo flows obtain member DSIDs from the public family result.
Storage binds usage, quota, media and unknown account metadata. Plan summaries
bind the opaque JSON value, including arrays and null, for global/China gateways.
New reference-derived cases cover omitted families, named nulls, provider refusal,
zero byte totals, usage above quota and multiple media categories. They pass the
pinned Python reference offline. Shared response handling reads/closes every body;
unit controls verify read/close causes suppress output across all four new methods.

Non-generated coverage includes `pkg/icloud` and handwritten `internal` code,
including the transport and binary-photo adapter. Replay: 284/348 statements
(81.6%); unit: 287/348 (82.5%); combined: 321/348 (92.2%). These historical account-milestone measurements
are separate (LIB-07) and describe its five account operations. See
[Drive contracts](drive-wire-contracts.md) for current coverage including the two
Drive reads. Neither measure establishes all selected service coverage. Constructor validation and generic malformed caller
inputs are outside the endpoint replay target. Run `make sdk-coverage` for the
three measurements. `make check` and blocking CI require at least 80% replay
and combined coverage; CI saves separate profiles. Schema-owned protocol
constants include inline operation parameters and have syntax/collision checks.

## Unknown values and remaining work

Additional provider fields retain raw JSON, including nulls and integers beyond
floating-point precision. The reference does not define concrete subscription
plan fields; plan summaries remain raw JSON. Family properties whose actual
types have not been observed also remain raw JSON rather than inferred types
(SCHEMA-03, SCHEMA-07).

Generated nullable wrappers preserve both named explicit nulls and omitted
values. Media usage entries require `mediaKey`, which the reference constructor
reads unconditionally. Regression checks cover both contracts.

Unknown JSON and subscription values explicitly allow every JSON type, including
null and arrays containing null. An OpenAPI 3.0 union keeps raw additional fields
compatible with the pinned generator while permitting null in schema validation.
Named nullable values retain separate absent/explicit-null wrappers.

The error object describes currently known JSON fields. Authentication-specific
error classification, complete failure/framing contracts and the full
source-to-schema endpoint gate remain work in progress. This document does not
declare complete account schema acceptance.
Other services and physical security-key behavior are outside this account
contract milestone. Generic invalid caller inputs do not define the selected
endpoint replay coverage target.
