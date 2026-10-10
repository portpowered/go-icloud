# Documentation audience audit

This inventory covers every tracked Markdown and MDX file in the integrated
documentation source based on library commit `ca02c17`. It separates customer
usage from contributor contracts, provenance and review records (LIB-17;
template items 12–13). Add new documents here when their implementation commits
are integrated. This source audit does not certify the rendered deployment.

| File | Audience and unique purpose | Audit disposition |
| --- | --- | --- |
| `README.md` | Customer landing, installation, SDK entry point, badges | Native authentication first; saved-session route retained; focused installation and service guides. |
| `docs/guides/index.mdx` | Customer service navigation | New MDX landing, links to each operation guide. |
| `docs/guides/authentication.mdx` | Customer account/session lifecycle | Native SRP, MFA, bridge, hardware/injected assertions and complete private state; saved-session renewal retained. |
| `docs/guides/account.mdx` | Customer account discovery | New devices/family/storage usage, generated reference links. |
| `docs/guides/findmy.mdx` | Customer discovery and controls | New device/session usage, selection and cleanup. |
| `docs/guides/drive.mdx` | Customer browsing and transfers | New public flow, reference links and explicit file ownership. |
| `docs/guides/photos.mdx` | Customer libraries, assets, mutations, uploads and local synchronization | Current generated APIs, callback ownership, private receipts, explicit provider-error and date-dialect adaptations. |
| `docs/guides/shared-photos.mdx` | Customer legacy shared streams | Distinct account-discovered read service, null/empty downloads and excluded inert Source mutations. |
| `docs/guides/reminders.mdx` | Customer lists/snapshots/writes | New revision ownership and core mutation usage. |
| `docs/guides/configuration.mdx` | Customer injection and error handling | New cancellation/session/transport ownership guidance. |
| `docs/guides/cli.mdx` | Customer CLI install and commands | Native auth, full typed Photos reads/writes/visitors/sync and Reminders writes; strict requests and private receipts. |
| `docs/native-authentication.md` | Contributor SRP, challenge and trust contracts | Preserved integrated generated-model decisions and scoped synthetic evidence; no live authentication claim. |
| `docs/authentication-bridge.md` | Contributor owned socket lifecycle and framing | Preserved bootstrap, protobuf/websocket provenance, cancellation, state copies and paired timelines. |
| `docs/authentication-security-key.md` | Contributor HID/WebAuthn boundary | Preserved portable backend pin, injected device seam, cryptographic/frame tests and physical-device evidence limits. |
| `docs/authentication-pcs.md` | Contributor consent, terms and logout behavior | Preserved explicit consent ownership, polling/cancellation and credential clearing semantics. |
| `docs/photo-sync.md` | Contributor local materialization and Source function inventory | Preserved native JSON state adaptation, Source replay scope and pinned Windows/C date dialect. |
| `docs/account-wire-contracts.md` | Contributor model decisions and account evidence | Retained unique unknown-field/projection provenance; historical milestone scope identified. |
| `docs/drive-wire-contracts.md` | Contributor multipart, date, session and transfer contracts | Retained wire decisions and measured evidence; historical totals are not current coverage. |
| `docs/findmy-wire-contracts.md` | Contributor generated Find My wire contracts | Retained context ordering, controls and model decisions; current signoff linked separately. |
| `docs/reminders-wire-contracts.md` | Contributor CloudKit model and lookup projections | Retained unique field/coercion contracts; removed obsolete unimplemented-write claim. |
| `docs/client-design.md` | Contributor public package and ownership boundaries | Replaced generic template narrative with actual client/transport boundaries. |
| `docs/findmy-session.md` | Contributor Find My lifecycle/state contracts | Consolidated implementation contract; customer examples moved to MDX. |
| `docs/findmy-behavior.md` | Contributor Source behavior evidence | Consolidated projection/monitor semantics with explicit replay limits. |
| `docs/findmy-transport.md` | Contributor exact request encoding and provenance | Consolidated route/receipt contract, removed stale milestone counters. |
| `docs/reminder-writes.md` | Contributor CRDT/mutation evidence and exceptions | Owned by Reminders integration; retained separate detailed evidence. |
| `docs/reminders-text-protocol.md` | Contributor binary text/protobuf provenance | Retained byte-level decoding decisions, generation pin and corpus boundaries; linked write encoder evidence. |
| `docs/library-standards.md` | Contributor shared acceptance requirements | Authoritative standards retained, never duplicated as customer prose. |
| `docs/completion-matrix.md` | Contributor current implementation acceptance record | Preserved; integration/review owners update verdicts. |
| `docs/migration-checklist.md` | Contributor ordered migration work record | Historical snapshots explicitly separated; original receipts and acceptance boxes preserved; current work linked to completion matrix. |
| `docs/independent-review.md` | Independent reviewers' verdict and evidence | Preserved verbatim; documentation work cannot award itself approval. |
| `docs/operation-matrix.md` | Contributor historical live capture scope | Retained account-specific observations; remaining gaps labeled historical batch scope. |
| `docs/python-provider-migration-playbook.md` | Contributor reference-first evidence workflow | Replaced copied second checklist with concise workflow and authoritative links. |
| `docs/reference-capture.md` | Contributor instrumentation and private artifact handling | Consolidated current commands, provenance classes and replay expectations. |
| `docs/reference-endpoint-coverage.md` | Contributor endpoint occurrence audit semantics | Removed changing totals and obsolete deferred-status claim; linked actual gate semantics. |
| `docs/reference/README.md` | Contributor historical reference classification | Retained unique historical evidence rule. |
| `docs/releasing.md` | Contributor exact commit/tag release procedure | Consolidated actual library/CLI publishing boundaries and acceptance prerequisites. |
| `docs/verification.md` | Contributor gate ownership and commands | Removed duplicated standards/control checklist and stale bootstrap-only claims. |
| `docs/website.md` | Contributor renderer/deployment procedure | Updated pinned action, recursive schema view, site gate and unresolved external rollout. |
| `docs/documentation-inventory.md` | Contributor audience/duplication audit | This exhaustive baseline record; no independent acceptance verdict. |
| `tests/replay/README.md` | Contributor matcher and SDK replay distinction | Removed stale counts and zero-SDK claims; retained exact matching/normalization rules. |
| `tests/replay/fixtures/captured/README.md` | Contributor captured artifact provenance | Retained unique live evidence/sanitation classification. |
| `tests/replay/fixtures/synthetic/README.md` | Contributor synthetic artifact provenance | Retained unique implementation-derived classification. |
| `tests/replay/fixtures/synthetic/http/README.md` | Contributor HTTP scenario families and observations | Retained unique Source evidence descriptions; corrected date exception guidance. |
| `tests/replay/fixtures/synthetic/socket/README.md` | Contributor socket timeline evidence limits | Retained transcript framing/clock/provenance obligations; historical seam cannot prove complete native auth. |
| `pkg/dependencies/srp/testdata/README.md` | Contributor SRP corpus provenance | Preserved pinned Source vectors, synthetic input classification and private-secret exclusion. |

Contributor documents are linked from repository instructions, contracts and
acceptance records; removing them would discard review provenance. Repeated
customer setup and operation examples now live in MDX rather than parallel
Markdown guides. Wire contract milestone numbers remain historical evidence,
while changing aggregate totals come from the actual audit commands.

The site expectations cover every canonical OpenAPI operation and both canonical
bridge channel directions, plus all customer guides. AsyncAPI 2.6 channel
presentation requires the shared renderer's explicit version adaptation; keep the
canonical socket schema and its extension references intact. The final integrated
commit still needs a complete export, visual inspection, external deployment/link/
badge checks and independent verdicts. Source inventories and local baseline
checks do not substitute for those release requirements.
