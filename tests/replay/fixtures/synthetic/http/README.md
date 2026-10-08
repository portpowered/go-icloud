# Portable HTTP service scenarios

These JSON scenarios are synthetic and derived from the pinned reference
implementation. They describe expected implementation behavior, not observations
of Apple's service. No live writes are performed to obtain them.

Each scenario contains its source pin, service instantiation inputs, ordinary
HTTP session state, operation inputs, ordered paired exchanges, and the expected
semantic result or typed error including its message. Request bodies and response
bodies use the same base64 entity format as the private recorder. All identities,
origins, resource names, account values, cookies, and token parameters are
invented. There are no real credentials or private account cookies in these
scenarios.

The reference runner creates the actual service and session, injects strict
paired replay below request preparation, and forbids HTTP and socket fallback.
Every request must match before its response is returned, and every exchange
must be consumed. Rejected traffic remains a failure even when reference code
catches the immediate exception. Semantic and wire negative controls are part of offline checks.
The JSON does not contain Python object snapshots; the Go runner must consume
these same artifacts when its equivalent operations are implemented.

Current cases cover Drive folder/app retrieval with zero, one, and many results;
shared-folder selectors; rename, delete, move, trash, restore, and permanent
deletion request shapes; missing download tokens; account devices and family
lists with zero, one, and many results; and storage projections. Other operations
and error variants remain open in the migration checklist.

Find My scenarios cover service initialization, zero/one/many devices, sound,
messaging, lost mode, erase-token exchange and erase request shapes, unavailable
capabilities, and missing erase tokens. Its monitor must stop before session
teardown, and offline checks assert no monitor thread survives the suite.
Reminders scenarios cover zero/one/many lists, sync-token pagination, absent
zones, per-record errors, missing reminder lookups, query/fallback sync tokens,
and incremental empty results. These are synthetic cases even when their
operations overlap captured reads.

The Reminders endpoint extension covers create/update/soft-delete, record
conflicts and missing records, completed/uncompleted queries with zero/one/three
records and pagination, hydrated creation, lookups, and incremental changes.
Linked-record cases cover tags, URL/image attachments, recurrence rules and
location alarms, including caller model change tags and membership after
accepted writes and provider rejection. Related lookups cover zero/one/three
items, chained alarm/trigger lookup, and both inline and asset-backed list
membership. Asset-backed cases match the follow-up download request separately.

Portable input descriptors use `{"$model":"Reminder","value":{...}}` (or the
named related domain model) and `{"$datetime":"ISO-8601 value"}`. These are
caller values, not Python object snapshots. `observe_arguments` selects caller
models for post-operation projection; success includes them in `result`, while
`error_arguments` records them after a rejected write. This verifies what the
caller can observe, rather than only accepting a null mutation return.

Mutation cases declare an `entropy` object containing a finite Unix clock value
and an ordered sequence of canonical UUID-v4 samples. The reference's actual
write and protobuf/CRDT code runs with those inputs; no provider method is
replaced. Requests, repeated UUID references, timestamps, resolution-token maps,
and decoded caller state remain exact. Unexpected or unconsumed UUID generation
fails, including when a provider error occurs, and the clock/UUID factories are
restored at teardown. These remain invented reference examples, not live writes.

Drive transfers include zero-byte and binary data/package downloads, multipart
uploads with and without a receipt, document registration, and folder creation.
Multipart rules validate the boundary format and its agreement with the encoded
body; part order, complete part headers, filename, field count, file bytes, and
remaining request metadata must match. Folder creation permits only an explicit
UUID-v4 client-ID pattern; every other JSON field remains exact.

Photos cases cover service instantiation with ready/pending/missing indexing,
cached/discovered/missing sync tokens, private/shared zone discovery, incremental
zero/one/many updates, deletions, and unavailable shared streams. They do not
yet cover the full album/asset/upload/shared-stream function matrix.
Photo change timestamps also have an explicit millisecond-to-UTC semantic case.

The Photos endpoint extension covers album discovery with zero/one/three custom
albums, nested folders and pagination; asset counts and queries with zero/one/
three records and a 50-record page followed by one record; existing/missing
lookups; and empty, binary, unavailable, alternative, medium and Live Photo video
downloads. Offline mutations cover album/folder creation, rename, deletion and
membership, plus asset deletion and favorite updates with refresh and conflict.
Asset projections bind metadata, resource versions and returned record fields.

Creation declares ordered base64 `random_bytes` samples and a
`photos_position_ms` clock value consistent with `unix_seconds`. Injection replaces
the reference's byte entropy source and its clock-only album-position helper;
the actual creation method still prepares the record and performs the request.
The sample sizes and complete consumption are checked, and factories restored.

The rejected asset-deletion case records a reference defect: a per-record error
still returns true. It is implementation evidence, not an observed successful
Apple write. The Go implementation must report the rejection, with that deliberate
departure explicitly tested rather than silently copied. Complete shared-stream
error coverage remains open.

Upload endpoint cases now cover reservation, raw file transfer, registration and
progress lookup with zero/one/three results, unknown jobs, binary/empty bytes,
HTTP errors, invalid JSON and unexpected provider payloads. The actual uploader
pipeline covers successful registration, existing-asset duplicates, provider
rejection, missing reservation and zero/multiple registration results. Temporary
file inputs declare basename, base64 entity bytes and modification seconds.
Finite UUID samples also bind the uploader's imported UUID factory; the explicit
`photos_local_timezone` environment sample binds zone and offset through its
clock/environment-only helper. Actual request and upload workflow methods run.
CloudKit failures require `error_payload` as well as type/message. The field
preserves provider JSON or invalid-response text exposed through the exception;
missing or changed payloads fail replay.

The HTTP recorder snapshots seekable binary file bodies from their current
position, restoring the position before forwarding live traffic. Replay validates
the complete entity and Content-Length. This is HTTP entity matching at the
adapter seam; it does not record TCP chunk framing. Streamed responses retain
read/close behavior and duplicate response headers for cookie processing.

Shared-stream cases use the actual modern service's legacy stream dependency.
They cover zero/one/three albums, asset counts and asset pages; multi-page reads;
existing/missing lookups; metadata and likes; and binary/empty downloads. Their
origin, DSID and album-provided content location remain exact request inputs.
These synthetic cases do not establish observed Apple stream behavior. Remaining
endpoint error variants still need cases.

The service-upload extension calls `PhotosService.upload` through the configured
upload client and then actual CloudKit record lookup. Cases cover immediate and
duplicate hydration, delayed indexing, bounded backoff, transient/persistent
invalid JSON, incomplete/not-found records, timeouts, absent registration IDs,
normalized registration rejection, target-album membership and missing albums.
Timeouts return null after registration; they do not imply the file was absent
from provider storage. HTTP 429 currently raises the session's public error before
the CloudKit hydration retry handler sees it; this reference behavior is explicit.

`photos_wait_trace` is an ordered list of `{ "kind": "monotonic" | "sleep",
"value": seconds }` environment events. Monotonic samples are finite,
nonnegative and nondecreasing; sleep durations must match exactly. No real wait
is used. Unexpected calls, wrong order or unconsumed samples fail even on errors,
and both time factories restore at teardown. This verifies retry decisions at
the clock boundary, not live scheduling.

Account endpoint additions cover family-photo requests with zero/one/three
members, empty/binary streamed photo bodies and provider rejection. The result
binds member ID, response status, headers and bytes; the response is closed after
reading. Subscription summary cases call the actual property against its global
and China gateway constants, binding empty results, invented plan projections
and provider errors. These payloads describe reference behavior rather than an
observed complete subscription schema.
