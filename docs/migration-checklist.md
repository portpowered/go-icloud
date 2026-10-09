# iCloud migration checklist

The reminder change-iteration matrix contains 72 Source-executed scenarios and
75 paired exchanges. It binds optional cursors, pagination,
multiple zones, ordered duplicate events, unrelated records, tombstones,
full deleted reminders, provider failures and invalid replies. Negative controls
reject changed events, order, requests, unused pages and structured error payloads.
The Source harness now binds payloads for all 54 Reminders API error scenarios,
including null payloads. Portable HTTP inventory is 668 scenarios and 1,375 pairs.
The initial Go change-iteration port passed the original 25 cases, but independent
review found ambiguous record/error selection did not match Source's smart union.
47 additional Source cases distinguish preferred alternatives, ties, nested
metadata, boolean coercion and encrypted-field validity. Independent review found
missing nested identities and valid metadata coercions omitted by the initial
matrix. Go now validates nested required identities, coerces metadata, preserves
arbitrary participant integer magnitude and reproduces audit-date microsecond
rounding and the pinned Windows runtime's date range. Independent review, full
verification and exact-commit CI approval
remain required before merging this public SDK milestone.
The preceding all-evidence Source run enters 592/905 inventoried functions and
covers 3,339/5,398 body statements (61.86%) and 1,031/2,076 branch exits. This
measurement covers the preceding 70-case matrix; the two explicit null-container
cases were added afterward. Coverage remains a conservative scope diagnostic.

The sync-cursor reference matrix now has 53 Source-executed scenarios and 85
paired exchanges. Cases cover token defaults, query decode fallback,
provider errors, pagination, multiple zones, missing final tokens and validation
of every known CloudKit field tag alongside future tags and nested metadata. Portable
HTTP inventory at the sync-cursor milestone was 600 scenarios and 1,304 paired exchanges. This is reference
evidence for the public Go `GetReminderSyncCursor` port, which passes all 53
paired semantic replays with exact tokens, ordered response metadata and prior
response evidence on failures. Public change iteration remains open. The latest
preceding all-evidence Source measurement enters 592/905 inventoried functions and covers
3,326/5,398 body statements (61.62%) and 1,024/2,076 branch exits. The preceding
synthetic-only run enters 584 functions and covers 3,278 body statements and
1,011 branch exits; these evidence categories remain separate.

The preceding Go lookup milestone: `GetReminder` passes 19 paired Source/Go cases binding all
21 public fields, versioned title/notes, ordered related IDs, field dates, audit
fallback after sentinel normalization, unreadable/empty documents, invalid UTF-8,
raw/prefixed identifiers, unrelated records, alternate success/cookies, missing
records and provider/schema failures. The portable corpus now contains 549
scenarios and 1,222 exchanges. Missing-field Source controls remain in place.
This scoped lookup port does not complete the remaining Reminders operations.

The preceding merged Go milestone implements reminder zone and list reads in
the SDK and separate CLI. The CLI passes 34 complete semantic replays against
the published SDK dependency, with 275/335 handwritten statements covered by
replay (82.1%) and 282/335 combined (84.2%). PR32 passed exact-commit CI and
independent review before merging. The last Source measurement, before the new
lookup cases, entered 592/905 functions and covered 3,293/5,398 body statements
(61.0%) and 1,012/2,076 branch exits. Public Go Photos semantic parity, remaining
Reminders operations, native login and full template acceptance remain open.

Objective: cover every relevant endpoint/function in Photos, Find My/devices,
Drive, account, and reminders; establish portable reference scenarios; implement
the equivalent Go library and CLI; publish, verify, and iterate until two fresh
independent reviewers pass every template criterion at the released commit.
Contacts, calendar, Notes, Hide My Email, and Invites are outside the requested
scope. Unavailable account data and destructive live operations remain in scope
for synthetic verification; they are not grounds to omit supported endpoints.

## Work stages

- [ ] Complete the reference function/endpoint/model/socket inventory, with
  explicit scope and evidence classification and a coverage denominator.
- [ ] Exercise relevant functions with observed or labeled synthetic paired
  exchanges. Measure function bodies and branches; document unexercisable paths.
- [ ] Capture authentication socket traffic and prove complete lifecycle replay.
- [ ] Export sanitized portable initial state, exchanges, inputs, results, typed
  failures, volatile bindings, and teardown without Python object dependence.
- [ ] Define responsibility-specific canonical schemas, examples, generation,
  complete wire model inventory, and fail-closed endpoint/model provenance gates.
- [ ] Implement `pkg/icloud`, generated `pkg/dependencymodels`, injected transport,
  explicit account sessions, options, caller-visible token exchange and errors.
- [ ] Port every reference scenario into Go replay and verify semantic parity;
  enforce non-generated SDK/transport coverage (80% minimum, target 90%).
- [ ] Implement and verify the separate installable SDK-consuming Go CLI module.
- [ ] Publish and inspect MDX guides, generated references, whole-site links,
  consumer examples, coverage/badges, and CLI installation instructions.
- [ ] Run blocking all-module lint, race/build/tests, generation/source/model
  gates, module tidy/formatting, consumer checks, and exact-commit CI.
- [ ] Resolve both independent reviewers' findings against all numbered rules.
- [ ] Merge and publish coordinated SDK/CLI module tags; verify proxy downloads,
  consumer builds, release gates, Pages, and both final reviewed commit verdicts.

## Template standards acceptance

All items remain open until proved for the final library, not the bootstrap.
See [independent review](independent-review.md) for findings and verification.

| Standard | Required acceptance evidence | State |
| --- | --- | --- |
| 1 | Consumer-independent SDK/examples/docs; adapters in backend | Open |
| 2 | Documented exported API; compiling examples; schema-valid sanitized examples | Open |
| 3 | Live Go/CI/coverage/release/reference/license/docs badges | Open |
| 4 | Complete routes/models/sockets; generated definitions; gates/negative tests; rendered reference | Open |
| 5 | Blocking pinned all-linters and exact-commit CI verified independently | Open |
| 6 | Functional scenario matrix; non-generated package coverage gate | Open |
| 7 | Required package boundaries; responsibility-specific generated models; consumer imports | Open |
| 8 | Explicit functional options, defaults, and validation | Open |
| 9 | Stateless reusable SDK; account isolation and explicit session lifecycle | Open |
| 10 | Injection and framed replay at every HTTP/socket network edge | Open |
| 11 | Explicit token exchange/refresh returned to caller | Open |
| 12 | Published MDX guides and whole-site link/content verification | Open |
| 13 | Complete documentation audience/duplication audit and accurate claims | Open |
| 14 | Two independent full audits with fixed findings at named final commit | Open |
| 15 | Portable paired replay with strict match/bindings, failures, and teardown | Open |
| 16 | Separate published SDK-consuming CLI; guide workflow, controls/logout, offline tests | Open |

## Current coverage baseline

The owner clarified that endpoint behavior for the selected five services and
required authentication is the priority. Further generic malformed-input and
unrelated utility tests are deferred. The coverage scope now excludes ten named
functions: five excluded service facades, four reference-CLI password/keyring
helpers, and the calendar-only case-conversion helper. Each exclusion resolves
against the pinned source and is listed with its definition line; unknown or
duplicate exclusions fail. Authentication's keyring lookup and shared helpers
used by selected endpoints remain included. The remaining scope is conservative
diagnostic coverage, not proof that every included utility needs endpoint replay.

The Go [HTTP occurrence audit](reference-endpoint-coverage.md) maps the portable
suite to 74/75 draft HTTP routes with no unmatched exchanges. Physical security-key
verification is the sole missing occurrence and is deferred; strict mode fails
on that gap. Diagnostic mode runs
in CI. Occurrences do not replace functional matrices, actual send provenance,
schema/model bindings or separate socket acceptance.

The first [Go paired transport](../tests/replay/README.md) now consumes exact-body
portable exchanges through `http.RoundTripper`, including sticky mismatches,
ordered queries, framing, repeated response headers and body ownership. This is
verification infrastructure, not an SDK operation port. Explicit JSON pattern
and credential-redaction rules now instantiate all 17 portable declarations;
negative controls reject changed fixed fields, string formats, code types and
ambiguous paths. Multipart rules now bind ordered headers/bytes, filenames and
strict boundary framing. All 490 scenarios/1100 exchanges instantiate and fifteen
upload pairs match Go's writer. These are matcher checks, not SDK semantic parity.
Socket/auth timelines, the remaining scenario projections and SDK operations
remain open. No Python semantic replay is counted as passing Go SDK replay.

The public SDK now implements the five account reads, with all 34 portable
account scenarios/40 exchanges executed through public methods and independent
schema-generated projections. Family/photo flows derive IDs from public results.
Two-account replay checks concurrency and repeated fresh requests without shared
cookies. An independent consumer module compiles. The account milestone measured
81.6% replay, 82.5% unit and 92.2% combined. With eleven Drive service methods
and the session/entry lifecycle, current handwritten SDK and internal transport
coverage is 1219/1457 statements (83.7%) replay, 780/1457 (53.5%) unit and
1293/1457 (88.7%) combined; these figures do not
represent the complete service port (LIB-07).

The portable HTTP suite now has 490 scenarios and 1100 paired exchanges after
adding Drive guards and upload/refresh behavior alongside selected
authentication setup, verification, consent, logout and SRP
sign-in behavior. SRP cases run both password protocols with declared client
entropy and exact proof matching; they also exercise refusal, trust-token reuse,
paused MFA token login and SMS challenge setup. Thirty combined bridge cases
contain 116 HTTP pairs and 297 socket events, with shared consumed timelines,
actual bootstrap signing/framing and modern SPAKE2/AES-GCM verification, nonce
retry, prompt setup, SMS fallback and legacy code verification. Three cases
exercise complete SRP-to-bridge login, including modern rejection without trust
or account setup. Physical security-key interaction is deferred; live socket
evidence, Go interoperability and final SDK/release gates remain open.
Authentication socket replay adds 53 synthetic ordered duplex scenarios
(283 events), separate from combined bridge transcripts. Combined replay now
enters 576/905 functions and executes 3,195/5,385 body statements (59.33%) and
973/2,076 branch exits. These measurements
remain diagnostic; they are not a complete endpoint acceptance gate.
Drive now has 79 scenarios and 139 pairs, including navigation/cache/refresh,
node-backed endpoints, empty downloads, offset uploads and provider refusal at
each transfer stage. Errors bind file position, token parameters, node state and
exposed response context. Its pinned date parser mishandles negative offsets
with nonzero minutes. Computed Go entry properties preserve this recorded
reference arithmetic for compatibility; raw provider timestamps retain their
offsets and support standard UTC conversion (SCHEMA-09, LIB-05).
These cases are synthetic reference behavior, not observed Apple writes.
Eight added node-upload/facade cases close measured function-entry gaps for
`DriveNode.upload` and `DriveService.__getattr__`. They bind the received node's
document ID/zone, file cursor, empty/binary content and all three upload-stage
refusals. Owned instrumentation files close on success and failure. New negative
controls reject changed cursor, zone, node state and unconsumed traffic.
The [Drive wire contracts](drive-wire-contracts.md) now bind all 139 exchanges to
13 operation contracts, with generated canonical models and internal clients.
Contract validation, drift checks and negative controls pass. Drive client/SDK
semantic replay now covers all 79/79 scenarios: nine reads, sixteen mutations,
thirteen downloads, nine service uploads and thirty-two node flows. Explicit
Drive sessions own cache, cookies, tokens and cancellation; concurrent tenants,
copied snapshots and close/current/queued work have race controls. Native
authentication persistence and the full source/schema runtime gate remain open;
schema-only contract checks
do not increase the SDK coverage numerator.
Find My has 71 scenarios and 140 pairs after adding explicit refresh, family
readiness/progress/retry bounds and provider refusal at command/token stages.
The [Find My wire contracts](findmy-wire-contracts.md) bind these exchanges to
seven operations, validate payloads/query parameters and round-trip each JSON reply
through generated canonical models. Regeneration and protocol-constant drift
checks pass. The [internal transport driver](findmy-transport.md) also executes all 140 pairs,
including a Source-verified reverse-order refresh context. The
[public Find My session](findmy-session.md) executes all 71/71 scenarios through
140 paired exchanges with full functional snapshots, command acknowledgements,
capability/token guards and bounded family waits. Independent race controls cover
owned monitoring, close, callback reentrancy, copied state and concurrent account
cookie rotation. Seven new device-description scenarios exercise selected Source
getters and Go cached projections; three timed-loop cases bind Source and Go
monitor recovery, partial replies and cookie rotation after repeated failures.
Twenty-four additional command-reply cases bind binary/malformed JSON, arrays,
scalars and HTTP-200 JSON provider failures; all 38 command replies also pass
through generated byte-preserving parsers. Native authentication and forced-reauth
remain pending.
Declared waits are consumed offline; received device/user/server state and
exposed errors are bound. A source monitor-replacement race is documented for
Go correction. Partial refreshes retain cached devices rather than infer removal
without provider deletion evidence. Local checks now include 70 offline test methods.
The latest 40 scenarios add 145 pairs for private/shared Photos container changes,
shared lookup/library behavior and Reminders zones, including provider refusal
and provider payload errors. Photos exceptions bind optional photo and album
resources. Source inspection excludes dormant Reminders database changes and
includes the active shared batch-count route. Shared favorites use the private
mutation client before a shared refresh in the pinned reference; Go must preserve
the selected container and zone. These are synthetic cases, not observed writes.
The preceding shared-Photos/Reminders milestone entered 574/905 functions and
covered 3,188/5,385 statements (59.20%) and 968/2,076 branch exits.
The preceding Find My milestone entered 572/905 functions and covered
3,165/5,385 statements (58.77%) and 959/2,076 branch exits.
The preceding Drive milestone entered 569/905 functions and covered
3,145/5,385 statements (58.40%) and 949/2,076 branch exits.
The preceding complete-login milestone entered 547/905 functions and covered
3,079/5,385 statements (57.18%) and 917/2,076 branch exits.
The preceding combined-bootstrap milestone entered 506/905 functions and
covered 2,842/5,385 statements (52.78%) and 851/2,076 branch exits.
The preceding SRP milestone entered 488/905 functions and covered
2,718/5,385 statements (50.47%) and 822/2,076 branch exits.
The preceding authentication session milestone entered 481/905 functions and
covered 2,615/5,385 statements (48.56%) and 794/2,076 branch exits.
The preceding hydration/account milestone entered 445/905 functions and covered
2,320/5,385 statements (43.08%) and 697/2,076 branch exits.
The preceding upload/shared-stream milestone entered 433/905 functions and
covered 2,249/5,385 statements (41.76%) and 670/2,076 branch exits.
The preceding Photos album/asset milestone entered 376/905 functions and covered
1,995/5,385 statements (37.05%) and 597/2,076 branch exits.
The preceding Reminders milestone entered 331/905 functions and covered
1,792/5,385 statements (33.28%) and 519/2,076 branch exits.
The earlier bridge-message suite entered 252/905 functions and covered
1,290/5,385 statements (23.96%).
Before the scope correction, the same replay covered 1,290/5,436 statements
(23.73%); the scope correction removed 51 unrelated statements, not missing
endpoint cases.
The socket cases execute the actual pinned raw client at an injected plaintext
TCP/TLS boundary;
they exercise push-token, subscription, acknowledgment and push decoding, but
do not establish full bridge login or complete protobuf/cryptographic coverage.
Failed-upgrade cases show no explicit socket-close call; Go must close on failure.
See the [socket transcript contract](../tests/replay/fixtures/synthetic/socket/README.md).
Historical measurements below are retained as progress receipts.

The reference coverage command uses pinned coverage.py with statement/branch
measurement, per-scenario contexts, and an AST inventory of function bodies.
It does not count a function definition being imported as a function call.
Included but unexecuted source files remain in the denominator. Generated
protobuf exclusions are explicit in `tools/reference/coverage-scope.json`.

Initial private read replay baseline: 915 inventoried functions; 165 entered;
754/5,436 function-body statements (13.87%); 179/2,094 branch exits. This is a
conservative preliminary scope inventory, not proof of complete reachability or
endpoint coverage. Audit legacy facades and shared helpers before classifying
unreachable/out-of-scope functions. The uncovered list drives further scenarios.

Reports stay in `%LOCALAPPDATA%\go-icloud\reference\coverage` because HTML embeds
source and measured paths. `function-coverage.json` lists missing function lines
and candidate network call sites; AST network discovery is explicitly not a
complete schema/provenance gate. Raw private read replay still has 17 scenarios
and 95 exchanges; this receipt is not the functional coverage denominator.

The first portable synthetic suite adds 23 scenarios and 23 paired exchanges
for Drive and account, with service constructors and semantic error checks.
At that initial milestone, combined replay entered 176/915 functions and executed 804/5,436
function-body statements (14.79%). The report also gives captured and synthetic
coverage separately; overlapping statements are counted once in combined
coverage. This remains an early baseline, far from functional completeness.

Subsequent Find My and Reminders cases bring the portable suite to 45 scenarios
and 53 paired exchanges. Combined coverage enters 202/915 functions and executes
909/5,436 function-body statements (16.72%), with 231/2,094 branch exits covered.
Offline checks include 24 test methods and assert monitor cleanup and error
meaning. The independent interim review covers the earlier committed 23 cases;
later additions still need review.

The subsequent portable suite reached 65 scenarios and 90 exchanges after adding Drive
transfers and Photos initialization/synchronization. Combined replay enters
221/915 functions and executes 1,005/5,436 body statements (18.49%) and 267/2,094
branch exits. There are 29 offline test methods, including UUID and multipart
negative controls. The draft [wire inventory](reference-endpoints.json) lists
76 HTTP/socket boundaries with resolved pinned source definitions. It still
requires actual send-site/route/payload/dependency audit and schema bindings;
it is not a complete endpoint gate or proof of endpoint coverage.


The [Reminders wire layer](reminders-wire-contracts.md) now binds all 78 existing
scenarios/86 paired exchanges to six operations. Shared CloudKit models include
all 28 source CKRecord fields and normal-record response union members omitted
by automatic source schema export. Generated-model roundtrips, source-invalid
zone rejection, ten examples, drift/constant checks, consumer imports and limited
asset URL-issuer controls pass focused checks. SDK/domain/protobuf orchestration
and full runtime provenance remain open; wire checks do not increase semantic
Go replay coverage.

### Reminders document decoding

- Pinned both active source `.proto` files and reproducible Go generation.
- Ported zlib/gzip/raw current/legacy/bare decoding against 52 portable Source cases.
- Preserved raw decoded bytes and typed failures; no live operation was performed.
- Public domain mapping, CRDT writes and full service replay acceptance remain open.

### Initial Go CLI read milestone

The separate CLI module consumes the publicly resolved SDK version for main
8104b8ee8340. Seven Account/Drive/Find My read commands accept a private explicit
AuthContext file. Canonical paired synthetic replays bind requests and projected
results, with an additional explicitly synthetic 401 control. Native login,
reference-session import, Photos/Reminders commands, writes, consumer installation,
CLI module release and live verification remain open. Item 16 is not complete.

### Saved-session SDK authentication

`ResumeSession` now validates cookie state and refreshes saved tokens using the
public saved-credential request model. Thirteen canonical success cases and three
canonical failures bind source discovery, trust/MFA flags, service URLs, ordered
response metadata and credential rotation. Two new success-status controls were
executed against the pinned Source; HTTP inventory is now 497 scenarios/1109
pairs. Two further Source-executed cases bind empty auth headers to preserved
saved token/trust/country values, including subsequent refresh request inputs. An explicit synthetic control proves omitted cookie scope is normalized,
rotated credentials replace stale values and remain off another service origin.
Password/SRP, MFA completion, accepted terms, CLI import, live verification and
full authentication acceptance remain open.

Native cookie fidelity now has three further Source-executed controls: embedded
quotes in saved cookies, quoted response rotation preserved in the returned
credentials and next refresh request, and explicit Cookie-header precedence.
Cookie pair/separator/quote formats are schema-generated. A private live Go
saved-session probe returned provider 421; this is a failed authentication
attempt, not verification of live service behavior or valid saved credentials.

### Photos download failure evidence

Ten additional Source-executed synthetic download scenarios add forty paired
exchanges, bringing the portable HTTP inventory to 507 scenarios/1149 pairs.
Provider refusals cover 401, 403, 404, 410, 429, 500, and 503. Transport cases
inject timeout, connection failure, and chunked-transfer errors. Six negative
controls reject changed expected errors, wrong asset origins, and unused pairs.
The Go transport instantiates all pairs; public Go Photos service orchestration
and semantic result parity remain open. This milestone does not complete the
reference endpoint/function inventory or any release gate.

### Recently Added pagination evidence

Six additional Source-executed scenarios add 27 pairs, bringing the portable
HTTP inventory to 513 scenarios/1176 pairs. They use the public root library and
default 100-item windows, preserve newest-first ordering, remove overlapping
assets, terminate on partial/empty/duplicate-only pages, and avoid count calls.
Five negative controls reject altered ranks, result ordering, duplicate results,
and unused exchanges. The fresh reference diagnostic enters 583/905 functions
and covers 3234/5398 function-body statements (59.91%) and 993/2076 branch exits.
The Go transport instantiates the pairs; public Go Photos parity and full
migration/release acceptance remain open.
