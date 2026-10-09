# Completion acceptance matrix

This is the current work list for finishing the selected-service migration.
The compatibility source is `tools/reference/source.json`'s live revision.
Captured evidence, synthetic reference scenarios, and historical milestones are
separate evidence classes (LIB-05, LIB-12). A route occurrence does not establish
operation parity. No row below grants final merge or release acceptance.

## Remaining operation ports

| Area | Required behavior | Acceptance |
| --- | --- | --- |
| Photos libraries | Discover and select primary, additional private, SharedSync and shared zones | Open |
| Photos albums | Create, rename, delete, add asset; revision and relation handling | Open |
| Photos assets | Favorite/unfavorite, delete, shared-library reconciliation | Open |
| Photos uploads | Reserve, transfer, register, status/hydration, duplicate and timeout behavior, optional album attachment | Open |
| Photos changes | Cursor discovery, zone/container pages, lookup, ordered changes and tombstones | Open |
| Photos legacy streams | Discover albums, count, enumerate, lookup and download | Open |
| Photos local sync/watch | Persistent state, materialization, selection/options, reconciliation, bounded cancellable watch | Open |
| Reminders | Create, update, soft delete; CRDT encoding and resolution tokens | Implemented; 11 paired scenarios, full local checks and two scoped reviews |
| Reminders relations | Location alarm/trigger, hashtags, URL/image attachment updates, recurrence rules; atomic links and revision updates | Open |
| Auth login | Native SRP s2k/s2k_fo, service one-factor paths, paused MFA and returned credentials | Open |
| Auth verification | Status, trusted devices, two-step codes, trusted-device/SMS two-factor codes, trust, hardware assertion | Open |
| Auth consent/logout | PCS/web consent polling, logout scopes, failure-stage rotations and credential ownership | Open |
| Auth sockets | Injected raw WebSocket, protobuf bootstrap/subscription/ACK, signing/proofs, modern/legacy bridge, teardown | Open |
| CLI | Public SDK workflows for all supported reads/writes, native login/verification/logout, cancellation and cleanup | Open |

For each port: bind source inputs/defaults and complete results/errors to named
schema models, execute the public Go operation against every applicable paired
scenario, assert full consumption, and add missing functional and negative
controls. Validate time, randomness and framed payload meaning explicitly;
never ignore fields to accommodate the port (SCHEMA-10, LIB-05).

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
checkout. Their findings include missing selected-library semantics, authentication
sockets, local Photos sync/watch, source/model gates, package placement and site/
release acceptance. Milestone commits may land on the implementation branch;
final merge remains conditional on the complete acceptance above.
