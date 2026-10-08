# Portable HTTP service scenarios

These JSON scenarios are synthetic and derived from the pinned reference
implementation. They describe expected implementation behavior, not observations
of Apple's service. No live writes are performed to obtain them.

Each scenario contains its source pin, service instantiation inputs, ordinary
HTTP session state, operation inputs, ordered paired exchanges, and the expected
semantic result or typed error including its message. Request bodies and response
bodies use the same base64 entity format as the private recorder. All identities,
origins, resource names, and account values are invented. There are no cookies or
credentials in these scenarios.

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
