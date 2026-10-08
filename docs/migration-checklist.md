# iCloud migration checklist

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

The portable HTTP suite now has 281 scenarios and 565 paired exchanges after
adding selected authentication setup, verification, consent and logout behavior.
Authentication socket replay adds 53 synthetic ordered duplex scenarios
(283 events). Combined replay now enters 481/905 functions and executes
2,615/5,385 body statements (48.56%) and 794/2,076 branch exits. These measurements
remain diagnostic; they are not a complete endpoint acceptance gate.
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
Combined replay currently enters 176/915 functions and executes 804/5,436
function-body statements (14.79%). The report also gives captured and synthetic
coverage separately; overlapping statements are counted once in combined
coverage. This remains an early baseline, far from functional completeness.

Subsequent Find My and Reminders cases bring the portable suite to 45 scenarios
and 53 paired exchanges. Combined coverage enters 202/915 functions and executes
909/5,436 function-body statements (16.72%), with 231/2,094 branch exits covered.
Offline checks include 24 test methods and assert monitor cleanup and error
meaning. The independent interim review covers the earlier committed 23 cases;
later additions still need review.

The current portable suite has 65 scenarios and 90 exchanges after adding Drive
transfers and Photos initialization/synchronization. Combined replay enters
221/915 functions and executes 1,005/5,436 body statements (18.49%) and 267/2,094
branch exits. There are 29 offline test methods, including UUID and multipart
negative controls. The draft [wire inventory](reference-endpoints.json) lists
76 HTTP/socket boundaries with resolved pinned source definitions. It still
requires actual send-site/route/payload/dependency audit and schema bindings;
it is not a complete endpoint gate or proof of endpoint coverage.
