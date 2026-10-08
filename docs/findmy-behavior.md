# Find My device descriptions and timed monitoring

Seven new portable description scenarios call the pinned Python device name,
model, display model, device type, status, location and capability getters. They
also exercise iteration, count, indexed and dynamic access for received extra
metadata. The Go DescribeDevice operation returns copied cached data and the
same complete functional projection through independent generated public models.

Cases cover populated metadata, missing/default metadata, enabled/disabled LOC,
missing/null/empty locations and selection among several devices. Requested
additional status values preserve nested unknown JSON, explicit null and integers
larger than the exact range of a double. The reference's four default status
fields remain present as null when missing. Local description getters perform no
I/O; Refresh remains explicit, including when the session is closed. A focused
control verifies description/cache ownership and no post-close request (API-05,
SCHEMA-07). No unrelated caller-input validation was added.

## Timed-loop artifacts

Three additional portable streams run the actual Source _monitor_thread and
actual Go owned monitor. Source wait completion and datetime sampling are
injected, and all HTTP calls use strict paired offline replay. The Source's
previous monitor is stopped and joined before clock injection. The tick callback
calls the real manager refresh and records the whole post-tick state, failure
flag and actual error status; exceptions propagate to the real monitor's catch
and continue path. No provider request, response or error handler is replaced.

The artifacts declare the interval, ordered completion timestamps and stop
outcomes. Normal completed intervals advance beyond the Source's strict wall-clock
comparison; final stop need not advance time. The Go scheduler checks durations,
completion eligibility and stop order, then releases the actual session loop.
It observes state and typed errors at the next wait boundary. Both adapters
require finite time values, a nonempty trace and exactly one terminal stop as
its last event. A stopped wait maps to owned session cancellation, and
MonitorDone must close. This binds actual
orchestration rather than selecting recorded route names (LIB-05).

Cases bind failure followed by recovery, repeated refusal with provider cookie
rotation, and empty/absent partial replies that reset continuation context.
Family LOADING metadata received during monitoring causes no family re-poll.
Monitor requests omit new-location controls. Exact response status/headers/body
and cookie scopes remain checked, even when the loop catches a provider failure.
The transport-only driver injects a caller-owned stream cookie state; the SDK
checks Authentication cookie name/value/domain/path/secure/host-only projections
at each tick boundary, including a new cookie from the final failed response.
No fixture matcher was relaxed.

Permanent negative controls reject changed wait durations, ineligible completion
times, unconsumed events, premature stops, changed snapshots/failure meaning and
altered metadata, location/capability/status results and final cookie state.
Independent review caught the initially missing mandatory Go terminal stop and
an infinite Source terminal timestamp; permanent Source/Go controls now reject
both before running any monitor replay. Source and Go use the same
implementation-derived artifacts; synthetic does not mean a captured live call.

## Evidence and limits

Current Find My inventory is 71 scenarios/140 pairs (37 session cases, seven
descriptions, three timed loops and 24 additional command reply cases); 127 JSON responses round-trip through
canonical wire models. Global HTTP inventory is 490 scenarios/1100 pairs.
Native Go auth/417 reauth, stopped-monitor
implicit getter refresh branches and wall-clock boundary/skew behavior, remaining
selected-service ports, full runtime/source/model gates, documentation publication
and release remain open. Generic bad caller inputs and unrelated services remain
outside the requested focus. No live device command was performed.

PR15 was independently approved at exact
75bc5e61a5b25ac792c5584c4993983921af8aa5 after CI37833251608 passed, merged to
2140a28aae9e89e368057f75a021094466418864 and its branch was deleted. Main
CI37833871353 also passed. These verified milestones do not establish full
migration acceptance; final checks and exact-commit review remain required for
this follow-up (GO-15).

PR16 local checks passed: make lint/check, Go race/contracts/regeneration/consumer
checks and 69 Python test methods, with the final cookie cases additionally
re-probed through focused Source and Go checks. Handwritten replay coverage is
1518/1805 statements (84.1%), unit 1085/1805 (60.1%) and combined 1608/1805 (89.1%).
These are separate diagnostic measurements (LIB-07); cancellation paths can vary
by a few statements. Live Go integration and full migration acceptance remain open.

PR16 synthetic-only Source coverage over its final artifacts is 575/905 entered
functions, 3149/5385 body statements (58.48%) and 964/2076 branch exits. Find My
enters 35/39 functions, covers 148/171 body statements and 51/68 branch exits.
The actual monitor now covers 8/8 body statements and 3/4 branch exits. Four
unentered functions are manager/device display __str__/__repr__ helpers. Remaining
selected branches include forced reauth, stopped-monitor implicit getter refresh,
integer selection and the timer boundary/skew path; generic bad caller inputs
remain outside the requested focus. Counts do not establish full acceptance.

## Additional command reply evidence

Twenty-four Source/Go cases now cover all four selected command methods with
binary 200, malformed JSON 200, JSON array 202, scalar text/json 200 and provider
error objects under both accepted JSON media types at HTTP 200. Commands return
raw acknowledgement bytes; this acknowledges the request, not physical completion.
Provider payload errors return typed Provider errors with exact HTTP 200 context.
They are sent once, including erase after its fresh token lookup. The schema
models the opaque bytes and keeps an optional arbitrary-JSON parsed projection.
Permanent Source controls reject changed failure reason/code, suppressed JSON
classification, changed error body, duplicated traffic and invented completion.
These are implementation-derived synthetic responses, not live Apple captures.

Generated command response parsers also preserve raw Body bytes for all media
and statuses. Their wildcard byte schemas prevent oapi-codegen from decoding
advertised JSON into []byte or routing successful replies through a default
JSON error parser. A contract test executes all 38 command reply artifacts
through the four generated parsers and requires exact byte equality. The shared
SDK raw reader still owns provider error classification and typed public errors;
canonical valid-JSON projections remain separately checked. Generation drift
checks bind this behavior to the checked-in schema (SCHEMA-10, SCHEMA-16).

Final local make lint/check passes with all Go race/contracts/regeneration/
consumer checks and 70 Python test methods. Handwritten replay coverage is
1518/1805 (84.1%), unit 1085/1805 (60.1%) and combined 1608/1805 (89.1%).
Fresh synthetic-only Source measurement remains 575/905 entered functions,
3149/5385 body statements (58.48%) and 964/2076 branch exits. New functional
command response cases exercise shared functions already reached by other
scenarios; unchanged diagnostic counts do not imply omitted functional cases
(LIB-07). Scoped independent working-tree review has no remaining blocker;
exact-commit approval after CI and full migration acceptance remain open (GO-15).
