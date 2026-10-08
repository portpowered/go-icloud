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

Reviewed commit, complete wire-model/endpoint inventories, CI run URLs, release
tags, proxy verification, Pages inspection, and reviewer verification of each
fixed finding remain to be recorded. Every unresolved finding prevents sign-off.
