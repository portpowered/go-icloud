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
The future SDK adapter must preserve complete escaped paths and provider queries;
the current fixtures have no provider query on their transfer URLs.

Canonical models live in `api/external/drive-models.openapi.yaml` and generate
`pkg/dependencymodels/drive`. The internal route and content clients import those
definitions. `make generate-api` regenerates all three artifacts; contract tests
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

This is a wire-contract milestone. It does not yet execute the Drive scenarios
through the generated clients or public SDK. Public projections, node navigation
and refresh state, transfer behavior, typed error parity and the complete
source-to-schema runtime route gate remain open. Existing SDK coverage percentages
still describe the five account methods; schema validation does not count as
Drive SDK replay coverage (LIB-07).

The transfer issuer checks currently cover observed fixture URL forms. Exact
provider query binding, complete common-parameter/header validation, provider
failure media variants and a full source field inventory remain part of final
schema acceptance. The generic generated content client cannot alone prove
multi-segment path preservation or binary responses labeled JSON; the SDK needs
explicit adapters and paired replay before those behaviors can be accepted.
