# Drive wire contracts

The pinned reference's Drive service uses eleven fixed or zone-parameterized
operations for node details, app libraries, folder creation, rename, move, trash,
recovery, deletion, download tokens, upload destinations and document registration.
`api/external/drive.openapi.yaml` describes those routes. Authentication supplies
the Drive and document origins; this schema does not invent fixed provider hosts.

`api/external/drive-content.openapi.yaml` describes two additional content-transfer
operations. Their paths and origins come from a preceding token or destination
response. The path template is a contract for provider-issued URLs, not permission
to forward to arbitrary hosts. The contract tests reject transfers with no issuer,
changed origins or paths, the wrong method, and an unrelated preceding route.
They prefer `data_token.url` over `package_token.url`, as the reference does.
The download SDK adapter preserves complete escaped paths and provider queries.
Its focused controls cover repeated provider keys and escaped path segments;
the original portable fixtures have no provider query on their transfer URLs.

Canonical models live in `api/external/drive-models.openapi.yaml` and generate
`pkg/dependencymodels/drive`. The internal route and content clients import those
definitions. `make generate-api` regenerates all three artifacts and schema-owned protocol constants; contract tests
regenerate from the checked-in configs and reject drift (SCHEMA-10, SCHEMA-16).

## Evidence and verification

The source is timlaing/pyicloud commit
`e2e44ab875d47dab4475096021da60030f26c35e`, especially
`pyicloud/services/drive.py`. All 67 portable Drive scenarios and 115 paired
exchanges are labeled synthetic and implementation-derived. No new live writes,
private captures, tokens or account values are published. The four model examples
are invented (SCHEMA-08, SCHEMA-12).

The Go contract suite validates all request and response payloads against their
operation, status and media contracts. Multipart field shape is checked here;
the existing paired multipart tests verify exact bytes, ordered headers and
framing. Raw download bytes, including empty content, remain uninterpreted.
Upload preparation and document registration use the reference's `plain/text`
media label with a JSON entity. Upload registration retains the upload token in
query parameters, while the multipart transfer does not append those parameters.

Rename, movement and recovery have distinct request models and required fields.
Permanent trash deletion omits the `clientId` used by ordinary deletion. Empty
movement lists are legal. Source-required transfer receipt fields are required;
the receipt string is optional, including for zero-length uploads.

Eight node-upload/facade scenarios add 26 pairs and execute the actual
`DriveNode.upload` and `DriveService.__getattr__` function bodies. Node uploads
derive their document ID and zone from the preceding folder reply. They bind
the input cursor, binary/empty transfers, each stage's provider refusal, received
node state and final file cursor; instrumentation-owned streams close on every
exit. Cursor, zone, node/error state and extra-exchange negative controls reject.

The measured Drive module enters 42/45 functions and covers 155/176 body
statements and 48/66 branch exits. Unentered functions are local cached-child
removal and display helpers (`remove`, `__str__`, `__repr__`), rather than new
wire endpoints. The conservative diagnostic denominator retains them. Combined
reference coverage enters 576/905 functions and covers 3,191/5,385 statements
(59.26%) and 969/2,076 branch exits. These figures do not establish functional
completeness or Go SDK parity. Reports stay private because their HTML embeds
source and local paths; no captures or account values are published (LIB-07).

Node metadata remains optional because status-only replies and incomplete nodes
are supported. Unknown node kinds and statuses stay open. Sizes accept nonnegative
integers or decimal text because the reference converts either representation.
Unknown metadata retains raw JSON, including null and large integers. Named error
fields preserve omitted versus explicit null. Negative controls cover required
fields, types, bounds, temporary folder IDs and fixed request values (SCHEMA-11).
Generated timestamps retain RFC 3339 offsets; a regression verifies that
`03:04:05-07:30` becomes `10:34:05Z`, correcting the reference's negative-minute
offset bug when the SDK later exposes UTC projections.

## Remaining work

The public SDK now exposes `GetDriveNode` and `ListDriveLibraries`. Nine existing
portable scenarios execute the public methods and compare the full semantic
result, while matching every paired request and consuming every exchange. They
cover empty, one and many records, shared selectors and provider refusals.
Independent public projections preserve recursive children and unknown JSON.
Provider-shape controls retain exact failure bytes and reject missing records;
string encoding and empty-share omission controls bind the pinned reference's
wire behavior. Credentials remain per-request on the shared web transport
(API-02, API-03, API-13).

The document-registration operation is named `DriveRegisterDocument`, leaving
`DriveUpdateDocument` as its request-model name. This prevents ambiguous generated
operation-path and model-property constants; generation rejects collisions and
checks both account and Drive artifacts for drift (SCHEMA-10).

Seven public mutation methods now execute sixteen existing portable scenarios
through generated request builders and typed wire models: folder creation,
rename, movement, trash, recovery, ordinary deletion and permanent deletion.
Schema `x-order` declarations preserve the pinned request field order. Bulk
movement preserves order and accepts an empty list; each deletion variant
retains its own client-identifier behavior. Creation overrides the caller's
Content-Type for that request, as the reference does, without mutating caller
headers. No live writes are performed.

The generator derives operation/media constants from the route document and
field constants from the canonical model document. A model-owned
`x-protocol-prefix` must match the complete anchored literal prefix and a valid
UUIDv4 instance. Truncated, mistyped and unanchored declarations fail; a changed
valid prefix follows the schema. UUIDs use fresh cryptographic entropy. These
checks enforce generated ownership without handwritten wire models (SCHEMA-10,
SCHEMA-15). The Client-only interfacebloat exception keeps selected operations
on one traced interface (API-01); all other lint rules remain enabled.

Unit checks cover read/close failure suppression across all seven mutations,
invalid provider acknowledgements, missing versus empty lists, unknown JSON
values and the explicit creation header override. Two-account concurrent
paired rename tests repeat requests after separate Set-Cookie replies, retaining
request identity, cookie and result isolation (API-03, API-13). The public
consumer build exercises all fourteen current methods.

`DownloadDriveFile` executes seven existing reference scenarios through the
generated document-token builder and an issuer-bound content adapter. It prefers
data-token URLs, falls back to package URLs, preserves complete escaped paths
and ordered provider queries, and owns both response bodies on success and
failure. A binary file remains binary when its media label says JSON, while
reference-style JSON provider refusals still produce a typed error. Content
responses retain their actual 2xx status, including empty 204 and partial 206
responses; the content contract and focused controls cover both. Successful
results expose metadata for both stages; a second-stage failure retains copied
preceding metadata in `ClientError.PriorResponses()` (API-14, LIB-05).

The content request uses caller-supplied authentication headers. Token-response
cookie updates are exposed afterward rather than applied through an intermediate
cookie jar. This preserves the explicit-header boundary; complete reference
session cookie-jar parity remains open. No account state is retained on the
shared client (API-03, API-13). Focused controls cover token preference, missing
and malformed token replies, escaped zones and paths, repeated provider query
keys, binary bytes labeled JSON, cleanup at both stages, and preceding metadata
copy ownership. The independent consumer compiles all fifteen current methods.

Node navigation and refresh state, uploads and the complete
source-to-schema runtime route gate remain open. Thirty-two read/mutation/download scenarios do not
establish parity for all 67 Drive scenarios. The non-generated library coverage
measurement includes the five account reads and ten Drive methods;
schema validation alone is not SDK replay coverage (LIB-07).
Replay measures 552/666 handwritten SDK/internal statements (82.9%); unit
measures 578/666 (86.8%); combined measures 617/666 (92.6%). Live Go integration
is still pending. `make lint` and `make check` pass. The portable reference
inventory remains 437 HTTP scenarios/991 pairs and 67 Python test methods.

The transfer issuer checks currently cover observed fixture URL forms. Exact
provider query binding, complete common-parameter/header validation, provider
failure media variants and a full source field inventory remain part of final
schema acceptance. The generic generated content client cannot alone prove
multi-segment path preservation or binary responses labeled JSON. The download
SDK adapter now verifies those behaviors; the upload adapter remains open.
