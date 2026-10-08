# Find My public session

The [device-description and timed-loop follow-up](findmy-behavior.md) expands
the current SDK inventory to 47 scenarios and 86 pairs. Verification figures
below record the preceding PR15 session milestone.

The public session implements discovery/refresh and sound, message, lost-mode,
and erase requests using generated independent public models. Account credentials,
cookies, device records and provider continuation state belong to each session;
one stateless SDK can serve several accounts (API-03, API-04, API-13).

## Refresh behavior

Opening initializes Find My, then performs bounded family readiness polling.
The default waits are 500 milliseconds, at most five retries. Polling stops when
no members remain LOADING or the set of loading members stops changing.
`Refresh` uses the retained ordered provider context and the caller's locate
choice. Missing and empty content retain previously discovered devices; updated
records replace whole records and new IDs append in discovery order. The source
clears top-level theftLoss to null while preserving unknown context keys and
order. Empty or missing userInfo projects to null, matching the reference's
functional result. Unknown device and family metadata remain intact.

`Snapshot` returns copied cached data without performing a request. This is an
explicit Go lifecycle choice: Python dynamic device attributes can trigger a
refresh when its monitor is stopped. Call `Refresh` when fresh data is required;
local getters do not hide network calls. Snapshot remains usable after Close.

## Owned lifecycle and scheduling

The opening context bounds the session lifetime. Close cancels active requests,
queued calls and scheduler waits and is repeatable. It leaves caller-owned
transports and schedulers open. MonitorDone closes when the one owned monitor
exits; it is already closed when monitoring is disabled. Background refresh
runs every five minutes by default, requests no new locations, performs no
family readiness re-poll, and continues after a provider failure.

The Go session owns one cancellable loop, correcting the reference monitor
replacement race. Network calls serialize through a cancellable gate; state
locks protect copied state only. User scheduler callbacks and I/O run outside
those locks (GO-08, GO-09, GO-10). WithFindMyScheduler accepts cancellable waits
for replay or application scheduling. A scheduler must honor cancellation and
support concurrency if shared across sessions. Arbitrary scheduler failures
stop monitoring and return typed Transport failures; canceled and expired waits
retain their corresponding classifications and causes.

## Commands and response evidence

Advertised SND, MSG, WIP and lostModeCapable flags guard commands. Missing devices
return NotFound; unsupported capabilities return Unavailable without sending
anything. Erase obtains a fresh setup token and retains received cookie updates
before sending its command. Missing token fields prevent the command; an empty
but present token remains present as in the source. Setup and Find My cookies
keep their own response host/path scopes. No command is automatically retried.
Acknowledgement evidence never establishes completed physical action (API-11).

LastResponses reports all HTTP exchanges in the latest refresh or command,
including earlier stages before an error. Local capability refusals leave the
previous evidence intact. A local monitor scheduler fault clears the evidence
because that monitor operation made no request. LastError retains a copied
failure and clears after success; Close does not erase cached state. Error
headers, body, cause, prior responses and cookie scope remain inspectable.

## Verification and remaining work

All 37 current synthetic Find My scenarios and 70 request/response pairs run
through the public SDK driver. The driver selects public operations from scenario
inputs rather than choosing routes from recorded requests. It checks exact
request/response matching, complete consumption, family wait traces, full
functional snapshots and command acknowledgements (LIB-05). Independent model
schemas own the public projections; canonical wire models remain separate
(SCHEMA-02, SCHEMA-10). Separate focused controls exercise concurrent close,
monitor recovery, cancellable family waits, callback reentrancy, two-account
cookie rotation, and copied credentials, nested data and HTTP evidence under
race detection. Existing Drive controls cover the shared gate/cookie extraction.

This is a verified milestone, not full selected-service acceptance. Native Go
authentication and the reference's 417 forced-reauth retry remain pending.
Selected Source metadata/status/location and real timed monitor branches,
broader successful acknowledgement shapes, Photos/reminders ports, full runtime
provenance gates, published documentation and package release remain open.
Fixtures are synthetic implementation-derived evidence from timlaing/pyicloud
commit e2e44ab875d47dab4475096021da60030f26c35e; no private captures or tokens are
published. Live access remains read-only, and no live command was performed.

The preceding transport milestone PR14 was independently approved at exact
77e72a19cb1a9a5c950ee1c4ed55c40324d3d379 after CI 37828401650 passed, merged as
00586b140bb852f18c71e883818496e8dfecfffd, and its branch was deleted. Main CI
37829084493 also passed. This session milestone still requires final local
checks, CI and independent exact-commit approval before merge (GO-15).

Final local `make lint` and `make check` pass: all Go race/contracts/drift/consumer
checks and 67 Python tests. Handwritten replay coverage is 1456/1758 statements
(82.8%), unit 1045/1758 (59.4%) and combined 1567/1758 (89.1%). Live Go coverage
remains pending. Fresh synthetic-only Source measurement is 564/905 entered
functions, 3119/5385 body statements (57.92%) and 952/2076 branch exits. It excludes
private captures; the preceding combined captured/synthetic measurement of
576/905, 3195/5385 and 973/2076 is a different evidence set. These diagnostic
counts do not replace functional parity or full selected-service acceptance
(LIB-07). Global HTTP inventory remains 456 scenarios/1030 pairs, 74/75 draft
HTTP routes, zero unmatched; physical security-key interaction remains deferred.
