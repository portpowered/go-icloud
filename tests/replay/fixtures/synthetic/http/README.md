# Portable HTTP service scenarios

These JSON scenarios are synthetic and derived from the pinned reference
implementation. They describe expected implementation behavior, not observations
of Apple's service. No live writes are performed to obtain them.

Each scenario contains its source pin, service instantiation inputs, ordinary
HTTP session state, operation inputs, ordered paired exchanges, and the expected
semantic result or typed error including its message. Request bodies and response
bodies use the same base64 entity format as the private recorder. Account
identities, resource names, account values, cookies and token parameters are
invented. Some authentication origins are pinned Apple constants; real network
access remains forbidden. There are no real credentials or private account
cookies in these scenarios.

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

Drive node scenarios additionally cover cached zero/one/three-child listings,
forced and service-level refresh, shared-folder navigation, named lookup,
provider missing-folder responses, trash navigation and node-backed writes.
Downloads bind binary content and the empty-file shortcut's absence of traffic.
Uploads bind multipart bytes, the caller's current file position, custom zones,
timestamps and session token parameters. Provider refusals cover each transfer
stage and the selected single-request endpoints; no later exchange may occur
after a refusal. `observe_transfer` projects the return, parameters and final
file position. Errors bind `error_arguments`, `error_drive_state`, optional
`error_node_state`, and the exposed HTTP `error_context`.

Node upload cases invoke the pinned `DriveNode.upload` after retrieving the
folder. They bind the node's document ID and zone, binary/empty/current-cursor
content, and provider refusal at preparation, transfer and registration. The
portable result exposes final cursor and request parameters; failures also bind
received node/root metadata. Instrumentation-owned file objects close after
success and failure. A service-directory case exercises the public service
facade's delegation to the root node. These eight cases are synthetic, with
26 paired exchanges; they are reference behavior, not live writes or Go SDK parity.

These examples preserve a pinned-reference date defect: a negative offset with
nonzero minutes is decoded incorrectly: `2024-01-02T03:04:05-07:30` yields
`09:34:05` rather than the correct UTC `10:34:05`.
The Go client should parse RFC 3339 offsets correctly and document this deliberate
departure rather than reproduce the defect. No fixture contains live file data.

Find My scenarios cover service initialization, zero/one/many devices, sound,
messaging, lost mode, erase-token exchange and erase request shapes, unavailable
capabilities, and missing erase tokens. Its monitor must stop before session
teardown, and offline checks assert no monitor thread survives the suite.
Reminders scenarios cover zero/one/many lists, sync-token pagination, absent
zones, per-record errors, missing reminder lookups, query/fallback sync tokens,
and incremental empty results. These are synthetic cases even when their
operations overlap captured reads.

Find My refresh cases call the actual manager's public refresh method, binding
initialization versus refresh routes, locate flags, server-context forwarding,
device updates and absent/empty/multiple response content. Family cases bind
ready members, stalled progress, improving progress, retry bounds and provider
refusal. `findmy_wait_trace` uses the same finite ordered wait contract described
below; the real polling decisions run while sleep durations are consumed offline.
Device command and erase cases include provider refusals at token and command
boundaries. Errors bind exposed response context and, after an explicit refresh,
the received cache in `error_findmy_state`.
The historical `token_origin` input is a configured endpoint base, including
`/setup/ws/1`, as supplied by the actual account facade. Older examples omitted
that prefix; all erase-token expectations now use the source-matched full path.

An empty or absent refresh payload retains previously known devices in the
pinned implementation. Cache snapshots read received content without the
device.data property's implicit refresh after a failed request. Thread scheduling
state is not a portable result; the actual monitor is stopped and joined at
teardown, and the suite checks that no new thread survives. Synthetic command
acknowledgments do not establish physical action on an Apple device.

The reference refresh can also skip monitor replacement while the old monitor
is still exiting, leaving polling stopped after a successful refresh. An
independent scheduling probe reproduced this race. The Go implementation must
join a stopped owned monitor before starting its replacement, or expose refresh
without implicit polling. Cached devices are retained for partial refresh
payloads; a Go removal policy must use explicit provider deletion evidence.

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

Authentication endpoint cases instantiate the actual pinned account service with
declared account/session/challenge state. They cover cached and refreshed token
login, paused/untrusted states, session probes, trusted-device lists, verification
delivery/validation, SMS, session trust, terms/repair, one-factor service login,
PCS consent/cookie polling and logout controls/failures. Zero/one/three device
results and token/cookie rotations are explicit. A required MFA login without
fresh credentials records the reference's failed-login result; it does not
pretend to complete SRP authentication.

Success projects both the operation value and resulting account/session/challenge
state, cookies with policy metadata, resolved webservices, trust/delivery flags,
caller argument changes and persistence-file presence. Auth failures require
`error_auth_state` and `error_arguments` alongside exact type/message, including
device changes made before a rejected verification request. `error_context`
binds reason/code and the exposed HTTP response's status, headers and body.
File paths and Python object
snapshots are not part of this contract. The optional `synthetic_password` is an
invented input; outgoing password/code fields use existing explicit match rules.

PCS retries use ordered `auth_wait_trace` sleep events under the same consumed
clock contract as Photos; the two trace selectors cannot coexist. Auth and
service-upload operations reject undeclared clocks/waits even without an entropy
object. Rejected events remain failures if reference code catches the exception.
Delays are matched without actual waits.

SRP sign-in cases run the actual reference password stretching and SRP proof
calculations for both `s2k` and `s2k_fo`. Their declared `random_bytes` sample
supplies the 256-byte client secret at `os.urandom`; no SRP business method is
replaced. The pinned `srp==1.0.22` implementation sets the secret's high bit before
deriving the public value. Authoring verified client and server proofs against a
synthetic server verifier. Replay matches the public value, both proofs, challenge
response, trust-token forwarding, resulting cookies/session state and errors.
Cases cover successful account setup, authorize/init/complete provider refusal,
paused MFA token login, and auth-shell discovery followed by SMS code delivery.
All salts, secrets, passwords and challenge values are invented; these are
implementation-derived protocol examples, not captured authentication evidence.
Hardware security-key interaction is deferred. The SRP HTTP cases alone do not
prove complete bridge authentication; the combined full-flow cases below cover
the synthetic HTTP/socket workflow.

Combined bridge scenarios carry `bridge_network` with declared P-256 private
scalars, socket connections/events and a shared HTTP/socket timeline. They run
actual source key generation, signing, bootstrap, prompt setup and raw WebSocket
framing. The curve-key and bounded prover-scalar entropy factories plus TCP/TLS
factories are injected; prover algorithms are not replaced.
The variable ECDSA signature must verify against the exact declared public key
and nonce; the portable bootstrap matcher independently checks canonical
protobuf fields, field order, expiration and signature envelope. No signature
bytes or generated connection URL are treated as unconstrained wildcards.

The shared timeline binds connection/wrap/timeout, complete socket wire events,
HTTP exchanges and close. Receive chunk sizes may vary while each entire event
and the source client's internal buffer must be consumed. Rejected frames,
routing, entropy and connection attempts remain failures even if caught. Both
source cleanup and teardown close retained prompt connections inside the
declared entropy scope; factories restore on success and failure.

The initial twelve cases cover prompt setup, repeated prompt calls, step-0 statuses
200/204/409, refusal, invalid-nonce retry with server time, mismatched session
UUID, SMS fallback and legacy (`_W`) device-code success/rejection with trust
and account setup. Bridge state projects protocol fields,
verified bootstrap metadata and whether a socket remains active, without
Python object snapshots. TLS cryptography and live trusted-device interactions
are not established by these synthetic transcripts.

Fifteen modern verification cases execute actual scrypt/P-256 SPAKE2 point math,
HMAC confirmation/key derivation and AES-GCM decryption. Ordered `prover_random`
samples declare the exact `secrets.randbelow` upper bound and hexadecimal value;
zero-then-nonzero retry is exercised and every draw must be consumed. Undeclared
or rejected draws remain failures even if caught, and the factory restores.
Authoring used the pinned reference's server helper with an invented peer scalar
to verify both confirmations and encrypted plaintext before recording. Active
replay uses the actual client prover, not that helper.

Cases cover code-validation statuses 200/204/409/412, final steps 4 and 6,
rejected confirmation/ciphertext, provider error pushes, session mismatch and
refused step/code/completion endpoints. The invented decrypted code has an
explicit base64 entity expectation, binding its exact bytes rather than merely
its six-digit shape. The recorder still redacts private passwords and codes;
exact synthetic expectations do not change private capture behavior.

Three complete login cases start with empty session/account/challenge state,
run actual SRP and automatic auth-shell/bridge setup, then modern or legacy code
verification and trust/account transitions. Modern 412 retains MFA requirements
and sends neither trust nor account-login requests. All HTTP and socket events
share one consumed timeline. These establish synthetic pinned-reference login
compatibility, not current live Apple behavior, physical device interaction or
Go interoperability.

Photos container scenarios select `container_scope` as `private` or `shared`
and invoke the actual CloudKit client's `database_changes` or `lookup`. Lookup
zone inputs are portable dictionaries converted to the reference request model.
Shared-library scenarios select the actual discovered library through `library`;
they bind discovery, shared batch counts, queries, pagination, changes and writes.
Shared libraries expose a restricted smart-album set, so private custom-album
zero/one/many behavior is not copied into that operation. Reminders `zones`
invokes actual zone discovery, including zero/one/three results and provider errors.

Every expected `PhotosServiceException` requires `error_resources` with nullable
`photo` and `album` projections. Missing resources and changed resource fields
fail replay even when error type and message agree (LIB-05). These projections
contain ordinary JSON, without Python object snapshots.

The pinned shared-library favorite operation sends its mutation through the
private client with the shared zone and then refreshes through the shared client.
Both exchanges are bound, including the refresh after a rejected record. This
source quirk is synthetic reference evidence, not an observed Apple mutation.
The Go implementation must keep the asset's selected container and zone rather
than reproduce the private-client routing defect.
