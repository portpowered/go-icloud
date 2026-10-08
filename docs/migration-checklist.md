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
