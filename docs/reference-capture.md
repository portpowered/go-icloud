# Reference capture development

This is the first migration stage, not a completed library or release. The
layout, Go verification tools, all-linters configuration, and contributor
standards come from `go-third-party-template`. The reusable Go client, generated
provider schemas, public CLI, docs website, and consumer adapter follow after
reference evidence has been collected. No widget example is presented as an
iCloud API. Library standards 6 and 15 and LIB-05/LIB-12 require evidence-driven
paired replay; LIB-13 keeps the repository verification tools in Go. Python
instrumentation here executes the external Python reference, not the Go SDK.

## Source and runtime

`tools/reference/source.json` pins both references. The original
[picklepete Photos implementation](https://github.com/picklepete/pyicloud/blob/622cd160d8a259db644c8b3f3c96f76795a12f74/pyicloud/services/photos.py)
is the historical baseline. Its password sign-in predates the SRP change;
[PR 459](https://github.com/picklepete/pyicloud/pull/459) proposes the update.
The live CLI uses a pinned [timlaing fork](https://github.com/timlaing/pyicloud/tree/e2e44ab875d47dab4475096021da60030f26c35e)
with SRP authentication and newer Photos/reminders implementations. These are
distinct references; neither source establishes current Apple behavior without
an observed exchange. Every recording carries the live revision.

Setup clones clean, detached checkouts into ignored `.reference/` and installs
locked dependencies into ignored `.venv/`. The CLI rejects a dirty checkout,
revision drift, or importing pyicloud from a different environment. The wrapper
instruments the HTTP adapter before constructing/authenticating the reference,
without changing upstream source. Its license remains MIT; the new repository
uses the template's Apache-2.0 license.

## Private recording format

Each numbered JSON file is `portos.http-exchange.v1`: source URL/revision,
operation, UTC time, sequence, request and response, or a typed transport failure.
Requests contain method, origin, escaped path, ordered repeated query pairs,
headers, and exact base64 body bytes. Responses preserve repeated headers and
decoded HTTP entity bytes in base64. This is HTTP semantic replay, not a packet
capture: compressed transfer/framing bytes are not preserved. Interactive
password/code JSON fields use explicit nonempty/six-digit match rules; original
Content-Length is excluded only for such redacted bodies during replay.

Synthetic compressed-wire controls explicitly set response `bodyRepresentation`
to `wire`. Their base64 bytes enter the real content decoder in both languages.
`decoded` or an omitted representation retains the recorder convention and
prevents double decoding while preserving the original response headers. Replay
rejects every other representation. The HTTP content corpus separately binds
compressed bytes to the pinned requests/urllib3 decoded result or decoding error;
these invented responses are not live iCloud captures.

The 604 paired HTTP content controls cover complete gzip, zlib and raw DEFLATE,
every byte prefix of a dynamic stream, fixed/stored blocks, optional gzip headers,
concatenated members, incomplete trailers and corrupted checksums. Incremental
DEFLATE uses pinned `github.com/dsnet/compress/flate v0.0.1`; the standard Go
inflater loses some complete literals at truncated EOF. No padding bytes are
invented. Go replay compares exact decoded entities and authentication results,
or a transport failure with the recorded HTTP status and preserved native cause
(GO-07, LIB-05). It does not reproduce Python exception message text. Python
replay independently verifies the pinned requests/urllib3 result or full error
against the same portable pairs (LIB-12). The corpus measures HTTP decoding
compatibility; it does not establish live account access or full service coverage.

Tokens, cookies, account identifiers, locations, filenames, and response data
remain in the **private** recording. Passwords and entered codes are never
stored. The recorder does not automatically sanitize arbitrary personal data.
Do not copy these files into the repository. Reviewed public fixtures will use
`tests/replay/fixtures/captured/`; labeled artificial cases belong in
`tests/replay/fixtures/synthetic/` (SCHEMA-12, LIB-04).

`result.json` holds selected reference results using plain JSON, not Python
objects. It accompanies the ordered exchanges for later semantic comparison.
Failed commands retain any exchanges already observed. Errors print only their
type, avoiding upstream messages that may contain personal data.

The strict `ReplayAdapter` validates full requests
before returning a response, rejects mismatches/duplicates, asserts consumption,
and cannot access the network. New read captures include a private snapshot of
the starting cookie/session files and account selection. `replay.py` copies them
to a temporary directory, freezes cookie-expiry evaluation at capture time,
installs the replay adapter, rejects fallback HTTP traffic, and compares the
reference's JSON result or error type. It never modifies the live saved session.
Find My's polling thread is stopped and joined before the HTTP session closes.

Replay all completed private read captures locally:

```powershell
.venv/Scripts/python.exe tools/reference/replay.py --all
```

For one capture, supply its directory instead of `--all`. Live captures are not
part of ordinary CI. Offline checks include the portable synthetic JSON cases
under `tests/replay/fixtures/synthetic/http`, as well as temporary seam controls.
Replay blocks default HTTP adapters, TCP connect/connect_ex, and socket connection
factories, including attempts by dependencies outside the injected HTTP session.

Run `make reference-coverage` to measure reference function bodies and branches
under private read replays and portable synthetic scenarios. Reports separate
captured and synthetic coverage and also show their combined coverage. Run
`tools/reference/measure.py --synthetic-only` for measurement without an account.
The pinned coverage tool writes private JSON and HTML
reports; its scope and generated-code exclusions are explicit. The current
[migration checklist](migration-checklist.md) records the uncovered baseline
and [independent review](independent-review.md) findings. Passing replay does not
mean all endpoints/functions have been exercised.
Run `make endpoint-coverage` for the Go
[portable HTTP occurrence audit](reference-endpoint-coverage.md). It lists
missing and unmatched routes and has an optional failing strict mode; diagnostic
success does not establish endpoint completeness.
The draft [wire inventory](reference-endpoints.json) records known HTTP/socket
boundaries and their pinned source definitions. Schema bindings, actual send
provenance, payload variants, dependency edges, and complete scenario matrices
remain open; function-body coverage does not prove those requirements.
Raw private snapshots are not publishable fixtures: portable sanitation and
volatile token/ID bindings still need review. Initial login cannot yet replay
because its trusted-device bridge includes uncaptured socket traffic.

## Known gaps and next steps

- Live login/code entry was completed by the account owner, followed by observed
  trusted-session reuse and reads. See [operation evidence](operation-matrix.md)
  for successful and failing scenarios. Security-key and legacy two-step login
  are not implemented by this capture CLI.
- The fork can use a trusted-device bridge with separate TLS/socket traffic.
  The HTTP recorder does **not** capture those binary frames. Inventory and
  instrument that edge before claiming full authentication replay coverage.
- Response-only fixtures from the backend are existing test inputs, not proven
  current captures. Audit provenance before importing them.
- Build a matrix for each operation with observed success, zero/one/many,
  pagination, missing/malformed data, expiry, throttling, provider/transport
  failure, and initialization behavior. Mark unobserved cases and add labeled
  synthetic failure cases. Broader read and download coverage follows the first
  account exploration; writes require a separate explicit workflow.
- Establish offline reference scenarios, then reuse the reviewed artifacts for
  generated-schema Go implementation and semantic result comparisons. Complete
  the template checklist and independent reviews before migration sign-off.

## Verification

Reference coverage disables coverage.py's default line and partial-branch
exclusions, including upstream `no cover`, `no branch`, and `TYPE_CHECKING`
markers. Named file/function exclusions remain in the audited scope policy.
The report records the effective exclusion patterns; a never-matching partial
pattern prevents coverage.py 7.16.2 from interpreting an empty pattern as a
match for every branch. Negative controls verify missing statements and branch
exits remain visible.

The fresh synthetic-only measurement enters 578/905 functions and covers
3195/5398 function-body statements (59.19%) and 977/2076 branch exits. This is a
diagnostic baseline, not endpoint completeness or release acceptance. The
measurement runs the canonical HTTP/socket and Reminders text corpora plus
both reference saved-login corpora. The latter use separate `synthetic:local:`
contexts and the same strict replay runner as the reference tests: four initial
resume/read flows consume eight exchanges, and four restore/resume/read/re-resume
flows consume twelve. They restore global and China login files and compare
authentication state, cookies, headers, and empty or multiple-device results.
Negative controls reject changed state, mismatched requests, and unused pairs
(LIB-05, LIB-07). The 604 dependency-only HTTP content controls remain separate
from this source-function receipt.

Import-time execution uses `setup:reference-import` and is reported under
`by_evidence.setup`, alongside separate captured and synthetic results. The
combined baseline above includes setup; it is not a live-capture coverage figure.

Account reads use the `webservices.account.url` returned by authentication.
The setup origin used for login is a separate service and is not a substitute
when account discovery is absent. Four implementation-derived paired flows
cover restoring global and China reference login files, authenticating, and
reading empty or multiple-device Account results at a distinct discovered
origin. The pinned Python reference and the Go SDK consume the same HTTP pairs.
The Go CLI consumes the repeated-resume corpus through its public SDK dependency.
These offline flows do not establish successful live account access. The CLI
supports saved-login import and resume; native password login and MFA completion
remain pending.

After setup, run `make lint` and `make check`. They run the template's blocking
Go lint/build/race checks over migration tools and Python lint/offline capture
tests. They do not contact Apple. The Python tests cover recorder interception,
redaction, strict matching, transport failure, private storage, CLI login/session
reuse, command selection, and reference drift. Coverage of Go verification tools
does not represent iCloud client coverage.
