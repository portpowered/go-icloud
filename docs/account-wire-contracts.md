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

All examples in this schema are invented. The 18 public account replay
scenarios contain 24 synthetic exchanges. The contract suite checks their
JSON response shapes using the actual operation's status and media schema,
checks explicit operation ownership, validates ten schema
examples, and checks known required fields, types and byte-count bounds.
Binary member photos remain uninterpreted bytes, including empty content.
These counts demonstrate wire-contract checks, not SDK semantic replay or
live endpoint coverage (LIB-07).

The generated Go client now executes all 18 synthetic account scenarios against
strict paired replay. Requests originate from portable initial account state
and operation inputs, rather than from the expected request. Result checks bind
device records, family names, storage bytes, plan JSON, and member-photo
status/headers/bytes; provider failures bind the reference status/body/message.
Both route-client and wire-model generation use their checked-in configs in
drift tests. Successful member photos use an adapter that reads and closes raw
response bodies: the generic generated JSON-default parser can mistake binary
success labeled `application/json` for an error object. This is fixture/source
compatibility, not a claim about observed live photo content types.

These tests are generated request/response interoperability. They do not prove
the public SDK's result types, error types, session cache semantics or account
isolation; those layers still need implementation and the same replay tests.

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
gates, public projection schemas and account SDK operations remain work in
progress. This document does not declare complete account schema acceptance.
Other services and physical security-key behavior are outside this account
contract milestone. Generic invalid caller inputs do not define the selected
endpoint replay coverage target.
