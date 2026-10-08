# Independent review record

Review state: **not approved**. No final implementation/release commit exists.
Both final reviewer verdicts must independently prove all numbered template
items; neither the implementer nor this initial audit signs off completion.

## Reviewer A — initial blind audit

Reviewer `blind_review` started with fresh context and only the repository path
and audit instructions. It independently inspected files/Git state and ran
`make lint`, `make check`, and private replay. At review time the repository had
no HEAD, tracked files, or tags. It observed 17 offline tests and 17 private
scenarios/95 exchanges passing. It did not inspect or print private data.

Verdict: uncommitted Python reference capture bootstrap, not a Go library ready
for migration/release sign-off.

| Standard | Reviewer finding | Disposition |
| --- | --- | --- |
| 1 | Bootstrap independent; no SDK to evaluate | Open |
| 2 | No public Go API, examples, canonical schema validation | Open |
| 3 | Required live badges missing | Open |
| 4 | SDK/schema/model/route inventory and negative gates missing; reference controls and socket edges unrepresented | Open |
| 5 | Local blocking checks/pinned CI configuration exist; no committed exact-CI evidence, formatting/module gates | Open |
| 6 | Scenario count has no completeness denominator or enforced SDK coverage | In progress: function/branch measurement and separate captured/synthetic reports added; SDK gate still open |
| 7 | Public/generated/transport package boundaries and consumer imports absent | Open |
| 8 | Go options constructor/validation absent | Open |
| 9 | Go sessions/account isolation absent | Open |
| 10 | Bridge TLS/WebSocket exchanges uncaptured; replay blocks HTTP only | In progress: TCP/socket/HTTP fallback guard added; bridge replay still open |
| 11 | Reference silently persists token rotations; Go caller-visible token exchange absent | Open |
| 12 | MDX guides/Pages/reference/rendered-site checks absent | Open |
| 13 | Copied website/release docs assert nonexistent workflows and retain placeholders | In progress: identified stale claims corrected and independently checked at bootstrap commit; full documentation audit still open |
| 14 | No checklist, combined reviewer record, reviewed commit, or second reviewer | In progress: checklist/current record added; final reviews still open |
| 15 | Private Python snapshots/raw values are not portable sanitized Go fixtures; failure type-only comparison; login/socket/teardown gaps | Open |
| 16 | Python reference CLI is not an installable Go SDK-consuming module | Open |

Changes marked in progress require this reviewer to verify the final fix; the
implementer's disposition is not an independent passing verdict.

### Reviewer A — interim fix audit

The same independent reviewer inspected commit
`dd374577c340eca7c3ad587abe0b177f3f5a2843` and independently reran `make check`,
23 portable Drive/account scenarios (23 exchanges), and coverage measurement.
It confirmed strict request/result/error matching, consumption, HTTP/TCP
fallback prohibition for examined paths, separate captured/synthetic contexts,
and the corrected infrastructure claims. Combined coverage reproduced
176/915 entered functions and 804/5,436 function-body statements. Synthetic-only
measurement credited zero statements to captured evidence.

Verdict: **these limited bootstrap fixes pass; full library acceptance does
not pass**. Actual Go interoperability, complete sockets/endpoint discovery,
portable captured state, captured error semantics, and all SDK/release gates
remain open. Coverage by evidence describes execution contexts: one-time
initialization belongs to whichever context first runs it, so synthetic-only
and combined synthetic counts need not match. The reviewer did not verify the
later Reminders and Find My additions in this audit.

### Reviewer A — expanded reference audit

At `37381acfbfeb5ffadfd24620d20f232c94aa5578`, the reviewer independently passed
`make check` (27 offline methods), 65 portable scenarios/90 exchanges, and
combined measurement (221/915 functions; 1,005/5,436 statements; 267/2,094 branch
exits). All 76 draft inventory rows matched their pinned source function names
and definition lines. It verified the limited Photos, Reminders, Find My,
Drive-transfer, UUID-pattern, and timestamp claims while leaving full acceptance
open.

The reviewer found that multipart matching ignored MIME preamble/epilogue and
accepted appended bytes inconsistent with Content-Length. It also identified a
stale statement claiming fixtures contained no cookies, although upload cases
have invented values. Fixes now reject preamble/epilogue, validate byte length
before every replay send, add negative controls, and correct that statement.
Disposition: **both findings independently resolved** at
`b002f65f8e520d687e2b7448208480e560b4ba00`. The reviewer reran its original
appended-byte exploit and preamble/epilogue/blank-padding variants with corrected
Content-Length. All were rejected before response delivery with the exchange
index still zero. It also checked invalid framing lengths, the corrected
credential statement, and passing `make lint`/`make check` (29 offline methods).
Full migration findings remain open.

### Exact-commit CI availability

The [draft migration PR](https://github.com/portpowered/go-icloud/pull/1) is open.
The [bootstrap CI run](https://github.com/portpowered/go-icloud/actions/runs/37740660821)
for `b002f65f8e520d687e2b7448208480e560b4ba00` failed before starting any job
steps. GitHub's check annotation reports an account payment/spending-limit
restriction; the runner name is empty. This is not a failing code test or passing
CI evidence. Local checks and independent retests do not substitute for the
required CI.

The account owner made the repository public on 2026-10-08. The subsequent
[CI run](https://github.com/portpowered/go-icloud/actions/runs/37740853708) for
`d7f45d05f2df5f586da656398d448e52fafab0bc` completed successfully, including
pinned reference setup, all-linters, build/race checks, and 29 offline methods.
Actions availability is restored. This verifies the committed bootstrap checks;
the SDK/module/generation/source/model/coverage gates and final independent
exact-commit CI verification remain open. Do not mark standard 5 satisfied.

## Reviewer B — final audit pending

### Interim authentication socket audit

The independent reviewer evaluated the 18 implementation-derived socket
scenarios (88 ordered duplex events) against the actual pinned raw client.
Its original exploit appended an extra frame or arbitrary trailing bytes to a
coalesced receive; replay wrongly accepted both because only transcript buffers
were checked. The runner now retains the reference instance across initializer
failure and rejects unconsumed bytes in its own buffer after teardown. The
reviewer independently reran both exploits and the failed-upgrade regression;
the consumption finding is **resolved**.

The limited audit verified route/SNI/timeout/ownership, ordered send/receive,
explicit upgrade entropy binding, canonical masked framing, decoded payloads,
semantic outcomes, and consumption at both buffering layers. `make lint` and
`make check` passed (35 offline methods). Synthetic-only measurement includes
all 18 socket cases and credits zero captured functions. Failed upgrades show
no explicit close call; this injected seam does not prove persistent live leaks.

Verdict: **passes this interim seam only**. TLS cryptography, bridge protobuf and
bootstrap exchanges, full login lifecycle, live socket captures, and all final
library criteria remain open. This audit is not final Reviewer B sign-off.

Not assigned yet. Assign a reviewer with fresh context after a complete candidate
exists. Record its reviewed commit and separate verdict/evidence for every item.

## Final verification pending

### Interim modern proof and full-login audit

The independent reviewer replayed fifteen modern SPAKE2 cases with 65 HTTP
pairs, then three complete login flows with another 31 HTTP pairs and 32 socket
events. It reconstructed the server-side scalar-3 handshake, verified client
confirmations and directly AES-GCM-decrypted the final payloads for six completed
verification flows. The decrypted invented code matches the exact HTTP entity.
It independently verified SRP peer proofs for all three complete login flows.

The full flows begin with empty account/session/challenge/cookie state and invoke
actual `authenticate`, automatic bridge discovery and `validate_2fa_code`.
Successful modern/legacy cases finish trusted with MFA cleared; modern 412 stays
untrusted with MFA required and omits trust/account setup. No active prover
algorithm is replaced. Declared bounded scalar draws are consumed and sticky.

The reviewer confirmed rejected proof/challenge/ciphertext and mixed timeline/
MFA-state mutations, caught wrong-bound-to-valid entropy retry, exact decrypted
code mismatch and factory restoration across success and HTTP/crypto failures.
Private recording still redacts passwords/codes. Independent `make check`
passes all 60 methods, including the 319-scenario sweep, lint, build and race
checks. Current totals independently enumerate 319 HTTP scenarios/709 pairs.

Verdict: **limited interim pass**. This establishes pinned-reference synthetic
proof/login compatibility. Live TLS/socket evidence, Go interoperability and
all final SDK/release acceptance criteria remain open. Documentation is updated
to reflect this scope; physical security-key interaction remains deferred.

### Interim combined trusted-device bridge audit

The independent reviewer replayed twelve combined cases with 20 HTTP pairs and
99 socket events. Actual source bootstrap/key generation/signing and raw framing
execute with declared curve entropy and TCP/TLS factories; the signature is
verified independently while other bootstrap fields match exactly. A shared
timeline binds HTTP to socket setup, token/subscription/push and close behavior.
Cases cover nonce retry, prompt setup/idempotence, provider step-0 statuses,
refusal, session mismatch, SMS fallback and legacy device-code outcomes.

The reviewer found that rejected extra key or connection attempts could be
caught without failing the replay. Shared sticky validation now covers those
factories and every socket boundary. The reviewer independently reran both
original exploits and confirmed rejection with restored factories. Added
controls also reject a caught frame mismatch followed by retry, invalid
signatures, reordered traffic, trailing client-buffer bytes and unused scalar,
connection or timeline declarations. The finding is **independently resolved**
under LIB-05. Independent `make check` passes all 57 methods.

Verdict: **limited interim pass**. Synthetic evidence does not establish live
trusted-device interaction or TLS cryptography. Modern SPAKE2 verification,
complete authenticated bridge login, Go interoperability and final SDK/release
criteria remain open. Physical security-key interaction is deferred.

### Interim SRP sign-in audit

The independent reviewer replayed eight SRP scenarios with 28 HTTP pairs and
verified both proof values for all six sign-in completion exchanges with a
separate synthetic server verifier. Both password protocols execute actual
reference stretching/SRP calculations with declared client entropy; no provider
business method is replaced. Cases bind successful setup, provider refusal,
trust-token forwarding, paused MFA login and SMS challenge discovery/delivery.

The reviewer passed the proof/entropy controls and six additional mutations of
salt, challenge, iteration, protocol, resulting state and response consumption.
Entropy and clock factories restore after success, refusal and mismatch.
Independent `make check` passes all 51 methods, including lint, build and race
checks. No actionable flaw was found in this limited extension.

Verdict: **limited interim pass**. Evidence remains synthetic and
implementation-derived. Trusted-device bridge lifecycle, hardware authentication,
Go interoperability and all final SDK/release acceptance criteria remain open.

### Interim authentication HTTP/session audit

The reviewer independently passed 51 auth scenarios with 83 pairs and latest
`make check` with 50 methods. The cases bind token/cookie rotation, verification,
trust, PCS retries, terms, account setup and logout to actual pinned methods.
Portable session projections include challenge/cookie policy and file presence.
They do not serialize Python objects or real credentials.

Two concrete findings were fixed under LIB-05. Auth failures previously omitted
caller argument state; the reviewer now rejects missing/changed expectations
and its original late-mutation exploit. Removing retry entropy previously allowed
an ambient sleep. Auth and service-upload clocks now reject undeclared events;
the reviewer confirmed missing traces/whole entropy cause zero real sleeps,
caught violations remain sticky and factories restore. Both findings are
**independently resolved**. The added failure-context binding also rejects
changed exposed HTTP responses, codes and missing expectations.

Verdict: **limited interim pass**. Full SRP, bridge lifecycle, hardware
authentication, Go interoperability and all final acceptance criteria remain open.

### Interim upload hydration and account endpoint audit

The reviewer independently replayed 16 Photos service-upload cases with 83
pairs and 11 account photo/summary cases with 17 pairs. `make check` passed 48
methods, including lint/build/race checks. Twelve added Photos probes rejected
extra responses, unused/nonfinite/decreasing clocks and wrong event order;
five account probes rejected region, result, error and consumption drift.
Clock, UUID and timezone factories restored after success, expected failure
and replay mismatch.

Actual HTTP 429 produces the pinned session error before hydration retry handling.
Account family-photo and summary routing match the pinned source. The reviewer
identified stale hydration/count documentation; the fixture guide and migration
checklist now reflect the new evidence. Verdict: **limited interim pass**. Full
endpoint/schema/Go interoperability and final acceptance remain open.

### Interim Photos upload and shared-stream audit

The reviewer independently checked 44 new cases with 112 pairs: actual upload
reservation, file transfer, registration/progress and pipeline behavior, plus
shared-stream albums/counts/assets/lookups/downloads. File snapshots preserve
position and bind remaining entity bytes; streamed responses support raw reads
and closure. Clock, timezone and imported UUID factories restore after replay.
These are synthetic HTTP entity cases, not live writes or TCP chunk transcripts.

The reviewer found that a caught CloudKit exception's caller-visible payload
could change while type/message still passed. Upload fixtures now require exact
`error_payload` projections. The reviewer reran its original execute-wrapper
exploit and verified all 12 CloudKit failure cases reject changed or missing
payloads. Actual HTTP errors retain their reference exception classification.
Unsupported stream errors also remain sticky across attempted recovery.
Disposition: **payload finding independently resolved** under LIB-05.

Both local checks and the independent `make check` pass 47 methods. Verdict:
**limited interim pass**. Service upload hydration/target-album membership,
remaining endpoint variants, Go interoperability and full acceptance remain open.

### Interim Photos endpoint audit

The independent reviewer passed `make check` with 43 methods and confirmed
159 HTTP scenarios with 270 paired exchanges. Its extra probes rejected changed
same-size entropy, surplus random samples, incorrect returned change tags,
caught extra HTTP traffic after all pairs were consumed, and caught socket
fallback. Entropy and clock factories restored after success, provider rejection
and failed probes. The sticky guards close the swallowed-refresh-error bypass
under LIB-05.

The actual pinned Photos methods construct requests. The injected album-position
helper contains only a timestamp calculation; portable projections bind metadata
and record fields. The reviewer verified the documented asset-deletion defect:
per-record rejection still returns true in the reference. The Go implementation
must report that rejection through an explicit tested departure.

Verdict: **limited interim pass**. Uploads, complete shared-stream coverage,
canonical schemas/generated models, Go interoperability and all final acceptance
criteria remain open.

### Interim Reminders endpoint audit

The independent reviewer passed the Reminders extension across 128 HTTP
scenarios and 159 paired exchanges. It independently ran all four synthetic
test methods and `make lint`. Extra probes rejected changed caller titles,
incorrect error-state tags, surplus UUID samples on provider rejection, and
unconsumed exchanges. It verified clock/UUID restoration after successes,
provider errors, and rejected probes. Local `make check` also passed 40 methods.

The reviewer confirmed that JSON model/datetime descriptors are portable caller
inputs and that actual pinned methods emit CRDT/protobuf and linked-record
requests. Success and rejection projections check caller-visible model state.
The HTTP fixture guide now documents the expanded cases and entropy contract.
Verdict: **limited interim pass**. Observed Apple writes, canonical schema
bindings, Go interoperability, complete endpoint coverage and final migration
acceptance remain open.

### Interim bridge-message and deadline audit

The reviewer independently passed the extension through selected subscription,
push-token and push helpers: exact acknowledgments, topic filtering, semantic
nonce timestamp and errors, and consumed monotonic traces. Its additional
probes rejected missing acknowledgments, changed message IDs, appended unread
frames, missing closure, empty/boolean/decreasing/insufficient/surplus clock
traces, and premature expiration. It verified restoration of `time.monotonic`
after successes, provider failures, and rejected probes. The extended socket
population is 53 scenarios with 283 events. This verifies deterministic deadline
behavior, not real scheduling/cancellation, full bootstrap or prover behavior.
This remains a limited interim pass, not final full-library acceptance.

The owner subsequently prioritized selected endpoint behavior and deferred
additional generic input/utility testing. The reviewer confirmed ten diagnostic
coverage exclusions against actual source usage: five excluded service facades,
four reference-CLI password helpers, and a calendar-only helper. Auth-used
keyring lookup remains included, and candidate network discovery still scans
the complete included files. It found that whitespace-only reasons passed the
exclusion gate. The gate now requires a non-whitespace string; the reviewer
independently verified whitespace, boolean, and empty reasons fail and valid
exclusions remain recorded. Disposition: **scope finding resolved**. This is a
disclosed denominator correction, not endpoint completeness or final acceptance.

### Interim Drive endpoint audit

The independent reviewer passed all 36 Drive additions and 58 paired exchanges,
including provider refusal at upload/download stages, node-backed endpoints,
cached and refreshed folder navigation, empty files and transfer state.
Its probes rejected changed or missing arguments, file positions, parameters,
node state and exposed HTTP errors. It independently verified deep-copy
projections remain snapshots after later node-data mutation, and ran `make check`
with all 62 methods passing. Drive now contributes 59 scenarios and 89 pairs
to the 355-scenario, 767-pair HTTP suite.

The reviewer reproduced the negative non-hour date-offset defect and checked
its disclosed deliberate Go correction. Historical checklist count wording was
corrected and independently verified. Verdict: **limited interim pass** with
no remaining blocker for this extension. Complete selected-endpoint/schema
gates, Go interoperability and final migration acceptance remain open.

### Interim Find My refresh audit

The reviewer independently passed 18 additions with 37 paired exchanges and all
30 corrected Find My scenarios with 55 pairs. Probes rejected missing waits,
changed cache/error state, surplus responses and historical token-path prefixes;
clock factories restored and no monitor thread survived. Independent `make check`
passed all 63 methods. The full HTTP suite is 373 scenarios and 804 pairs.

The reviewer checked `/setup/ws/1` erase-token routing against the actual account
facade and independently reproduced retained-device behavior and the monitor
replacement race. It verified their documentation and required Go correction.
Verdict: **limited interim pass** with no scoped blocker. Complete selected
endpoint/schema gates, Go replay interoperability and migration/release acceptance
remain open.

### Interim Go endpoint occurrence audit

The reviewer independently reconciled the Go report: 373 scenarios, 804 exchanges,
66/75 HTTP routes, nine missing routes and no unmatched exchange. It passed
negative controls for active pin/evidence, templates, ambiguity, temporal URL
binding and strict gaps. Strict CLI failure, default working-directory behavior
and explicit root selection were independently checked. Lint, race-enabled Go
tests and full `make check` passed.

The reviewer confirmed the documentation limits this to diagnostic HTTP
occurrences, separately from private captured evidence, socket coverage and
source/schema/functional acceptance. Default CI mode reports the gaps; it does
not silently approve them. Verdict: **limited interim pass** with no actionable
flaw. Complete selected endpoint/schema/model gates, Go SDK interoperability,
release and final migration acceptance remain open.

### Interim Photos container and Reminders zone audit

The independent reviewer passed all 40 additions and 145 paired exchanges.
Source inspection confirmed the active shared batch-count route and the dormant
Reminders database-change helper: no Reminders service, adapter or CLI operation
calls or exposes it. The reviewer verified shared favorite mutation routing and
its mandatory shared refresh against the pinned source.

The reviewer found that Photos errors checked type/message without binding
their optional photo/album resources. The runner now requires nullable portable
projections. Independent probes that drop the actual exception's photo, inject
an album, omit expectations or change resource fields all reject. Disposition:
**error-resource finding resolved** (LIB-05).

Independent `make check` passes 65 methods, lint, build and race-enabled Go tests.
The occurrence audit reports 413 scenarios, 949 pairs, 74/75 active HTTP routes
and no unmatched exchange. Physical security-key verification remains deferred.
Verdict: **limited implementation pass**. Selected-endpoint functional/schema
gates, Go SDK interoperability and final migration acceptance remain open.

### Interim Go exact-entity replay transport audit

The reviewer independently inspected `tests/replay/http.go` and its race-enabled
tests. It found an unbound Go Close flag and request bodies left open on early
rejection. The transport now rejects Close and closes request bodies on every
exit before exposing an outcome; close errors suppress responses and remain
sticky. Independent target/query/unexpected-request and Close probes verify both
findings are resolved (LIB-05, GO-09).

The reviewer found no further scoped flaw in immutable snapshots, ordered query
and header matching, exact entities, sticky failures, response read/close
consumption, mutex ordering or network isolation. It passed the focused race
suite and documentation check. Verdict: **limited interim pass** for exact-base64
HTTP transport replay. This is not Go SDK semantic replay; structural match rules,
socket/auth timelines, production schemas and the client remain open.

### Interim Go JSON rule audit

The reviewer independently checked the 14 credential-redaction and two JSON
pattern declarations against the pinned reference. It confirmed nonempty string
passwords, six ASCII digit codes, exact fixed structure and large-number
preservation. It found that escaped equivalent paths bypassed raw duplicate
checks and let one replacement hide a later format violation.

Paths now use canonical typed identities, equivalent duplicates reject and all
patterns check original request values before replacements. The original bypass
independently fails construction. Focused race tests and documentation checks
pass. Verdict: **limited interim pass** for JSON request rules (LIB-05).
Constructor acceptance of 16 declarations is not SDK semantic replay; multipart,
socket/auth timelines, projections, schemas and the client remain open.

### Interim Go multipart rule audit

The reviewer independently replayed all seven selected upload multipart pairs
with a new 32-hex boundary. All pass. Its 28 additional probes for bare-LF
framing, blank preamble/epilogue and extra part headers reject before a response
is returned. It verified part order, exact headers/data, outer Content-Length
and header matching, constructor snapshots and encoded/nested-part rejection.
Only declared Content-Type variation is normalized after full matching.

Independent race-enabled tests confirm all 413 portable scenarios/949 exchanges
instantiate and seven upload pairs match Go's writer. Documentation accurately
limits those counts to matcher interoperability. Verdict: **limited interim
pass** (LIB-05). SDK semantic replay, socket/auth timelines, production schemas,
the client and final migration acceptance remain open.

### Interim account wire-contract audit

The reviewer checked the five account routes against the pinned reference and
independently passed race tests and fresh model generation. It found three
gaps: fixture checks bypassed actual operation response bindings, named raw
JSON values lost explicit nulls, and media usage omitted required `mediaKey`.
Checks now use the operation's status/media schema with an explicit owner gate;
generated nullable wrappers preserve null and absence; mediaKey is required.
The reviewer independently verified all three fixes (SCHEMA-07, SCHEMA-11,
SCHEMA-16). Verdict: **limited contract pass**. Exact-commit review approved
`2054323d9e5a9cd2b74948693244946ce9d381e1` after
[CI 37772542619](https://github.com/portpowered/go-icloud/actions/runs/37772542619)
passed. The milestone merged in PR #1; main commit
`81b84223ed26c2a0862459d3b9241528788f9122` also passed
[CI 37773064544](https://github.com/portpowered/go-icloud/actions/runs/37773064544).
Public account operations, full source/schema gates and
SDK migration acceptance remain open.

### Interim generated account-client interoperability audit

The reviewer independently passed race tests for all 18 account scenarios and
both config-based generation checks. It verified that requests derive from
portable initial account state and returned family member IDs, not expected
requests. The route schema references one shared canonical wire-model owner;
generated clients import aliases instead of duplicating definitions.

The binary member-photo adapter preserves status, headers and exact bytes while
reading and closing the response. Read/close failures suppress output and retain
both causes. The generated JSON-default parser limitation is documented.
The reviewer approved the exact-file revive package-comment exception because
the generated sibling owns the package godoc (GO-13, GO-15).
Verdict: **limited interoperability pass**. Public SDK projections, session
isolation, client error behavior and full migration acceptance remain open.
Exact-commit review approved `18cd869332f07c2c6530978b7b7149f3bca15578`
after [CI 37774763797](https://github.com/portpowered/go-icloud/actions/runs/37774763797)
passed. PR #2 merged; main commit `c9488cf353ce88f4f82771d726dbdb37c49687e7`
also passed [CI 37775195864](https://github.com/portpowered/go-icloud/actions/runs/37775195864).

### Interim public account-device SDK audit

The reviewer independently passed eleven portable device replays, concurrent
account isolation, model generation and independent consumer compilation under
the race detector. It reproduced 80.2% replay, 88.2% unit and 91.4% combined
handwritten SDK/internal transport coverage. The exact-file projection package
comment exception is justified because the generated sibling owns the godoc.

The reviewer found that Content-Type was still a generator literal rather than
a schema-owned header. The external account schema now owns that declaration;
the generator derives the constant and a rename control rejects a hardcoded
fallback. The reviewer independently verified the fix (SCHEMA-10).
Verdict: **limited milestone pass** for GetAccountDevices. Exact-SHA review
approved `a0946b15e596a4983b191671b30015513d184063` after
[CI 37783571677](https://github.com/portpowered/go-icloud/actions/runs/37783571677)
passed. PR #3 merged in `ff819ed3a2f507824985ebe78026c8d0a2594ee4`; main also passed
[CI 37784242661](https://github.com/portpowered/go-icloud/actions/runs/37784242661). Remaining account operations, native
authentication and full migration acceptance remain open.

### Interim remaining account SDK operation audit

The reviewer independently passed all 34 account scenarios/40 pairs through the
public SDK, projection checks, body cleanup, schema generation and independent
consumer compilation. It reproduced 81.6% replay, 82.5% unit and 92.2% combined
handwritten SDK/internal transport coverage. Regional routing stays per request;
nullable family fields and opaque JSON retain their contract-defined values.

No actionable scoped findings remain. Verdict: **limited milestone pass**
for the five account reads. Exact-SHA review approved
`076aced2d6469ba871a38fedc07cd9b73f26ef3f` after
[CI 37787206097](https://github.com/portpowered/go-icloud/actions/runs/37787206097)
passed. PR #4 merged in `bb7eaf9baccfbc3511a2f3ea268accee062e0496`; main passed
[CI 37787956138](https://github.com/portpowered/go-icloud/actions/runs/37787956138).
Native authentication, other selected services and full migration acceptance
remain open.

### Interim Drive wire-contract audit

The reviewer independently passed 59 Drive scenarios/89 pairs across 13
operations, three generation drift checks, unknown-value presence and timestamp
controls. It found that the temporary folder identifier accepted arbitrary UUID
versions/variants. The schema now requires the reference's lowercase UUIDv4
with RFC 4122 variant; separate negative controls reject wrong versions, variants
and casing. The reviewer verified the fix (SCHEMA-15) and the exact-route/method
issuer checks for provider-issued content URLs.

Verdict: **limited milestone pass**. Exact-SHA review approved
`dfd6c6041317dc01cb288ab765b92c6b43de1b7b` after
[CI 37790183860](https://github.com/portpowered/go-icloud/actions/runs/37790183860)
passed. PR #5 merged in `268d6f2973cfdaf3dd468b7ae5c08f05afed74c4`; main passed
[CI 37791000251](https://github.com/portpowered/go-icloud/actions/runs/37791000251).
Drive SDK semantic replay, complete runtime/source gates and the full
migration acceptance remain open.

### Interim Drive node-upload reference audit

The reviewer independently passed all 19 synthetic-runner test methods, Drive
contract race checks and Go paired multipart checks. It enumerated 437 HTTP
scenarios/991 pairs, 67 Drive scenarios/115 pairs, eight additions/26 pairs and
13 multipart writer pairs. The pinned `DriveNode.upload` and service facade
delegation execute; instrumentation only constructs/closes its own input stream
and snapshots cursor/state. Failure and wire mutations remain fail-closed.

Verdict: **limited milestone pass** for the reference gaps. Exact-SHA review
approved `33213f1a5a4491fabbd4ec77a1b90800ab0aa2aa` after
[CI 37792226591](https://github.com/portpowered/go-icloud/actions/runs/37792226591)
passed. PR #6 merged in `0c52ea5cf1c6f948ebf69c1574f6640cd0c14ecc`; main passed
[CI 37792909781](https://github.com/portpowered/go-icloud/actions/runs/37792909781).
Drive SDK semantics and full migration gates remain open; diagnostic coverage
and local-helper gaps do not establish completeness.

### Interim Drive SDK read audit

The independent reviewer reproduced two wire discrepancies: a nonnil empty
sharing descriptor was sent instead of omitted, and U+007F was sent literally
instead of escaped. Both are fixed and a public-method regression binds their
exact request bytes. The reviewer reran its original probes and verified both
corrections against the pinned reference.

Independent race checks pass for the public SDK, shared web transport, replay,
contracts and constant generator. Nine existing portable reads exercise actual
public methods, generated builders and semantic results. Optional recursive
children, sharing fields, unknown nulls/large numbers, malformed provider replies,
error evidence, schema constant drift and an independent consumer are checked.
Existing account replays pass after moving the HTTP mechanics into the shared
transport (API-02, API-03, LIB-01).

Verdict: **limited milestone pass** for `GetDriveNode` and
`ListDriveLibraries`. Exact-SHA review approved
`d807198bcf0cd03d8dc929811378c93b4a8ba1f0` after
[CI 37795658444](https://github.com/portpowered/go-icloud/actions/runs/37795658444)
passed. PR #7 merged in `bab7d9e5f8c4f56f3a5f8bfa41e02d5ec3a71fea`; main passed
[CI 37796621601](https://github.com/portpowered/go-icloud/actions/runs/37796621601).
Navigation, mutations, transfers and complete migration acceptance remain open.

Reviewed commit, complete wire-model/endpoint inventories, CI run URLs, release
tags, proxy verification, Pages inspection, and reviewer verification of each
fixed finding remain to be recorded. Every unresolved finding prevents sign-off.

### Interim Drive SDK mutation audit

The reviewer independently passed sixteen portable mutation scenarios through
seven public methods and verified their generated request bodies against the
pinned source. It found that a truncated `x-protocol-prefix: FOLDER` declaration
could generate folder identifiers rejected by the same schema. Exact anchored
literal-prefix checking and a prefix-plus-UUIDv4 check now reject that case.
The reviewer reproduced its original probe and verified the fix. Negative
controls reject truncated, empty, mistyped and unanchored declarations; a
changed valid pattern/prefix produces the corresponding constant (SCHEMA-10,
SCHEMA-15).

Independent race checks pass for all seven mutation body-cleanup failures,
provider acknowledgements, creation-header ownership, unknown/missing-list
metadata and repeated two-account paired rename isolation. The Client-only
interfacebloat exception is accepted as a narrow architecture choice supporting
one traced interface; API-01 requires operations together in one file and does
not itself require a single interface. No global linter is disabled (GO-15).

Verdict: **limited milestone pass**. Exact-SHA review approved
`97b5449b9196ddae324f2fbbadc012e5d039bc2a` after
[CI 37800827064](https://github.com/portpowered/go-icloud/actions/runs/37800827064)
passed. PR #8 merged in `f3fe89ba4e95e5b1a759cb3df9726f68d090cd1b`; main passed
[CI 37801891576](https://github.com/portpowered/go-icloud/actions/runs/37801891576).
Twenty-five Drive read/mutation scenarios do not establish parity for
all 67 reference scenarios. Navigation, transfers and complete migration/release
acceptance remain open.

### Interim Drive SDK download audit

The independent reviewer passed SDK, replay, contract, generator-drift and
consumer-build checks under the race detector. It found an unquoted comma in
an inline YAML description, which silently created an extra sibling field.
Quoting the description and regenerating resolved the schema-example failure;
the reviewer independently verified the correction (SCHEMA-11).

Seven existing download scenarios verify full public status/header/byte results
and typed failures through the generated token request and issuer-bound content
adapter. Focused controls check token preference, escaped paths/zones, repeated
provider queries, binary data labeled JSON, both-stage cleanup and copied prior
response metadata. Repeated two-account paired downloads exercise both stages
on one shared client with all eight expected exchanges consumed (LIB-05, API-03).
The reviewer found that identical stage cookie values and header-count checks
could hide swapped metadata. Distinct tenant/stage values and exact header-value
assertions now detect that gap; the reviewer independently passed the revised
race replay.

The reviewer additionally probed an HTTP 200 JSON content refusal: downloaded
bytes are suppressed, while content response headers and copied preceding
token metadata remain caller-visible. Documentation explicitly states that
content requests reuse supplied headers and expose intermediate cookies
afterward. Complete reference session cookie-jar parity remains open.

The final status-policy audit independently passed 204/206 SDK and contract
controls. Successful content accepts 2xx responses while ordinary reads retain
their existing exact-200 policy; JSON provider-error handling remains active.

Verdict: **limited milestone pass**. The independent reviewer approved
`7dda2150faada1ab10740dba7748e81abcb59d86` after
[CI 37805115228](https://github.com/portpowered/go-icloud/actions/runs/37805115228)
passed. PR #9 merged in `05be38d814bc93d196e3c498d3f83a4c04d5db9e`; main passed
[CI 37805852612](https://github.com/portpowered/go-icloud/actions/runs/37805852612).
Thirty-two Drive read/mutation/download scenarios do not establish
parity for all 67 reference scenarios. Navigation, uploads and full migration,
documentation and release acceptance remain open.

### Interim account cookie-context audit

Six new implementation-derived portable Source scenarios independently replay
against the pinned dependency and through the public SDK. Paired requests and
complete download results bind cookie rotation, host/path scope, expiry,
deletion, repeated updates and explicit-header precedence (LIB-05).
The inventory is 443 HTTP scenarios/1003 pairs, 73 Drive scenarios/127 pairs,
and thirteen public SDK downloads; Drive SDK semantics cover 38/73 scenarios.

The reviewer found that response headers lacked the dynamic issuer origin/path
needed for caller-owned cookie persistence, and that an explicit seed domain
could broaden a host-only cookie. CookieScopeURL now preserves the response
request origin and escaped path without query credentials; HostOnly binds
supplied seeds exactly to their host. Both findings were independently verified
fixed. Six strict exchanges prove content-stage host-only cookie persistence
across calls without sending it to the token host or a later content subdomain.
Repeated concurrent two-account downloads verify fresh operation-local jars,
unchanged caller inputs, explicit-header precedence and stage-specific metadata
(API-03, API-13). The shared client retains no account jar.

The reviewer independently passed Source, SDK, schema, consumer, generator and
race checks. `make lint` and `make check` pass, including all 67 Python tests.
Handwritten SDK/internal coverage is 604/729 replay statements (82.9%),
608/729 unit (83.4%) and 669/729 combined (91.8%); live Go integration is pending.
The two exact-line cookie flag-copy exceptions preserve supplied native protocol
attributes rather than imposing browser policy; all linters remain enabled.

Verdict: **limited milestone pass**. Exact-SHA review approved
`21f1d5f0ed98d61a1bd983215fa3c355ececda8a` after
[CI 37809511428](https://github.com/portpowered/go-icloud/actions/runs/37809511428)
passed. PR #10 merged in `51a4fd7b59dae39ad67c15957d8f0d1e11cf9ad5`; main passed
[CI 37810409609](https://github.com/portpowered/go-icloud/actions/runs/37810409609).
Native auth/session persistence, uploads, navigation and full
migration/release acceptance remain open.

### Interim Drive service-upload audit

Nine portable Source upload scenarios execute through `UploadDriveFile`,
covering empty content, a nonzero cursor, custom zones, successful 201/202
responses and refusal at each stage. Complete request matching and public
receipt/registration projections include unknown metadata and each completed
stage's response evidence. The independent consumer compiles all sixteen
public client methods. Drive SDK semantics now cover 47/74 scenarios.

The reviewer found missing required provider fields could allow another write,
failure tests omitted Source cursor/parameter state, Windows filenames needed
platform basenames, and preparation/registration rejected successful 2xx
responses. Required-field validation, fourteen strict malformed-provider
controls, failure-state assertions, platform path handling and schema-owned 2XX
responses resolve those findings (LIB-05, API-14, SCHEMA-04). The reviewer
independently rechecked every fix and passed scoped race tests for the SDK,
replay, contract/consumer/drift and constant generator packages.

Cookie names, token patterns and multipart disposition templates are generated
from canonical schema declarations. Generator controls verify declaration
changes and reject invalid patterns/templates, as required by template rule 4
(SCHEMA-10). The caller retains ownership of the reader and extracted upload
token. No live write was performed.

`make lint` and `make check` pass, including all 67 offline Python tests.
Handwritten SDK/internal coverage is 773/918 replay statements (84.2%),
610/918 unit (66.4%) and 833/918 combined (90.7%). Live Go integration remains
pending. Local lint used the same pinned all-linters executable with parallel
runners enabled because another repository held its shared process lock.

Verdict: **limited exact-commit pass**. The reviewer approved
`6217ed8e2f7d24ccabc878b8a3ed252a3959c89a` after
[CI 37815289981](https://github.com/portpowered/go-icloud/actions/runs/37815289981)
passed. PR #11 merged as `949a4c73b4041fee360517ca0d1b74dfc08daa5a`; main passed
[CI 37816040773](https://github.com/portpowered/go-icloud/actions/runs/37816040773).
The following milestone implements node/session behavior; full migration and
release acceptance remain open.

### Interim Drive node/session audit

Thirty-two portable node flows now execute the public session and entry methods,
bringing semantic Drive replay to 79/79 scenarios and 139 paired exchanges.
Five new implementation-derived Source cases bind file navigation guards,
trash-only recovery/permanent deletion, and upload followed by refresh with
updated cookies and token. The inventory has 449 HTTP scenarios and 1015 pairs;
all remain explicitly synthetic. The consumer compiles all seventeen client
methods and the session/entry operations. No live write was performed.

The reviewer identified host-case cookie deletion and a local seek failure that
erased previous network evidence. Both fixes were independently re-probed.
Strict controls bind rotation/deletion, native cookie attributes, foreign-domain
rejection, explicit-header precedence and snapshot copy ownership. The origin
validator rejects path-prefixed origins before I/O. Two tenants share one client
and concurrently execute isolated upload/refresh flows. Close cancels current and
sixteen queued requests, is repeatable, and preserves caller resource ownership.
Expired contexts retain a typed timeout cause. The lifetime context has one
documented `containedctx` exception on its field (API-04, GO-09).

Independent race checks passed SDK, replay, contract/consumer/drift and constant
generator packages. The limited implementation review found no remaining code
blocker. `make lint` and `make check` pass, including all 67 Python tests and the
80% replay and combined coverage gates. Handwritten coverage is 1125/1351 replay
statements (83.3%), 695/1351 unit (51.4%) and 1191/1351 combined (88.2%). The
Source diagnostic measures 576/905 entered functions, 3195/5385 body statements
and 973/2076 branch exits. Live Go integration remains pending (LIB-07).
Verdict: **limited exact-commit pass**. The reviewer approved the final docs and
independently matched the coverage profiles and inventory to this receipt.
Exact SHA `bd5cfc376cf8644ba038604405036a86f0daed0f` was approved after
[CI 37820415070](https://github.com/portpowered/go-icloud/actions/runs/37820415070)
passed. CI's unit measurement is 51.5%, versus local 51.4%, a one-statement
execution variation. PR #12 merged as `d4fb128df3d4e55fb45bc90a20b8a66ce4f915eb`;
main passed [CI 37821166996](https://github.com/portpowered/go-icloud/actions/runs/37821166996).
The full migration criteria remain open.

### Interim Find My wire-contract audit

Seven operation contracts now bind all 36 portable Find My scenarios and 68
exchanges. Canonical generated models round-trip all 63 JSON responses; separate
initialization/refresh models, query controls and protocol-constant generation
bind the recorded shapes. Four invented schema examples validate. Drift,
unknown-field/null-presence and fixed-protocol constraint checks pass.

The independent reviewer found unformatted schema numbers generated 32-bit
floats and reduced coordinate/battery precision. Explicit double formats and
nontrivial decimal regressions address the finding (SCHEMA-04, SCHEMA-10).
The reviewer independently re-probed the precision fix. A second finding was
that non-200 command acknowledgements incorrectly bound to JSON errors. Four
new empty 204 command cases and one binary 201 case pass actual pinned Source
replay, with successful 204/2XX response contracts; the reviewer independently
re-probed all five. A further ownership finding required explicit response media
to precede wildcard fallback. A sixth Source case exercises JSON 201 metadata;
201/202 JSON ownership controls reject non-object acknowledgements while existing
Drive wildcard-only binary cases still pass. The reviewer independently re-probed
all three fixes and passed scoped race checks. Verdict: **limited exact-commit
pass**. The reviewer approved `21a9b50bea4e65fe5e28d5dc534613760bd4db5c` after
[CI 37824395168](https://github.com/portpowered/go-icloud/actions/runs/37824395168)
passed. PR #13 merged as `055eeb574b6597d5184d14eeb37c2c360d650def`; main passed
[CI 37825768538](https://github.com/portpowered/go-icloud/actions/runs/37825768538). These are schema/model checks,
not Find My SDK semantic replay; public operations, monitor lifecycle, complete
runtime/source gates and release remain open. No live device command was sent.

`make lint` and `make check` pass for this final tree, including all 67 Python
tests, Go race/contracts/drift/consumer checks and the 80% replay/combined gates.
Current handwritten coverage is 1125/1351 replay statements (83.3%), 696/1351
unit (51.5%) and 1193/1351 combined (88.3%). Cancellation paths produce small
run-to-run statement variations; these are diagnostic measurements, not changes
to the supported operation inventory (LIB-07). The global HTTP inventory is
455 scenarios/1028 pairs; Find My SDK semantic replay remains 0/36.
Fresh Source coverage over the final inventory remains 576/905 entered functions,
3195/5385 body statements and 973/2076 branch exits. These diagnostic Source
figures do not establish SDK parity or full migration acceptance.

### Interim Find My transport audit

Seven internal transport operations now execute 37 portable Find My scenario
streams/70 paired exchanges. Exact responses and typed discovery/token projections
are bound. The driver follows recorded route sequencing; it does not claim
production session orchestration or public SDK semantics. SDK parity remains
0/37; cache, family polling, capability/missing-token guards and monitor ownership
remain required work. Global HTTP inventory is 456 scenarios/1030 pairs.

The independent reviewer found sorted opaque refresh-context keys broke strict
Source wire matching. Schema-owned ordered raw context, a top-level theftLoss
rewrite and a new Source/Go reverse-order regression address the finding. Context
input ownership, response-byte independence and nested unknown JSON have controls.
No matcher was relaxed or live command performed. Independent Source replay and
scoped race/drift checks re-probed the fix and found no further scoped code blocker.

Final `make lint` and `make check` passed, including all 67 Python tests, Go race,
contracts/drift/consumer checks and the replay/combined coverage gates. Handwritten
coverage is 1219/1457 replay statements (83.7%), 780/1457 unit (53.5%) and
1293/1457 combined (88.7%); cancellation paths retain small run variations (LIB-07).
Fresh Source measurement over 456 HTTP scenarios and the other socket/combined
scenarios remains 576/905 entered functions, 3195/5385 body statements and
973/2076 branch exits. Live Go integration remains pending. Final documentation
and exact-commit approval after CI remain pending; full migration/release
acceptance remains open.

PR14 received exact-commit approval at
`77e72a19cb1a9a5c950ee1c4ed55c40324d3d379` after
[CI 37828401650](https://github.com/portpowered/go-icloud/actions/runs/37828401650)
passed. It merged to `00586b140bb852f18c71e883818496e8dfecfffd`; its branch was
deleted and [main CI 37829084493](https://github.com/portpowered/go-icloud/actions/runs/37829084493)
also passed. The preceding pending-approval note describes its pre-merge state.

### Interim Find My public-session audit

The public SDK now drives all 37 current Find My scenarios and 70 pairs from
scenario operation inputs. It binds full functional snapshots, wait traces,
command acknowledgements, response metadata and error meaning; it does not use
recorded routes to decide production orchestration. Independent public schemas
remain separate from canonical wire models. No live command was performed.

Initial replay checks caught absent userInfo projecting as an empty object and
empty acknowledgement slice comparisons treating zero bytes as unequal. Explicit
null projection matches Source results and byte-content comparisons retain exact
wire fidelity. Focused cancellation checks caught wrapped scheduler cancellation
classified as Transport; explicit wait classification preserves Canceled/Timeout
and original causes. Arbitrary scheduler faults remain Transport, clear unrelated
HTTP evidence, and stop the owned monitor. No matcher was relaxed.

An independent working-tree reviewer found no scoped blocker after re-running
race checks for SDK, replay, contracts, generator and transport. Reviewed controls
include two accounts on one SDK, provider cookie rotation, foreign-domain cookie
refusal, nested snapshot/credential/header ownership, cancellable family waits,
monitor callback reentrancy, provider failure recovery and concurrent current/
queued close. The session guide documents deliberate cache getter and monitor
ownership corrections. Exact-SHA approval after final local checks and CI is
still required before this milestone merges; overall acceptance remains open.

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

PR15 received scoped exact-commit approval at
`75bc5e61a5b25ac792c5584c4993983921af8aa5` after
[CI 37833251608](https://github.com/portpowered/go-icloud/actions/runs/37833251608)
passed, then merged to `2140a28aae9e89e368057f75a021094466418864`. Its branch was
deleted and [main CI 37833871353](https://github.com/portpowered/go-icloud/actions/runs/37833871353)
passed. CI coverage was 1456/1758 replay, 1046/1758 unit and 1566/1758 combined;
the preceding figures were explicitly local measurements. Full acceptance is open.

### Interim Find My getter and timed-loop audit

Seven new public-description scenarios and three timed-loop streams bring Find My
to 47 scenarios/86 pairs and global HTTP to 466/1046. Actual pinned Source getters
and _monitor_thread run under declared offline inputs, then the same functional
artifacts drive DescribeDevice and the actual Go owned monitor. Strict paired
requests, whole state, typed failures, response evidence, cookie updates and
wait/stop ordering are bound. No fixture matcher was relaxed or live command sent.

Permanent Source negative controls reject changed description results, rounded
unknown integers, missing nulls, wrong wait durations/completion eligibility,
unused events, premature stop, changed post-tick cache and false failure flags.
Independent review and final measurements are pending for this follow-up. The
behavior guide preserves native-auth/reauth and remaining functional/release gaps.

Independent review found the Go monitor driver could finish without a terminal
stop depending on scheduling, and independently replayed an infinite Source
terminal timestamp successfully. Source/Go adapters now prevalidate a nonempty
trace, finite time values, positive intervals and exactly one final stop;
permanent negative cases include missing and repeated stops plus infinity/NaN.
Both independent mutation probes now reject and focused race checks pass.
The reviewer also requested precise cookie evidence wording. Portable Source/Go
outcomes now inspect the cookie projection through Authentication at every tick,
including a changed cookie in the last failed reply; a mutated final cookie is
rejected. This closes a gap that subsequent-request checks alone could not prove.

Final local checks pass: make lint/check, Go race/contracts/regeneration/consumer
checks and 69 Python test methods, with the final cookie cases additionally
re-probed through focused Source and Go checks. Handwritten replay coverage is
1518/1805 statements (84.1%), unit 1085/1805 (60.1%) and combined 1608/1805 (89.1%).
These are separate diagnostic measurements (LIB-07); cancellation paths can vary
by a few statements. Live Go integration and full migration acceptance remain open.

Fresh synthetic-only Source coverage over the final artifacts is 575/905 entered
functions, 3149/5385 body statements (58.48%) and 964/2076 branch exits. Find My
enters 35/39 functions, covers 148/171 body statements and 51/68 branch exits.
The actual monitor now covers 8/8 body statements and 3/4 branch exits. Four
unentered functions are manager/device display __str__/__repr__ helpers. Remaining
selected branches include forced reauth, stopped-monitor implicit getter refresh,
integer selection and the timer boundary/skew path; generic bad caller inputs
remain outside the requested focus. Counts do not establish full acceptance.
