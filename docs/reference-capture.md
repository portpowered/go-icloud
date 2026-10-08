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

After setup, run `make lint` and `make check`. They run the template's blocking
Go lint/build/race checks over migration tools and Python lint/offline capture
tests. They do not contact Apple. The Python tests cover recorder interception,
redaction, strict matching, transport failure, private storage, CLI login/session
reuse, command selection, and reference drift. Coverage of Go verification tools
does not represent iCloud client coverage.
