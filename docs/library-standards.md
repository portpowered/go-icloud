# Library standards

Apply these requirements when creating a library from this template:

1. Keep the public client, examples, README, and site independent of any consuming application. Put application adapters and rollout plans in the consuming repository.
2. Document supported operations, authentication, errors, and transport injection with examples that match the exported API. Add customer-facing operation guides for important workflows, and distinguish verified behavior from synthetic examples and historical references.
   Put sanitized, schema-valid request, response and event examples in the canonical schemas,
   including complete envelopes, nested payloads, resource updates, relation changes and failures.
   Validate every example against its owning schema in CI and identify its evidence class in
   contributor material. Never publish credentials or private captures as examples.
3. Show Go version, CI, coverage, release, Go Reference, license, and documentation badges in the README. Replace every example repository value and point badges to live reports.
4. Generate the API reference in CI with the shared Fumadocs action and publish it to GitHub Pages.
   Describe supported HTTP operations, events, socket exchanges and known payload variants in
   checked-in schemas or source protocol files. Generate wire models and protocol constants from
   those inputs, use them in transport code, and check regeneration for drift. Keep public semantic
   models separate from provider wire models. Preserve documented open fields and future values.
   Include active network exchanges made by pinned dependencies and provide injectable transports
   so their request/response or framed exchanges can be tested offline.
   Verify actual emitted requests and public results with paired replay tests. Validate schema
   examples and representative malformed payloads with established schema tooling. Review the
   generated reference's required fields, variants, examples and request snippets for usability.
   Maintain a practical supported-operation list linking client methods, contracts and replay cases.
   Review changed serialization code and enum conversions for correctness; add a focused regression
   test when a concrete bug or compatibility risk warrants one.
   **Verification must remain proportional to the library change. Do not build or require custom
   compiler/AST provenance engines, interprocedural data-flow analysis, exhaustive alias or ownership
   proofs, per-value lineage registries, proof receipts or proof caches as library acceptance gates.**
   Unresolved symbolic provenance is not a functional defect or a release blocker. Use compilation,
   standard static checks, reproducible generation, schema validation and behavioral tests to assess
   the implementation. Do not turn a library task into a verification-engine project unless the user
   explicitly requests that separate project. Do not add adversarial tests of hypothetical verifier
   bypasses, fixture-token parity machinery or repeated audit runs solely to satisfy such machinery.
   Keep style fixes small; add tests for behavior and compatibility rather than formatting changes.
5. Run offline build, lint, race, and replay checks before release. Configure golangci-lint v2 with the literal `linters.default: all`, pin its version in CI, and make the full-repository lint run a blocking gate. Do not set `--issues-exit-code=0`, continue after lint failures, or limit CI to new issues. Keep all linters enabled; any exception must name the narrow rule and affected path, give its reason in a config exclusion or beside a source annotation, and receive independent review. Style fixes must preserve persisted example/config JSON keys; lock them with a regression test or document an intentional key migration. A reviewer who did not implement the migration must confirm passing blocking CI on the exact commit before checking item 5. Keep real captures sanitized and separate from synthetic fixtures.
6. Add deterministic synthetic request, response, error, and session fixtures for supported behavior. Measure coverage of non-generated production code by public and transport package and in combination; enforce at least 80% combined coverage in CI and target 90%. Report generated-code exclusions and remaining uncovered behavior rather than adding tests solely to raise a number.
7. Put the reusable public provider package under `pkg/<provider>`, generated provider wire models under `pkg/dependencymodels`, and transport behavior under `pkg/dependencies/<transport>`. Use distinct schema and generated Go files for each API responsibility, such as authentication, behaviors, devices, and feature payloads; a compatible shared Go package is allowed. Keep related request, response, and nested component definitions together rather than a monolithic model file or a second catch-all `internal/models` or `internal/wire` model bucket. Keep public semantic projections separate from provider wire contracts. Handwritten model companions may supply conversion or decoding behavior but must not redefine wire fields. Use compatibility aliases when moving existing exported types; when old field shapes differ, generate their compatibility definitions from a separate projection schema. Verify public import paths from a separate consumer module.
8. Initialize clients through explicit functional options (for example `NewClient(WithBaseURL(...), WithHTTPClient(...))`) with sensible defaults and validation. Keep account credentials out of reusable client configuration when the client serves multiple accounts.
9. Keep the reusable client stateless with respect to accounts and connections. Return explicit session objects for login, event streams, sockets, RTC, or other stateful lifecycles; make ownership, close, errors, and token state visible to callers.
   Include injected transport state in this audit. A shared `http.Client.Jar` must not
   transfer account cookies between sessions. Reject unsafe shared cookie jars with a
   distinguishable configuration error or keep cookie state in explicit sessions. Test
   two accounts through the same reusable client and assert complete outbound requests.
10. Allow callers to inject the transport at every network edge the library uses, including HTTP, HTTP/2, WebSocket, MQTT, RTC signaling, and sockets opened by dependencies as applicable. A configurable concrete dialer is insufficient when it cannot substitute an offline connection; provide a connection-producing dial hook or equivalent seam and test the actual framed request and response through it without real credentials or network access.
11. Expose token exchange and refresh as explicit operations that return the current credentials to the caller. Do not silently refresh or retain updated tokens inside a reusable client; document caller storage and renewal responsibilities.
12. Publish all customer-facing guides as MDX files under `docs/guides/` in the GitHub Pages site. Link guides to the matching generated reference pages. Keep separate repository Markdown only for contributor and release process notes; check internal links from **all** rendered pages, including the site root and generated references, and review external destinations and release-note links after a docs migration. Check schema-supplied links such as `externalDocs` even when they are loaded at runtime and absent from static HTML anchors. Verify the destination guide exists and renders its expected content; HTTP 200 alone can be a fallback error page.
13. Before release, edit every published page for concise copy: remove repeated caveats, stale claims, and links to duplicate repository documents; keep each page's purpose, evidence status, and next action clear. Keep the README focused on installation, a short authenticated example, supported capabilities, caller configuration and lifecycle obligations, and links to user guides. Put wire inventories, generation details, fixture source notes, coverage mechanics, migration history, and reviewer evidence in contributor material, not the README or customer guide navigation. Delete obsolete internal reports and duplicate process documents; keep one current checklist and independent review record plus contributor instructions needed to maintain the library. Review every tracked documentation file for audience, purpose, duplication, and incoming links, then review the rendered Pages site. Check release-note copy and URLs against the published guide locations.
14. Have an independent reviewer inspect the changed implementation, public API, tests,
    documentation and passing CI at the final commit before merging or releasing. Focus on
    functional correctness, security, compatibility and maintainability. Record the reviewed
    commit, concrete findings and their resolution in the pull request or one current review
    document. Re-run affected checks after fixes. Scale review depth and additional reviewers
    to the change's risk; do not require exhaustive per-value inventories, formal ownership
    proofs or repeated full-library checklist audits for routine changes. For a broad migration,
    review supported workflows, generated contracts, public consumer installation and useful
    customer documentation against functional test evidence.
15. Store and replay each wire exchange as a paired request and response (or an ordered bidirectional message transcript). Include method, origin, escaped path, repeated query values, relevant headers, and body or frame payload in the request expectation; include response status, relevant headers, and body. Match the outbound request before returning its response, reject unexpected or duplicate calls, and assert that every expected exchange was consumed in order where order matters. Never fall back to a response when request matching fails. Represent volatile IDs, timestamps, signatures, and redacted credentials with explicit match rules that still validate their format or decoded meaning. Apply this to every supported transport and classify each pair as captured or synthetic; a response-only fixture does not satisfy replay verification.
    Match authentication forms and headers in full, including CSRF, OTP, and token exchange fields.
    Bind OAuth state to the callback, validate PKCE challenge/verifier relationships, and bind the
    hardware identity across requests. Validate volatile field formats before normalization.
    Match every signaling envelope and handshake. Check lifecycle completion after the expected
    close or teardown; a startup notification or terminal read timeout is not cleanup proof.

16. Provide an installable standalone CLI that consumes the public SDK so customers can test the library without a consuming application. Use a separate module under `cmd/go-<provider>`; keep CLI concerns out of the SDK. Cover authentication and explicit token exchange, device or endpoint discovery, important read/control workflows, and event/session lifecycles where supported. Include useful help, machine-readable output, nonzero failures, cancellation, and session cleanup. Accept credentials through documented environment, stdin, or explicit file inputs; keep secrets out of arguments and ordinary output, and make credential export an explicit action. Require explicit commands for device changes. Document installation and customer examples in an MDX guide. Test CLI commands offline through injected paired request/response transports, including authentication errors and lifecycle cleanup, and run blocking pinned all-linter, build, test, and module checks for the CLI in CI. Verify a separate consumer installation from the published CLI module and release its module tags with the SDK.
    Make routine use a customer workflow: complete authorization, store credentials with
    explicit ownership, enumerate devices, select a device by ID, and expose useful reads
    and controls through named commands. Supply stable client identities, protocol defaults,
    and discovered device metadata automatically. Do not require request JSON, manual
    credential copying, or wire parameters for the ordinary login and device-control flow.
    Provide logout and document credential storage. Write the CLI guide as a short ordered
    sequence of copyable installation, login, discovery, and device-operation instructions.
    Explain how to choose discovered IDs; move advanced formats and protocol details to
    separate help or references. Verify the documented sequence offline, including failed
    authorization, device lookup, cancellation, and session cleanup.
    When supported by the provider, implement the complete interactive authorization flow,
    including browser consent, a localhost callback and provider-required activation calls.
    Expose composable SDK authorization mechanisms for caller-owned callback endpoints.
    Validate callback state and PKCE when supported; bind the exact redirect to the exchange.
    Keep listeners and temporary authorization state in an explicit cancellable lifecycle,
    with offline browser, listener and transport injection and idempotent cleanup. Document
    registered redirects, credential ownership and explicit export; manual code exchange alone
    is not end-to-end authorization.

See [verification](verification.md), [client design](client-design.md), and
[website publishing](website.md) for the implementation details.
