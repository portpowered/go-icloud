# Verification and fixture guidance

## Local checks

Format changed Go files, then run the checks before a release:

```sh
gofmt -w .
make check
```

`make lint` runs `go vet ./...` and the pinned golangci-lint v2 release from
CI. `.golangci.yml` must keep the literal `linters.default: all`; CI runs every
repository package, treats findings as failures, and does not filter to new
issues. `make check` runs that lint target, builds every package and example,
and runs tests with the race detector. CI also checks formatting and module
metadata on each pull request.

Keep all linters enabled. If a finding requires an exception, scope it to the
specific rule and affected path and explain the reason in a config exclusion or
beside the source annotation. Never set the issue exit code to zero or continue
after a lint failure. A reviewer who did not implement the change confirms the
passing blocking CI result for the exact final commit before checklist item 5
is checked.

CI reports public API changes against the pull request base. The report is
non-blocking so reviewers can decide whether an intentional break is appropriate.
The package list defaults to the module root and httpclient; pass -module and
-packages to tools/compatibility when the module path or public packages change.
Run make api-compatibility to compare against the most recent stable release tag.

For extracted libraries, report coverage for each public and transport package
and for non-generated production code in combination. Exclude generated files
explicitly rather than counting them as uncovered or using generated code to
inflate the percentage. Reach at least 80% combined coverage and target 90%.
Enforce the 80% floor in the extracted library's CI. Use deterministic
synthetic inputs to cover success, provider errors, invalid responses,
cancellation, token rotation, and session lifecycle where applicable.
Review uncovered behavior before adding tests; a percentage alone is not a
behavioral sign-off. Keep live integration results separate from synthetic
unit and replay coverage.

After moving the synthetic scaffold into `pkg/<provider>`, run the reusable
coverage gate from the module root:

```sh
go test -coverpkg=./pkg/... -coverprofile=coverage.out ./pkg/...
go run ./tools/coverage -profile coverage.out -min 80
```

The gate reports each package and a statement-weighted combined percentage.
It excludes generated files marked `Code generated ... DO NOT EDIT` and files
named `*.gen.go`; review the exclusions alongside the result.
Use `-percent-only -min 0` when producing a coverage badge from the same
non-generated measurement. Pass `-filtered-profile coverage.filtered.out` to
write a profile for an HTML report that uses the same exclusions.

## Model audit

Validate each canonical request, response and event example against its owning schema.
Cover every known dispatch variant with complete envelopes and nested payloads, including
resource updates, relation changes, empty acknowledgements and representative errors.
Test identifier/payload correlation: a known command must reject another command's
parameters and malformed known input must not escape through a future-value branch.
Inspect the rendered reference and generated request snippets as well as schema output;
required nested fields and useful example values must be discoverable to a customer.

For interactive authorization, replay browser consent, callback state validation and PKCE
binding, token exchange and provider-required activation calls as one complete flow.
Keep browser, loopback listener and transport injectable; test denial, invalid or duplicate
callback parameters, wrong redirects, cancellation, repeated completion and cleanup.
Assert ordinary output excludes credentials and explicit export is required.

Run source-gate negative controls through the repository's actual CI commands,
including their default working directory and root arguments. Include every
production sibling file in a package, even when it is newly added or carries a
forged generated marker. Test compile-valid sibling helper returns and map mutations;
keep caller-defined open values as positive controls.
Unresolved provenance is not evidence of caller ownership. Recursion, traversal
limits, and unresolved library helpers must produce a failing diagnostic at a
schema-owned wire value or key. Include a helper chain beyond the verifier limit
and a recursive fixed fallback as negative controls, with proven caller-input
positives.

During the model audit, inspect library-built maps and nested payloads encoded
inside strings or wrappers. Known fields, operation values, and message identifiers
need schema-generated definitions; an open object on the outer envelope is not
evidence for concrete payload variants. Keep caller-defined open input separate.
For GraphQL, inspect the emitted operation and custom decoder branches as well
as the SDL: generators may add discriminators that are absent from authored
documents. Record schema possible types with no selected subtype fields accurately.
Negative controls must catch unknown nested payloads or wire keys, missing or
mismatched decoder branches, possible-type drift, and missing discriminators.

Audit primitive constants and open-string known values separately from object types.
Resolve their uses through exact imports, check bindings to shared source enums,
and test removal of a binding as well as a changed value. Bind known JSON-scalar
members to generated types without changing the GraphQL selection. Negative tests
must introduce a new fixed value or key absent from every schema, including a
post-construction mutation and an alias; rejecting only known literal values is
not sufficient. Inspect generated string templates and cookie recognition rules.
Check every key in nested indexed receiver paths, including through reassigned aliases,
pointer dereferences, slices, and type assertions. Include negative probes for intermediate
keys and inferred nested composite literals; preserve caller-defined open keys in positive cases.

## Fixture provenance

Each replay case must carry both sides of the exchange. For HTTP, keep the
expected outbound method, origin, escaped path, complete query multimap,
relevant headers, and request body with the response status, headers, and body.
For streams and sockets, keep ordered client and server frames with payloads.
The replay transport must validate a request before returning its paired
response, fail on unexpected or repeated calls, and assert that all expected
exchanges were consumed. Do not use a sequential or operation-name fallback
that returns a response after a request mismatch. Explicitly describe how
volatile or redacted values are matched; compare their structure or decoded
meaning rather than skipping them. Check this for every transport during the
independent review (library standard 15).
Match full authentication forms and headers, including CSRF, OTP, and token exchanges.
For OAuth, verify state/callback and PKCE challenge/verifier bindings. Validate hardware
identity reuse and volatile formats before replacing values with placeholders. Match full
signaling handshakes and envelopes. Finish lifecycle assertions after the expected close or
teardown; neither a startup notification nor a terminal read timeout proves cleanup.

Keep real captures, synthetic examples, and historical references separate:

- Put observed and sanitized payloads under
  tests/replay/fixtures/captured/.
- Put hand-authored examples and generated test payloads under
  tests/replay/fixtures/synthetic/.
- Put old or superseded documentation under docs/reference/.

Do not call a fixture captured unless its source and collection date are known.
For each captured file, record the operation, capture date in UTC, source
category, redactions, and the behavior the fixture supports in a neighboring
provenance note. Remove access tokens, cookies, personal data, and account or
device identifiers before adding it. Keep credentials out of fixtures and CI.

Mark synthetic fixtures as synthetic in their filename or metadata. A synthetic
fixture can test an intended contract, but it is not evidence that a real service
behaves that way. Historical references are context and do not establish current
behavior.

The template includes guidance directories only; it does not contain captures
or claim any provider behavior has been verified.

## CLI verification

The standalone CLI must exercise the public SDK without a consuming application.
Check help, authentication, read/control commands, event cancellation and cleanup,
JSON output, and failure exit codes with credential-free paired replay tests.
Run lint, build, tests, formatting, and module checks in the CLI's separate module
as blocking CI jobs. Verify the published CLI can be installed from a clean
consumer module; document the commands and explicit credential export in MDX.
Replay the customer guide's ordered workflow: interactive authorization, persisted
account reuse, device enumeration and selection by ID, a useful read, an explicit
control, and logout. The customer must not construct request JSON, copy credentials
between files, or supply default wire parameters. Check invalid authorization,
ambiguous or unknown device selection, profile persistence and removal, secret
redaction, cancellation, and session cleanup. Inspect the rendered guide to confirm
the main instructions match CLI help and the tested command syntax; keep advanced
formats and protocol explanations outside that sequence.

## Independent standards review

Assign two reviewers who did not implement the migration to inspect the library
at the final implementation commit. Each reviewer checks every numbered library standard against
the exported API, schemas and generated files, client call sites, synthetic and
captured evidence, CI workflows and results, package layout, rendered Pages
site, and release/history record. Both reviewers write separate sections in one current review document
with an individual verdict and concrete evidence for each numbered standard,
including an endpoint inventory and negative gate tests for item 4. Link it
from the library checklist and record each finding there. Keep the
independent-review item open until every finding and other checklist item is
resolved. Tracking an open finding is not sign-off. Rerun the affected checks
and have both reviewers verify the fixes at the final commit before marking the
item complete. Keep provider behavior that lacks documented account evidence
labeled implementation-derived.

Include a complete wire-model inventory: schema component, generated Go type and file,
generator command, transport use, and any compatibility alias. Independently search all
production packages for named and anonymous serialization structs, including exported
dependency types and custom encoders or decoders. Prove the model gate rejects an
unreferenced exported handwritten JSON struct and an anonymous nested wire object.
Verify API components are grouped by responsibility in `pkg/dependencymodels`
and no parallel internal model bucket remains. Generated output alone is not a full audit.

For documentation sign-off, inspect every tracked document, including files excluded from
the site build. Record its audience and purpose, remove duplicate or obsolete internal
material, and check incoming links after deletion. Keep README content useful to callers;
put maintenance details in contributor material. Retain one current checklist and review
record rather than a chain of standalone historical reports.

## Continuous integration

Before signing off schema generation, list every outbound HTTP method and path,
GraphQL operation, event channel, stream frame, signaling route, and other wire
exchange the client can initiate or consume. Match each inventory row to a
checked-in schema entry, its generated endpoint definition and wire types, and
the client call site that uses them. Include query/header parameter names,
nested event and protocol-specific payload properties, and JSON or other
structures embedded in strings or encrypted wrappers, not just outer envelopes. Mark any
implementation-derived entry as such. An endpoint with only a schema entry or
only a generated reference page is incomplete.
Inspect pinned dependencies for network calls as well as repository code. Put
active external HTTP routes and binary message definitions in separate checked-in
schemas or protocol files, and record the exact dependency version and source
paths. A forwarding RoundTripper needs a fail-closed route inventory and paired
request/response replay; it cannot stand in for a schema. Trace direct TLS or TCP
dials outside that wrapper and provide an injectable connection seam so their
framed exchanges can be replayed offline. Add a CI check that detects
new method-and-path pairs, channels, and call sites without schema entries and
fails when regeneration changes checked-in output or creates an untracked
generated file. A plain `git diff` does not detect new untracked model files;
check tracked drift and untracked generated paths. Add negative tests that
introduce an unschematized route, change a method while retaining its path,
and add an unschematized channel; each must fail. Repeat this inventory for
each library; partial provider catalogues are not a sign-off.

Test the source gate at the wire call site after route expressions have been
evaluated. The negative cases must include path appends, wrappers, and invalid
formatting after a generated route value; lexical shadowing of a route variable
in a nested scope; and request method or path changes between construction and
send. Trust a route assignment only when it is guaranteed on every control-flow
path to the send. Include a failure where a generated route is assigned inside
only one branch of `if useList { ... }` and the send occurs after the branch;
include a positive control where every branch assigns the same expected
generated route. For full URLs, reject a construction such as
`fmt.Sprintf("https://%s%s", untrustedAuthority, generatedPath)` even when the
path is generated. Accept only an explicitly approved and inventoried authority
for event URLs; REST base prefixes must also come from configured or inventoried
origins. Include a positive control for an explicitly supported transformation,
such as a generated query map encoded onto its route, so the gate documents its
accepted boundary. In Alexa's directive URL, `c.authority` is the approved
authority. Confirm that `c.authority` and REST base URL fields resolve to the
actual Client receiver for the method; a shadowing local named `c` or a field
with the same name on another value must not establish trust. An inventoried
`Client.Do` must likewise resolve to that receiver's injected client field;
include a failing local `c` shadow for `c.httpClient.Do` or `c.client.Do` and a
positive control using the actual receiver field.

Query and header tests cover setter calls, direct index writes, map literals,
and aliases for `url.Values`, `http.Header`, and custom-header maps. Keys must
come from generated schema constants. Cover a map returned by a helper, a map
passed to a mutating helper, and aliases that reach either case; an unverified
helper must not become a path around the gate. Resolve custom-header maps by
lexical binding: include an outer map with a handwritten key and a nested,
same-named map with generated keys, then verify the outer map still fails when
it is passed to transport. Recursively inspect aggregate arguments: reject
`fill(headerHolder{values: headers})` when `fill` can add a raw key, including
when the map is nested inside another struct, array, or slice. Allow only a
direct handoff to a verified, inventoried wire helper whose header writes are
checked against generated keys. Also reject direct aggregate storage, even when
no helper is called: store a schema-keyed header map in
`holder := struct{ values map[string]string }{headers}` and reject a raw write
such as `holder.values["Cookie"] = "raw"`; store `url.Values` in
`holders := []url.Values{params}` and reject a write such as
`holders[0]["raw"] = []string{"x"}`. Also reject assigning query or header maps
to package-level variables, which could carry unverified keys into another
function. Keep provenance only across aliases proven local to the current
function; include a positive control for a tracked local alias. For schema-bound
`url.Values`, reject both the map and aliases passed as arguments or method
receivers to unverified helpers where handwritten query keys could be added; include a case
where the helper adds a raw key before the map is encoded. Track query-map
provenance by lexical binding and latest assignment. If route logic uses
`len(params)`, accept it as a built-in
only when `len` resolves to the Go builtin; a local function shadowing `len`
must not qualify. Reject a generated empty map reassigned from
`url.ParseQuery(raw)` before encoding, and reject maps sourced from
`url.URL.Query()` or `url.ParseQuery` as outbound generated-key maps even when
the caller later adds generated keys. A nested same-named empty map must not
transfer its provenance to a different outer binding.

Do not allow setter method values for schema-keyed maps to bypass key checking.
Reject aliases such as `set := params.Set` or `add := params.Add` when the saved
method is later called with an unchecked key, and cover aliases of
`req.Header.Set` and `req.Header.Add` as well. Normalize parenthesized receivers
and indexed expressions before applying the same rule. Include raw-key failures
for `(params).Set("raw", value)`, `(params).Add("raw", value)`,
`(params)["raw"]`, `(headers)["Cookie"]`, `(req.Header)["Cookie"]`,
`(req.Header).Set("Cookie", value)`, and `(req.Header).Add("Cookie", value)`,
plus a positive control where a parenthesized receiver uses a generated key.
Preserve request-header key provenance through `http.Header(req.Header)` and
subsequent aliases; a handwritten key written through a converted alias must
still fail. Reject passing a request Header map or its aliases to an unverified
helper. Include a positive exception only for an explicitly named, inventoried
helper whose header writes are checked against generated keys.

Generated selectors must resolve to their exact expected import paths, not only
to imports with matching local names. Resolve the generated route and model
package paths from the module and generated package inventory; resolve standard
library qualifiers to the exact `fmt`, `net/http`, and `net/url` paths. Test
local shadows of `apiroutes`, the generated model-constant package qualifier,
`fmt`, `http`, and `url`, plus counterfeit aliases such as
`import apiroutes "example.com/unverified/routes"` and same-named aliases for
the model package or standard library; each must fail to establish route, key,
formatting, query-map, or request-constructor provenance. `http.NewRequestWithContext`
is trusted only when its qualifier resolves to the actual `net/http` import.
Include positive controls proving the expected generated paths and standard
library imports are recognized, including when valid imports use aliases.

Also reject mutation-capable address and pointer paths for route strings,
query maps, and header maps. Test taking an address and mutating through a local
pointer, passing the pointer to an unverified helper, and passing the map or
string to an unverified helper that can mutate it. These escapes invalidate
the checked value through the eventual wire send unless the gate proves the
pointer path cannot change it.

Scan all shipped production Go modules, including standalone CLI and example
modules, rather than only the SDK package tree. Add a compile-valid unregistered
network call in a CLI module and require the default root gate to reject it.
Track mutable backing storage of request-body readers until the actual send.
A byte slice changed after `bytes.NewReader` and request construction changes
the emitted body; test that mutation and a safe immutable-body positive. Reading
the original reader alone is not proof of a changed request body when the HTTP
constructor clones the reader.

The network inventory tests reject direct calls to unregistered outbound
primitives and unregistered network imports. Exercise HTTP convenience calls
such as `Get`, `Post`, `PostForm`, and `Head`; sends such as `Do` and
`RoundTrip`; request constructors; and the dial or upgrade primitives used by
the library. They also reject primitive method values or method expressions
captured in local variables or at file scope, including package-level aliases
for request constructors, injected client sends, and transport helper methods
such as `(*Client).doRequest`. Exercise request and URL aliases,
post-construction mutation, storage through struct fields or indexed
collections, and request or URL values passed to helpers between construction
and send. Keep an inventoried edge tied to its generated method and route and
its injected transport.

Run the same schema generation, drift, route/channel inventory, and minimum
coverage gates on the exact release tag commit. Passing a prior `main` run is
not evidence that a different tag commit meets the release standard.

The current bootstrap CI pins golangci-lint and runs all linters, Go vet,
build and race tests, Python lint/format checks, and offline reference tests.
The SDK verification gates described above are release requirements still to
be implemented, including explicit Go formatting/module-tidy checks, schema
generation, endpoint/model provenance, SDK coverage, and rendered documentation.
Keep checks offline and deterministic. Live endpoint tests remain opt-in and
must not require credentials in pull request CI. No widget schema or API site
is included in this repository; see [website requirements](website.md).
