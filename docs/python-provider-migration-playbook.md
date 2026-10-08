# Python provider library to Go migration playbook

Use this playbook to turn an existing Python provider client into a standalone
Go library built from this template. Work through the tasks in order. Record
open questions as unknowns until provider evidence resolves them; do not turn
an assumption into a supported contract by copying it into Go.

The intended boundary is one provider library containing protocol behavior and
a small application adapter that translates between the library and the
application. Keep application storage, scheduling, orchestration, and provider
selection outside the library. See [client design](client-design.md) for the
conventions used by this template.

## Capture the reference before rebuilding it

The migration order is **reference inventory, instrumented reference, portable
replay artifacts, then Go implementation**. Do not write the replacement first
and use its own fixtures as proof of compatibility (library standards 6 and 15).

- [ ] Clone the reference library and pin its repository URL and commit. Record
  any local patches, dependency versions, and maintained-fork differences.
- [ ] Instrument the reference at its actual network edges before constructing
  its client. Capture constructor traffic, authentication and verification,
  session reuse, service instantiation, and each caller operation.
- [ ] Provide a CLI for interactive login and selected operations when live
  account access is needed. Store reusable session credentials and unreviewed
  captures outside version control; never persist passwords or entered codes.
- [ ] Store exchanges in a versioned, language-independent format containing
  ordered request/response pairs, source revision, capture time, redactions,
  explicit match rules, operation inputs, and expected public results. The
  format must be consumable by both reference and replacement harnesses without
  importing either implementation's classes or serializing their objects.
- [ ] Maintain a behavior matrix for every operation in scope: setup/auth,
  success, zero/one/many results, pagination, optional or missing fields,
  malformed responses, transport failure, authentication/expiry, throttling,
  and provider errors as applicable. Mark unobserved and inapplicable cases;
  a coverage percentage alone does not establish functional completeness.
- [ ] Establish reference replay before porting each operation. Reuse the same
  reviewed artifacts to verify the Go replacement's requests and semantic
  results, allowing only documented intentional public-API mappings.
- [ ] Keep live capture opt-in. Replay must run offline repeatedly in CI. Use
  labeled synthetic scenarios to exercise failures that cannot be captured;
  never describe those scenarios as observed provider behavior.

Reference instrumentation is migration tooling, not the public Go client.
Record any uncaptured network edges, including dependency-owned sockets, as
coverage gaps. Private recordings are evidence awaiting sanitation and review,
not publishable replay fixtures. Update the matrix as new captures arrive.

## 1. Inventory the Python implementation

Before moving code, list the behavior callers rely on. Search both the provider
package and its consumers, since application code may depend on behavior that
is not documented in the library itself.

- [ ] List every public method and the call sites that use it.
- [ ] Trace each method through request construction, authentication, transport,
  response decoding, retries, and error translation.
- [ ] Record supported API versions, endpoints or RPC methods, pagination,
  filters, timeouts, and any rate-limit behavior.
- [ ] Identify provider models, wire models, configuration objects, exceptions,
  callbacks, caches, background tasks, and long-lived connections.
- [ ] Record dependencies and the Python runtime features they provide.
- [ ] List existing tests, fixtures, live tests, and the source of each fixture.
- [ ] Identify application-only behavior mixed into the package, such as
  credential storage, job scheduling, database access, or provider selection.

Create a compact operation matrix and keep it with the migration notes:

| Operation | Python entry point | Provider request | Auth needed | Result meaning | Failure behavior | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| Example: fetch item | `get_item(...)` | `GET /items/{id}` | account token | one item | 404 means absent | capture or documented source |

Do not start by translating modules one-for-one. Use the inventory to identify
the provider contract first, then group the Go API around caller operations.

## 2. Record evidence and provenance

For each behavior in the operation matrix, note what establishes it: a sanitized
provider exchange, current official documentation, an existing test, or an
unverified assumption. Include source URLs or repository paths and dates where
available.

- [ ] Put observed and sanitized exchanges in
  `tests/replay/fixtures/captured/` with a neighboring provenance note.
- [ ] Record the operation, UTC capture date, source category, redactions, and
  the behavior each capture supports.
- [ ] Put hand-authored or generated examples in
  `tests/replay/fixtures/synthetic/` and label them synthetic.
- [ ] Keep superseded protocol notes under `docs/reference/` and label them
  historical.
- [ ] Remove credentials, cookies, personal data, and account or device IDs
  before adding any fixture.
- [ ] Keep unresolved questions in a migration note; do not treat a synthetic
  fixture or old document as proof of current provider behavior.

Follow [verification and fixture guidance](verification.md) for the repository
layout and provenance fields.

## 3. Define protocol schemas and behavioral contracts

Describe the provider contract independently of Python class definitions.
Python wire models can encode incidental details such as permissive defaults or
omitted fields; preserve only behaviors callers need and provider evidence
supports.

- [ ] For every operation, record method/path or RPC name, headers, query
  parameters, request body, success statuses, response body, and pagination.
- [ ] Mark required, optional, nullable, omitted, and unknown fields separately.
- [ ] Record identifier formats, timestamps and time zones, enum values,
  number units, ordering, and empty-result semantics.
- [ ] Capture provider-specific details such as token refresh, continuation
  tokens, long polling, WebSocket messages, or idempotency keys.
- [ ] Decide which malformed or partial responses should produce a typed error.
- [ ] Compare each claim with the evidence from task 2 and label gaps clearly.
- [ ] Treat the checked-in schemas as the source of truth for provider request,
  response, and internal model structures. Generate the Go model types from
  those schemas and migrate implementation code to use them instead of
  maintaining duplicate handwritten wire or internal structs.
- [ ] Inventory nested event properties, query/header names, and structured
  payloads encoded inside strings or encrypted wrappers; generate their wire
  models and constants from the checked-in schemas too.
- [ ] Check in the generation configuration and command, and add a CI check
  that fails when regenerating the models changes the working tree.
- [ ] Prove the endpoint gate fails for an unschematized method/path, a changed
  method on an existing path, and an unschematized event channel.
- [ ] Keep hand-written types only for library behavior or intentional semantic
  mappings that are not represented by the schemas; document those mappings
  and keep conversions at a clear boundary.

Place generated wire types in `pkg/dependencymodels`, split by API responsibility
alongside their schema components. Do not retain a parallel internal models bucket.
Inventory every serialized production struct and link it to a generated schema type;
add negative tests for handwritten and anonymous nested wire definitions. When a public library type has different
semantics from the provider schema, convert explicitly at the boundary. See
[client design](client-design.md) for the root package and transport split.

## 4. Map the Python API to a Go API

Make a mapping before implementation. Go callers should see stable operations
and provider concepts, not the source language's class hierarchy or convenience
wrappers.

| Python pattern | Go mapping | Review question |
| --- | --- | --- |
| `client.get_item(id)` | `Client.GetItem(ctx, GetItemRequest{...})` | Is the operation meaningful to a caller? |
| `dict` request/result | Named request and result structs | Which fields are required and validated? |
| `None` for absence | Documented zero value, optional field, or typed not-found error | What does the provider contract actually mean? |
| Python exception hierarchy | `*service.Error` with stable `ErrorKind` and wrapped cause | Which cases must callers branch on? |
| Async task or generator | Context-aware operation or explicit session | Who owns cancellation and closing? |
| Mutable client config | Immutable construction options plus request data | Can one client safely serve multiple accounts? |

- [ ] Define the exported `Client` methods and named request/result types in the
  root package.
- [ ] Add `context.Context` to every operation that can wait on I/O.
- [ ] Keep wire-only fields and provider response structs unexported.
- [ ] Document zero values, optional data, units, ordering, and partial results.
- [ ] Keep provider APIs specific to this provider. Add shared capability
  interfaces only after multiple independent clients establish the same
  semantics.
- [ ] Check that each exported type or method has a clear caller use and can be
  supported over the library's intended lifetime.

## 5. Move authentication to the library boundary

The library may perform provider authentication, but it must not own an
application's credential store. Model the credential inputs callers need and
keep them scoped to an operation or explicit account/session object.

- [ ] Document whether the provider accepts API keys, bearer tokens, signed
  requests, cookies, client credentials, or refreshed access tokens.
- [ ] Move only provider token exchange and token use into the library.
- [ ] Keep persistence, secret retrieval, rotation policy, and account linking in
  the application adapter.
- [ ] Put account credentials on requests or an explicit account-bound client;
  never mutate shared client authorization state per call.
- [ ] Define refresh and expiry behavior, including whether concurrent calls may
  trigger refresh and how refresh failures are returned.
- [ ] Ensure errors, logs, examples, fixtures, and tests never print or store
  live secrets.

## 6. Port the transport behavior

Implement transport code in a focused package such as `httpclient`. Keep the
public request/result types independent from HTTP details.

- [ ] Inject the HTTP dependency so callers can configure proxies, tracing,
  custom TLS, and deterministic tests.
- [ ] Build requests with the caller context and document timeout ownership.
- [ ] Validate base URLs and caller input before sending requests.
- [ ] Preserve required headers, query encoding, path escaping, body encoding,
  and response size limits where the provider contract requires them.
- [ ] Close response bodies and handle nil or unreadable responses safely.
- [ ] Make retries explicit: define retryable failures, maximum attempts,
  backoff, and idempotency requirements. Do not retry writes by default when
  duplicate effects are possible.
- [ ] Keep transport configuration immutable after construction and safe to
  share across goroutines.

If the Python package supports another protocol, give it a focused Go package
with the same public client contract instead of adding protocol details to root
API types.

## 7. Define error translation

Map provider and transport failures to a small set of stable error kinds that
callers can handle. Preserve useful causes and metadata without exposing
secrets or raw response bodies by default.

- [ ] Classify invalid caller input, configuration, cancellation/deadline,
  network failure, authentication, not found, rate limiting, provider server
  failures, unexpected statuses, and invalid responses as applicable.
- [ ] Decide which provider error codes map to each public kind; document
  ambiguous and unknown cases.
- [ ] Preserve the operation name, status code, provider request ID, and wrapped
  cause when available.
- [ ] Use `errors.Is` and `errors.As` compatible wrapping so callers can inspect
  typed errors and context cancellation.
- [ ] Bound or omit error-body excerpts and scrub credential-bearing values.
- [ ] Verify that returned zero values are safe to ignore whenever an operation
  returns an error.

## 8. Preserve concurrency and session semantics

Python locking, async tasks, and connection ownership often hide lifecycle
behavior. Write down who owns each piece of mutable state before translating it.

- [ ] Decide whether the client is safe for concurrent calls and make its
  dependencies satisfy that promise.
- [ ] Keep per-account tokens, request state, and decoded response state out of
  mutable shared client fields.
- [ ] Replace background tasks with caller-driven context-aware work unless the
  public library contract requires a managed session.
- [ ] For a streaming or long-running provider connection, expose an explicit
  session with context-aware open, documented ownership, idempotent `Close`,
  terminal error reporting, and a defined backpressure policy.
- [ ] Specify behavior when cancellation, `Close`, a provider failure, and a
  concurrent read or write happen together.
- [ ] Check that constructors and cleanup do not leak goroutines or connections.

Skip session types for ordinary request/response APIs. See the optional session
guidance in [client design](client-design.md) when a provider requires one.

## 9. Build replay fixtures and deterministic tests

Use the evidence from task 2 to test provider behavior offline. Keep tests
independent of live credentials and network access.

- [ ] Add replay tests for each supported operation's observed success and
  important failure responses.
- [ ] Assert request method, escaped URL, headers, authentication placement,
  query/body encoding, and response decoding.
- [ ] Keep each expected outbound request and its response in one replay pair;
  for sockets and streams, keep ordered bidirectional frames. Match the
  request before returning its response, reject unexpected or duplicate calls,
  assert all pairs were consumed, and never fall back after a mismatch.
- [ ] Give volatile and redacted fields explicit format or decoded-value
  matchers. Audit every transport and fixture for response-only replay cases.
- [ ] Cover pagination, optional or missing fields, malformed responses,
  oversized responses, rate limits, and request IDs where relevant.
- [ ] Test context cancellation, transport failures, and concurrent use when
  those are part of the contract.
- [ ] Use synthetic fixtures for edge cases that cannot be safely captured,
  with their synthetic status visible in the path or metadata.
- [ ] Keep live-provider checks opt-in and separate from ordinary CI.
- [ ] Verify tests do not depend on map iteration order, wall-clock timing, or
  external services.

## 10. Connect the Go library through an application adapter

Connect the Go library to the application through a narrow adapter. Keep
application workflows and persistence in the application.

- [ ] Construct the client once from application configuration and injected
  transport dependencies.
- [ ] Translate application requests and credential values into library
  request types.
- [ ] Translate library results and typed errors into the application's
  existing contract.
- [ ] Keep storage, scheduling, provider choice, and cross-provider workflows
  outside the Go library.
- [ ] Compare old and new behavior for every operation in the inventory matrix.
- [ ] Remove the duplicated Python provider implementation only after the Go
  adapter covers the supported behavior and application callers have migrated.

## 11. Write user-facing documentation

- [ ] Replace all template module names, package names, environment variable
  names, and example provider operations.
- [ ] Document supported operations, authentication inputs, timeouts, errors,
  concurrency guarantees, and any session lifecycle.
- [ ] Write customer-facing operation guides in `docs/guides/` that walk through
  the important supported tasks, including setup and authentication, required
  inputs, the Go calls involved, expected results, and relevant error or session
  behavior. Label captured behavior, synthetic examples, and unverified
  assumptions separately.
- [ ] Add the guides directory to the GitHub Pages build and review navigation
  and internal links across the whole rendered site, plus external and release
  links, alongside the generated API reference. See
  [website publishing](website.md) for the action input and directory layout.
- [ ] Provide a compiling example that uses a context and demonstrates typed
  error handling without real credentials.
- [ ] Label unsupported, unverified, synthetic, or historical behavior clearly.
- [ ] Explain setup and migration for application callers without exposing
  internal implementation details as part of the API.
- [ ] Review the README and examples from the perspective of a new Go consumer.
- [ ] Remove maintenance inventories, review history, and fixture audit details from the
  README and customer guides. Inspect every tracked document, delete redundant internal
  reports, and retain one current checklist and independent review record.

## 12. Prepare the module for release

- [ ] Replace the template module path and package names throughout the source,
  examples, tests, and CI/release configuration.
- [ ] Choose the license and include its license file.
- [ ] Run `gofmt`, `go mod tidy`, `make lint`, `go test -race ./...`, and
  `go build ./...`; `make lint` runs go vet and the pinned golangci-lint v2
  configuration with literal `linters.default: all` over the full repository.
  Keep it as a blocking CI gate; any exclusion must be narrow, rule-specific,
  and justified in configuration or beside a source annotation.
- [ ] Review API compatibility output and confirm each exported symbol is
  intentional.
- [ ] In a separate temporary module, fetch the candidate module version and
  compile a small consumer using the documented API.
- [ ] Confirm CI is offline and deterministic; keep credentials and private
  captures out of the repository.
- [ ] Follow [release guidance](releasing.md) for module versioning, tags, and
  published-module verification.
- [ ] Have an independent reviewer who did not implement the migration verify
  each library standard at a named commit. Record every finding, resolve all
  findings and open checklist items, and have the reviewer check the affected
  fixes at the final commit before checklist sign-off. The reviewer confirms
  passing blocking CI on that exact commit before checking library standard 5.

## Acceptance criteria

The migration is ready for review when all of the following are true:

- [ ] The Go module owns only one provider's protocol contract and contains no
  application storage, scheduling, or orchestration dependency.
- [ ] Every supported operation has a public Go request/result contract and a
  recorded evidence source; unresolved behavior is labeled as such.
- [ ] Schema-defined provider and internal models are generated reproducibly,
  implementation code uses those generated types, and CI checks for stale
  generated output.
- [ ] An independent complete wire-model inventory finds no handwritten wire definitions
  or duplicate internal model bucket; generated files and schemas are split by API.
- [ ] Captured, synthetic, and historical material is stored separately and
  sensitive values have been removed.
- [ ] Authentication, transport, failure mapping, cancellation, and concurrency
  behavior are documented and match the implementation.
- [ ] Replay and unit tests cover the supported contract without live network
  access or credentials.
- [ ] The application adapter covers the inventory matrix and the application
  no longer needs the duplicated Python provider implementation.
- [ ] Documentation examples compile from a separate consumer module.
- [ ] Customer operation guides are published under the GitHub Pages docs site
  and describe supported behavior with evidence labels.
- [ ] `make check` and the release readiness steps pass, and the reviewed public
  API contains only intentional exports.
- [ ] The independent standards review and exact-tag schema, endpoint, and
  coverage gates are recorded before release.
