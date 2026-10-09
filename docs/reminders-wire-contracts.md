# Reminders wire contracts

The Go `ListReminders` implementation passes all thirty-nine
portable compound query cases. It sends the literal list identifier, completion
filter, optional page size and continuation marker; consumes all pages; and
projects complete reminders and supported related records. Per-record failures
reject the page before projection, with exact HTTP evidence and prior responses.
Normal records outside the list are still decoded before final relationship
filtering, matching the pinned Source's behavior.

The initial eight compound reference cases entered every statement and branch exit
in `_ingest_compound_record` (26/26 statements, 16/16 exits). The synthetic-only
diagnostic keeps the unchanged overall denominators: 585/905 functions entered,
3,347/5,398 function-body statements (62.00%) and 1,051/2,076 branch exits covered
after all thirty-one compound cases, twenty-eight snapshot facade cases and
thirteen overlapping list-record union cases.
Further Source cases now exercise asset URLs, integer-coercible size metadata,
byte-backed text, invalid UTF-8 replacement and raw frequency selection. Six more
cases bind wrapper-sensitive discriminator and URL behavior: bytes are compared
before model coercion and byte-backed URLs avoid a second decoding step.
Two strict-decoding cases preserve STRING base64 URLs containing CR or LF.
Twenty-eight Source snapshot facade cases now cover public list discovery, explicit
and empty filters, pagination, replacement across lists, overlapping record/error/
tombstone alternatives and discovery-cookie reuse. The Go `ListReminderSnapshot`
port executes all twenty-eight scenarios and forty-six paired exchanges with
complete reminders, ordered response evidence and no partial result on failure.
Full verification and independent review remain pending.
Thirteen additional list discovery cases execute both Source and Go. Selection follows
the Source model union before errors are checked: normal records and tombstones
can win over error-shaped extra metadata, while a selected error with an empty
code still fails. The transport validates all records in every returned zone
before provider-error checks or list projection. Three paired Source/Go controls
place malformed records after errors or projection-sensitive records, within one
zone and across zones. All selected errors are rejected before list projection.
Remaining endpoint ports, live Go verification and
full migration/release acceptance remain open.

The selected Reminders service uses five CloudKit POST operations: record lookup,
query, modify, zone changes and zone discovery. It also downloads provider-issued
membership assets. `api/external/reminders.openapi.yaml` describes these six
operations and imports shared wire models from `cloudkit-models.openapi.yaml`.
Dormant database-change helpers are excluded from this active Reminders contract.

## Canonical models and generation

The model responsibility is shared CloudKit protocol data, separate from public
Reminders domain projections. Its named schemas include source record/query/
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

The current 302 implementation-derived synthetic Reminders scenarios contain
363 paired exchanges: 360 POST pairs and three membership-asset downloads. Contract
checks bind every pair to an operation and validate required query values,
requests and replies. Every POST request and every successful valid POST reply
round-trips through its canonical generated model without changing JSON values.
Forty source-invalid zone/list/lookup/query replies must fail JSON or schema validation. Ten sanitized
examples cover each POST request/reply responsibility. These are synthetic
reference examples, not captured Apple writes (SCHEMA-08, SCHEMA-12).

Fifty-three Source-executed sync-cursor scenarios bind query tokens, null/empty token
fallback, invalid JSON and invalid query records, paginated zone changes,
last-zone token selection, missing tokens and provider failures. The shared
session raises HTTP 401, 403, 429 and 503 before CloudKit fallback; those cases
consume only the query exchange. The two fallback decode failures do consume
zone changes. Additional cases exercise valid and invalid values for all fifteen
known CloudKit field tags, future tags, encryption coercion, downloaded bytes and
reference zone identity. Known malformed fields cannot escape through the future-tag
schema branch; accepted Source scalar coercions remain explicit in canonical input
schemas and runtime validation. These distinguish actual service behavior from the lower-level
CloudKit client's error handling. Negative controls reject changed final tokens,
changed public results and unused pages. Public Go `GetReminderSyncCursor`
executes these same 53 scenarios and 85 paired exchanges. It returns the exact usable token and ordered
response metadata, preserving prior exchanges when a later page fails. Decode
failures in the query permit fallback; HTTP/session failures stop immediately.

72 paired Source/Go change-iteration scenarios contain 75 paired exchanges and
bind no/null/empty cursors, ordered duplicates, unrelated records, complete deleted
reminders and tombstones, last-zone cursor selection, later-page failures and
schema rejection. Source API errors bind their structured payloads as well as
their type and message, including null payloads. Negative controls reject changed
events, event order, requests, unused pages and error payloads. 47 cases bind
Source's selection among overlapping record, tombstone and error alternatives,
including ties, nested metadata, boolean coercion and encrypted-field validation.
The Go adapter scores supplied declared model fields and strict/coerced matches
before selecting a valid alternative. The SDK preserves event order, duplicate
events, complete reminder projections, null tombstones and response metadata.
Nested required identities and non-nullable optional dictionaries are checked before union
selection. Participant bool/int coercions apply to owners, current users,
participants, requesters and blocked members; arbitrary integer magnitudes are
preserved. Audit numeric JSON floats round before truncation, while numeric text
retains integer parsing. Public dates reproduce float-seconds conversion to
microseconds, including the pinned Windows runtime's UTC range from Unix second
-43,200 through 32,536,850,399. Boundary controls execute the actual Source mapper.
These runtime limits are reference parity evidence, not Apple date restrictions.
The SDK milestone merged through PR #40 after full checks, successful CI and
independent exact-commit approval. This synthetic evidence does not establish
live Go behavior.

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

The internal [text protocol adapter](reminders-text-protocol.md) now decodes
versioned CRDT title and notes bytes against a separate portable Source corpus.
The public `GetReminder` mapper decodes title/notes, normalizes invalid UTF-8
subsequences using the pinned Unicode decoder, maps all 21 domain fields, and
uses audit dates after absent or normalized-unset creation/modification fields.
Nineteen complete semantic replays bind default/full results, ordered related
IDs, sentinel and fractional dates, document fallbacks, unrelated records,
alternate success status/cookies, missing records, provider errors and malformed
replies. `ListReminders` reuses that complete reminder projection while consuming
every compound query page and mapping related records. The all-lists snapshot
facade is implemented; linked-record lookup methods and write encoding remain pending.

The public Go `ListReminderZones` operation now passes ten semantic replays,
including empty, one and multiple zones, provider and schema failures, and
201/202/299 success. HTTP202 JSON is decoded despite a plain-text content type.
An additional case binds unknown metadata at response, zone and identity levels,
including large JSON integers and nulls. It returns caller-owned zone projections and metadata; authentication returns
the discovered reminders origin. See the [README example](../README.md).
The build-parameter scenario binds the optional authentication build and
mastering query values in Source order; the external schema owns these optional
parameters for every Reminders POST operation.

The public `ListReminderLists` reader now binds thirty-seven Source scenarios
to complete domain results and ordered response metadata. It consumes list
pagination and inline or downloaded membership; record errors fail the snapshot.
Twelve added scenarios execute against the pinned reference under the strict
offline adapter. They bind missing required response fields, nullable-array
rejection, numeric/container truthiness and display conversion, embedded asset
precedence, invalid membership, and empty pagination tokens. An additional nested
display case binds control-character escaping to the reference. Five malformed
responses must fail external schema validation as well as Source and Go decoding.
The remaining public Reminders API must still port
linked-record lookup methods, reminder and related-record mutations, CRDT write
encoding, write receipts/state updates, and their typed failures and portable
functional outcomes. Implemented query pagination and domain/text decoding are
covered by the preceding Source/Go cases. Native authentication,
Photos, CLI/publication/release, complete source/schema/runtime/socket gates and
both final independent full audits remain open. Wire schema checks alone do not increase
handwritten SDK replay coverage or establish full migration acceptance (LIB-07).

## Related lookup projections

`ListReminderTags`, `ListReminderAttachments`, `ListReminderRecurrenceRules`, and
`ListReminderAlarms` reuse the schema-owned lookup route with ordered record
names. Forty-six synthetic scenarios, executed through the pinned Python
reference, contain fifty-one paired exchanges. Each Go replay binds complete
ordered public results, response metadata, structured failure bodies, prior
responses and final exchange consumption (LIB-05/LIB-12).

Empty input skips HTTP, while empty replies return allocated empty lists.
Unrelated records and tombstones are skipped; unsupported attachments and
triggers are skipped after full wire validation. Provider errors are detected
before projection, but complete response validation precedes provider errors.
Alarm trigger IDs preserve duplicates; later duplicate triggers replace earlier
ones without collapsing the ordered alarm list. Missing triggers are null.
This is offline reference evidence, not a claim of successful live operations.
