# Drive wire contracts

Contributor contract history: measured counts and remaining-work statements below
refer to their individual milestones. Current acceptance is maintained in
[completion matrix](completion-matrix.md); customer usage belongs in MDX guides.


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
`pyicloud/services/drive.py`. All 79 portable Drive scenarios and 139 paired
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

The measured Drive module enters 42/45 functions and covers 159/176 body
statements and 52/66 branch exits. Unentered functions are local cached-child
removal and display helpers (`remove`, `__str__`, `__repr__`), rather than new
wire endpoints. The conservative diagnostic denominator retains them. Combined
reference coverage enters 576/905 functions and covers 3,195/5,385 statements
(59.33%) and 973/2,076 branch exits. These figures do not establish functional
completeness or Go SDK parity. Reports stay private because their HTML embeds
source and local paths; no captures or account values are published (LIB-07).

Node metadata remains optional because status-only replies and incomplete nodes
are supported. Unknown node kinds and statuses stay open. Sizes accept nonnegative
integers or decimal text because the reference converts either representation.
Unknown metadata retains raw JSON, including null and large integers. Named error
fields preserve omitted versus explicit null. Negative controls cover required
fields, types, bounds, temporary folder IDs and fixed request values (SCHEMA-11).
Generated raw timestamps retain RFC 3339 offsets; their standard UTC conversion
turns `03:04:05-07:30` into `10:34:05Z`. Computed entry properties preserve the
pinned Source arithmetic instead, including its negative-minute offset bug
(`09:34:05Z` in that example). Full node replays bind that distinction; callers
can use the raw timestamp for standard UTC conversion (SCHEMA-09, LIB-05).

## Historical implementation milestones

The following receipts retain their milestone-specific status and totals; they
do not enumerate current implementation gaps. See the
[completion matrix](completion-matrix.md) for the current work list.

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
consumer build exercises all seventeen current client methods and the session/entry methods.

`DownloadDriveFile` executes thirteen reference scenarios through the
generated document-token builder and an issuer-bound content adapter. It prefers
data-token URLs, falls back to package URLs, preserves complete escaped paths
and ordered provider queries, and owns both response bodies on success and
failure. A binary file remains binary when its media label says JSON, while
reference-style JSON provider refusals still produce a typed error. Content
responses retain their actual 2xx status, including empty 204 and partial 206
responses; the content contract and focused controls cover both. Successful
results expose metadata for both stages; a second-stage failure retains copied
preceding metadata in `ClientError.PriorResponses()` (API-14, LIB-05).

The content request uses caller-supplied authentication headers and an
operation-local jar seeded from structured cookies. Token-response cookie updates
apply before the content request; explicit Cookie headers retain precedence.
CookieScopeURL metadata preserves the issuer origin/path for caller-owned
persistence, and HostOnly preserves exact-host seed binding. Complete reference
authentication persistence remains open. DriveSession now retains its own
account's cookie and token updates. No account state is retained on the
shared client (API-03, API-13). Focused controls cover token preference, missing
and malformed token replies, escaped zones and paths, repeated provider query
keys, binary bytes labeled JSON, cleanup at both stages, and preceding metadata
copy ownership. The independent consumer compiles all seventeen current client methods and the session/entry methods.

All 79 portable Drive scenarios now execute through the public SDK: nine reads,
sixteen mutations, thirteen downloads, nine service uploads and thirty-two node
flows. The complete source-to-schema runtime route gate remains open.
The non-generated library coverage includes account reads, Drive service methods
and the explicit session and entry lifecycle;
schema validation alone is not SDK replay coverage (LIB-07).
Replay measures 1219/1457 handwritten SDK/internal statements (83.7%); unit
measures 780/1457 (53.5%); combined measures 1293/1457 (88.7%). Live Go integration
is still pending. `make lint` and `make check` pass. The portable reference
inventory remains 456 HTTP scenarios/1030 pairs and 67 Python test methods.

The transfer issuer checks currently cover observed fixture URL forms. Exact
provider query binding, complete common-parameter/header validation, provider
failure media variants and a full source field inventory remain part of final
schema acceptance. The generic generated content client cannot alone prove
multi-segment path preservation or binary responses labeled JSON. The download
SDK adapters now verify download, service-upload and node/session behavior;
the final runtime route/schema gate remains open.

## Public service upload behavior

`UploadDriveFile` executes nine portable `send_file` scenarios, including empty
content, a nonzero cursor, a custom zone and provider refusal at each stage.
The added Source scenario proves successful 201 preparation/transfer and 202
registration; those statuses are owned by explicit 2XX schema responses.
The document-service requests use generated builders and schema-owned field
ordering. Multipart transfer binds exactly to the preceding destination URL,
preserves provider query values and omits document-service parameters.

The caller owns the seekable reader. Sizing restores its starting cursor; the
multipart transfer consumes from that cursor. No phase closes the reader or
retries an uncertain write. MIME inference uses the filename; multipart filename
and registration use the host platform's basename, including Windows paths.
Default timestamps use the injectable client clock at registration.

Preparation validates required destination fields before transferring. Transfer
validates required single-file/checksum/key/size fields before registering.
Fourteen strict malformed-provider controls forbid another write and retain the
actual offending reply as typed error evidence (LIB-05, API-14, SCHEMA-04).
Failure replays bind recorded file cursors and the extracted token in Source
parameter state. The token is returned to caller-owned state on success and via
`ClientError.UploadToken()` on a failed stage. Receipt/registration unknown
metadata and every completed stage's response headers remain available.
These are synthetic offline controls; no live writes have been exercised.

Node upload derives the document ID and zone from its bound entry. Successful
and failed uploads retain the extracted token in the session; a strict upload
then refresh replay proves the next request sends the updated token and cookies.
The independent consumer compiles all seventeen client methods and the public
session/entry methods. Schema validation and matcher-only checks do not add SDK
semantic coverage.

## Explicit Drive session behavior

`OpenDriveSession` copies credentials without I/O and binds a cancellable lifetime
to one account. Root/trash replacement and forced child refresh preserve the
pinned cache semantics, including old root pointers and metadata merge behavior.
Copied authentication, response and node snapshots cannot mutate session state.
Operations serialize without holding a state lock during network or reader I/O.
The contained context exception represents this explicit lifetime (API-04, GO-09).
Close cancels current and queued operations; the shared transport and caller-owned
readers remain open. Two concurrent tenants execute complete uploads and refreshes
through one shared client with isolated cookies, tokens and node state (API-13).

Five additional Source-derived cases cover file navigation guards, recovery and
permanent deletion outside trash, and upload followed by refresh. Source and SDK
both execute them offline. Folder creation declares its UUIDv4 entropy while
keeping every fixed field and complete body shape bound. Focused controls verify
cookie rotation/deletion, host case normalization, native cookie attributes,
foreign-domain rejection and explicit-header precedence. A local seek failure
preserves prior network evidence rather than inventing an HTTP response (LIB-05).
