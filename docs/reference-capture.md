# Reference capture development

Twenty-eight synthetic snapshot facade cases execute the pinned Source's public
`reminders(list_id=None)` operation. The 45 paired exchanges cover explicit list
filters, discovery of zero/one/multiple lists, list and query pagination, empty
filters triggering discovery, duplicate replacement across lists, and failures
during discovery or query. The facade requests completed reminders with page
size 200. Complete results and structured errors are Source-derived; negative
controls reject altered inputs, completion filters, results, replacement values
and unused traffic (LIB-05/LIB-12). The added cases cover overlapping normal,
error and tombstone models and cookie reuse from discovery into the query.
Thirteen corresponding list-discovery cases bind Source selection and error precedence
to the public Go list reader, alongside its original twenty-four cases.
The current portable HTTP corpus contains 740 scenarios and 1,465 pairs, including
302 Reminders scenarios and 363 pairs. The Go snapshot facade executes all
twenty-eight Source cases; full verification and independent review remain open.
These cases do not establish live provider behavior or complete Reminders coverage.

The compound reminder reference matrix contains thirty-one scenarios and thirty-two
paired exchanges. It covers all supported related records, defaults, populated
fields, orphan filtering, unsupported types, duplicate replacement across pages,
assets, byte-backed text, recurrence selection and validation failures. Complete
results come from the pinned Source; negative controls reject changed relationships,
omitted results, stale replacement values, changed inputs and unused traffic.

Six wrapper-sensitive cases bind BYTES and ENCRYPTED_BYTES discriminator and URL
behavior. The Source skips byte-backed attachment and trigger types because it
compares their bytes to strings before model coercion. Byte-backed URL fields
retain their decoded text, even when that text contains a base64-encoded URL;
STRING URLs can undergo a further base64 decoding step. This is observable
reference behavior that the Go projection must preserve (LIB-05/LIB-12).
Two further cases preserve STRING base64 URLs containing LF or CR: the strict
Source URL decoder rejects those line breaks, while byte-field decoding permits
them. The two decoding policies must remain distinct.

STRING frequency `"2"` and DOUBLE frequency `2.9` fall back to daily. Asset size
accepts integer-coercible text for ASSET and ASSETID; its schema preserves integer
magnitude. Byte-backed ordinary text is UTF-8 decoded, while hashtag text replaces
invalid UTF-8. Non-strict base64 inputs, tombstones and invalid orphan images also
have reference cases. These are offline cases; Go compound parity remains open.
At the compound milestone the portable HTTP corpus contained 699 scenarios and
1,407 exchanges.
Before the six wrapper-sensitive additions, synthetic-only coverage measured
584/905 entered functions, 3,336/5,398 body statements and 1,045/2,076 branch exits.
These fixed denominators include remaining selected-service work (LIB-07).

Three synthetic Photos record-pairing cases cover a master without an asset,
an asset without its master, and a mixed response containing both orphans and
one valid pair. The pinned reference skips incomplete pairs and preserves the
full valid photo result. All twelve exchanges are consumed; negative controls
reject changed master references, omitted public results, and unused traffic.
The portable HTTP corpus contains 516 scenarios and 1,188 paired exchanges.
These cases describe reference behavior; public Go Photos parity remains open.
The fresh synthetic-only diagnostic measures 583/905 entered functions,
3,235/5,398 function-body statements (59.93%), and 994/2,076 branch exits. Scope
and denominators are unchanged; this closes the reachable incomplete-master
skip branch without excluding uncovered code.

The public Go `ListReminderZones` method now consumes the five original zone
scenarios, three Source-verified successful status cases (201, 202, 299), and
unknown metadata at the response, zone and identity levels. Large integers,
nulls and nested values retain their full Source meaning.
Another Source-backed scenario binds optional authentication build/mastering
parameters and their position before the client/account identifiers.
The 202 case advertises plain text while carrying valid JSON; JSON decoding is
independent of the media type, as in the reference and external schema's 2XX
contract. The SDK projects every known zone field, normalizes missing nullable
values to the reference's nulls, and preserves status, headers and typed failures.
Authentication carries the discovered reminders URL in caller-owned state.
Reminder listing, mutation methods, CLI commands and live Go verification remain
open; this milestone is zone discovery rather than complete reminders parity.
The combined portable HTTP corpus contains 521 scenarios and 1,193 exchanges.

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

Photos download failure scenarios execute the pinned service's indexing,
album/asset lookup, and asset GET against four paired exchanges per case.
Synthetic refusals cover HTTP 401, 403, 404, 410, 429, 500, and 503; transport
cases inject timeout, connection failure, and a chunked-transfer error.
The reference reports endpoint-gone for 410, connection errors for timeout
and connection failure, and API errors for the remaining cases. These are
implementation-derived outcomes, not observed account failures. Negative
controls reject changed error results, wrong asset origins, and unused pairs
(LIB-05, LIB-12). The fixtures enter the existing Go portable transport replay;
public Go Photos service orchestration and semantic parity remain pending.
Recently Added pagination uses six implementation-derived paired scenarios:
empty, one, many, a full 100-photo window followed by an empty page, overlap
followed by a partial page, and a duplicate-only next page. They discover the
root library through the public reference API and use the default page size.
The Source emits trailing `startRank` windows at 99 and 199, reverses each new
window, removes repeated assets, and stops without calling the count endpoint.
The same 27 HTTP pairs instantiate in Go transport replay. Negative controls
reject changed ranks, wrong result order, duplicate results, and unused pairs
(LIB-05/LIB-12); public Go Photos orchestration remains pending.

Reference coverage disables coverage.py's default line and partial-branch
exclusions, including upstream `no cover`, `no branch`, and `TYPE_CHECKING`
markers. Named file/function exclusions remain in the audited scope policy.
The report records the effective exclusion patterns; a never-matching partial
pattern prevents coverage.py 7.16.2 from interpreting an empty pattern as a
match for every branch. Negative controls verify missing statements and branch
exits remain visible.

The fresh synthetic-only measurement enters 583/905 functions and covers
3235/5398 function-body statements (59.93%) and 994/2076 branch exits. This is a
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
