# Completion acceptance matrix

This is the current work list for finishing the selected-service migration.
The compatibility source is `tools/reference/source.json`'s live revision.
Captured evidence, synthetic reference scenarios, and historical milestones are
separate evidence classes (LIB-05, LIB-12). A route occurrence does not establish
operation parity. No row below grants final merge or release acceptance.

## Implemented operations and pending acceptance

| Area | Required behavior | Acceptance |
| --- | --- | --- |
| Photos libraries | Discover and select primary, additional private, SharedSync and shared zones | Implemented; discovery and selected-library replay checks pass; integrated acceptance pending |
| Photos albums | Create, rename, delete, add asset; revision and relation handling | Implemented; mutation replay checks pass; integrated acceptance pending |
| Photos assets | Favorite/unfavorite, delete, shared-library reconciliation | Implemented; mutation and shared-library replay checks pass; integrated acceptance pending |
| Photos uploads | Reserve, transfer, register, status/hydration, duplicate and timeout behavior, optional album attachment | Implemented; 46 paired scenarios plus framing and ownership controls pass; integrated acceptance pending |
| Photos changes | Cursor discovery, zone/container pages, lookup, ordered changes and tombstones | Implemented; cursor, paging, lookup and tombstone replay checks pass; integrated acceptance pending |
| Photos legacy streams | Discover albums, count, enumerate, lookup and download | Implemented; legacy read and defensive-value controls pass; integrated acceptance pending |
| Photos local sync/watch | Persistent state, materialization, selection/options, reconciliation, bounded cancellable watch | Implemented, including CLI sync/watch; filesystem and pinned date controls pass; final integrated acceptance pending |
| Reminders | Create, update, soft delete; CRDT encoding and resolution tokens | Implemented; 11 paired scenarios, full local checks and two scoped reviews |
| Reminders hashtags | Create, rename, soft delete; atomic parent links and revision updates | Implemented; 6 paired scenarios and full local checks; final review pending |
| Other Reminders relations | Location alarm/trigger, URL/image attachment updates, recurrence rules | Implemented; 15 paired scenarios and an empty-ID control; scoped race checks; integrated acceptance pending |
| Auth login | Native SRP s2k/s2k_fo, service one-factor paths, paused MFA and returned credentials | Implemented; combined SDK race verification pending |
| Auth verification | Status, trusted devices, two-step codes, trusted-device/SMS two-factor codes, trust, hardware assertion | Implemented, including portable hardware adapter; combined SDK race verification pending |
| Auth consent/logout | PCS/web consent polling, logout scopes, failure-stage rotations and credential ownership | Implemented, including resumed consent policy; combined SDK race verification pending |
| Auth sockets | Injected raw WebSocket, protobuf bootstrap/subscription/ACK, signing/proofs, modern/legacy bridge, teardown | Implemented; 53 socket and 30 combined paired timelines plus lifecycle controls; integrated acceptance pending |
| CLI | Public SDK workflows for all supported reads/writes, native login/verification/logout, cancellation and cleanup | Implemented; complete workspace race suite passed at `8743c53`; published SDK pin/tidy verified at `970b428`; public-pin race and final acceptance pending |

For each port: bind source inputs/defaults and complete results/errors to named
schema models, execute the public Go operation against every applicable paired
scenario, assert full consumption, and add missing functional and negative
controls. Validate time, randomness and framed payload meaning explicitly;
never ignore fields to accommodate the port (SCHEMA-10, LIB-05).

## Integration evidence and remaining blockers

At integration snapshot `7334404`, the SDK implementation is publicly available
as `v0.0.0-20261010022446-ed7c72b31b4c`. The separate CLI pins that version;
its isolated `GOWORK=off` dependency download and tidy checks passed. The complete
CLI race suite passed at `8743c53` using an integration workspace. A complete race
run against the final public pin is still required.

At `3b66d50`, `make sdk-coverage` passed all SDK replay and production-package
tests under the race detector. Non-generated statement coverage was replay
10721/12874 (83.3%), unit 4314/12874 (33.5%), and combined 11107/12874 (86.3%).
The replay gate includes ten pinned Source HID transcripts, eight hardware
failure controls, and nineteen Source Photos materialization cases through the
public SDK and filesystem. Generated code is excluded from these numerators
and denominators. Final integrated repository checks, including contracts,
verification tools, Source tests and the later assertion fixes, remain pending.

Local documentation verification passed with renderer `07bbcec`: 99 HTML files,
82 OpenAPI operations, both bridge directions and ten guides, including visible
exact protobuf source, navigation, local/schema links and representative cURL
snippets. All 45 tracked documentation files were inventoried and historical
receipts separated from current work. The workflow-only update `4b5b467` retains
those renderer bytes and passed blocking action CI with all seven focused
JavaScript suites and the smoke build. It is the current library workflow pin.
Remote library publication is a separate gate.

The remaining final acceptance work is:

- Complete `make lint` and `make check` for the final SDK and public-pin CLI,
  including pinned Source tests, generation/module drift, source/model/socket
  controls, supported Go/OS builds and consumer/compatibility verification.
- Record fresh separate SDK and CLI replay, unit and combined coverage, with
  the required non-generated replay and combined thresholds and explicit gaps.
- Verify exact-commit blocking CI, including the Windows/Linux/macOS and Go
  1.25/1.26 matrix and documentation build in [PR 69](https://github.com/portpowered/go-icloud/pull/69).
- Verify deployed Pages navigation, external documentation and badge targets,
  coordinated SDK/CLI release workflows and tags, and isolated installation of
  the released versions. Feature pseudo-version availability does not establish
  that `@latest` supplies the completed workflows.
- Obtain both fresh independent complete-library reviews, disposition every
  original finding and verify fixes and CI at the final reviewed commit.

## Repository acceptance

- [x] Recover the interrupted library-discovery/session-response merge and run full baseline checks.
- [ ] Maintain a complete source operation and route/channel denominator, including active dependency traffic.
- [ ] Generate all known wire models, nested payloads, keys and fixed values and bind them to actual sends.
- [ ] Add blocking source/model/network provenance gates and the template's negative controls.
- [ ] Move transport behavior to `pkg/dependencies/<transport>` and include it in coverage (template item 7).
- [ ] Enforce formatting, tidy/generation drift, supported Go/OS builds, consumer and compatibility checks.
- [ ] Report separate SDK/CLI replay, unit and combined coverage with explicit uncovered behavior.
- [ ] Provide schema references, customer MDX guides, Pages publication, badges and whole-site verification.
- [ ] Consolidate stale milestone documentation and audit every published file for accuracy and audience.
- [ ] Verify coordinated SDK/CLI release workflows and isolated public module installation.
- [ ] Both independent reviewers verify every template item, findings and exact-commit CI at the final commit.

The initial independent gap audits were read-only and did not approve the current
checkout. Their historical findings concerned selected-library semantics,
authentication sockets, local Photos sync/watch, source/model gates, package
placement and site/release acceptance. Implementations now exist as listed above;
their findings still require explicit disposition by both final reviewers.
Final merge remains conditional on the complete acceptance above.
