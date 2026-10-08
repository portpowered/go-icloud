# Reminders wire contracts

The selected Reminders service uses five CloudKit POST operations: record lookup,
query, modify, zone changes and zone discovery. It also downloads provider-issued
membership assets. `api/external/reminders.openapi.yaml` describes these six
operations and imports shared wire models from `cloudkit-models.openapi.yaml`.
Dormant database-change helpers are excluded from this active Reminders contract.

## Canonical models and generation

The model responsibility is shared CloudKit protocol data, separate from public
Reminders domain projections. Its 64 named schemas include source record/query/
write/error/zone/field-wrapper models and three adapter contracts for arbitrary
unknown JSON, an empty zone-list request and downloaded bytes. Generation produces
`pkg/dependencymodels/cloudkit`, imported by `internal/remindersapi`. Protocol
methods, paths, parameter/field names and fixed values derive from the schemas.
`make generate-api` and drift tests bind all generated files (SCHEMA-10, SCHEMA-16).
An independent consumer module imports and constructs the generated wire types.

Models follow timlaing/pyicloud commit
`e2e44ab875d47dab4475096021da60030f26c35e`, particularly
`pyicloud/common/cloudkit/models.py`, `services/reminders/client.py` and the shared
CloudKit client. The source's automatic schema export silently omits ordinary
CKRecord response members because CKFields is a custom dictionary type. The
canonical document explicitly models all 28 declared record fields, the field
wrapper map, and normal records alongside tombstones/errors in lookup, query,
modify and zone-change replies. A negative control removes each normal-record
member and requires a normal record to fail validation. This guards the repaired
inventory rather than treating the automatic export as complete evidence.

Extensible objects retain raw unknown JSON, including large integers and explicit
nulls. Known nullable values use generated presence wrappers. Field tags, operation
types, query comparators and nested metadata follow the named source models.
Arbitrary app-level field names remain a dictionary of typed or unknown wrappers;
public domain values and document decoding belong in the future service adapter
(SCHEMA-03, SCHEMA-04, SCHEMA-07). Schema documents use JSON syntax, a YAML subset.

The reference parses successful POST bodies as JSON regardless of advertised
media. Both the JSON and wildcard media contracts therefore require the same
known payload shape. Reminders boolean query controls serialize as lowercase
text, default to true, and permit the source's base-parameter override to false.
Opaque provider failure bodies remain available for SDK classification. Generated
request builders and models establish wire infrastructure; MIME-selected generated
response convenience parsers do not replace the reference's unconditional JSON
parsing, error classification or public service orchestration.

## Evidence and checks

All 78 existing implementation-derived synthetic Reminders scenarios contain
86 paired exchanges: 83 POST pairs and three membership-asset downloads. Contract
checks bind every pair to an operation and validate required query values,
requests and replies. Every POST request and every successful valid POST reply
round-trips through its canonical generated model without changing JSON values.
The source-invalid zone reply is required to fail schema validation. Ten sanitized
examples cover each POST request/reply responsibility. These are synthetic
reference examples, not captured Apple writes (SCHEMA-08, SCHEMA-12).

Asset binding checks that the HTTPS origin, escaped path and decoded query occur
in a prior validated POST List record's ReminderIDsAsset/ASSETID downloadURL. It
rejects changed origins, paths, queries, orphan requests and wrong methods. This
is limited URL-issuer evidence. It does not prove the source's inline-membership
or downloadedData precedence, current-account runtime provenance, complete asset
selection semantics or SDK behavior. The existing strict Source replay continues
to bind actual selection, complete wire bytes, order and consumption; the full
Go service port and runtime/source gate remain required (LIB-05, LIB-18).

Permanent controls cover required record identity, zone-name types, modify enums,
normal-record union membership, unknown/null/large values, schema examples,
generation and protocol-constant drift, and unissued assets. No fixture matching
rule was relaxed and no live operation was performed.

## Remaining work

This milestone is the wire layer, not Go SDK semantic parity. The public Reminders
API must still port pagination, sync-token fallback, list membership precedence,
record/domain mapping, CRDT/protobuf text, linked record writes, receipt/state
updates, typed errors and all portable functional outcomes. Native authentication,
Photos, CLI/publication/release, complete source/schema/runtime/socket gates and
both final independent full audits remain open. Schema checks do not increase
handwritten SDK replay coverage or establish full migration acceptance (LIB-07).
