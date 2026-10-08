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
must be consumed. Semantic and wire negative controls are part of offline checks.
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
