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
| Auth login | Native SRP s2k/s2k_fo, service one-factor paths, paused MFA and returned credentials | Implemented; SDK race/replay passed at `bda83e1`; final integrated acceptance pending |
| Auth verification | Status, trusted devices, two-step codes, trusted-device/SMS two-factor codes, trust, hardware assertion | Implemented, including portable hardware adapter; SDK race/replay passed at `bda83e1`; final integrated acceptance pending |
| Auth consent/logout | PCS/web consent polling, logout scopes, failure-stage rotations and credential ownership | Implemented, including resumed consent policy; SDK race/replay passed at `bda83e1`; final integrated acceptance pending |
| Auth sockets | Injected raw WebSocket, protobuf bootstrap/subscription/ACK, signing/proofs, modern/legacy bridge, teardown | Implemented; 53 socket and 30 combined paired timelines plus lifecycle controls; integrated acceptance pending |
| CLI | Public SDK workflows for all supported reads/writes, native login/verification/logout, cancellation and cleanup | Implemented; published SDK `620a94af` pinned; exact-commit test and coverage receipts below; final public-pin checks and acceptance pending |

For each port: bind source inputs/defaults and complete results/errors to named
schema models, execute the public Go operation against every applicable paired
scenario, assert full consumption, and add missing functional and negative
controls. Validate time, randomness and framed payload meaning explicitly;
never ignore fields to accommodate the port (SCHEMA-10, LIB-05).

## Integration evidence and remaining blockers

The CLI at `5bd9f0a4` pins the published SDK
`v0.0.0-20261010102812-620a94af4ab3`, with no local replacement. Final checks
against this pin remain pending. The SDK includes
Source-paired fixes for adding a photo to an album without prior membership,
empty Photos change zones, exact nullable change cursors, and Reminders document
base64 decoding. Full SDK race/replay tests passed at `bda83e1`, affected schema
contracts passed at `a033a9d`, and all-enabled SDK lint passed at `0765205`.
Those are exact-commit baseline receipts; subsequent generated Photos and
Reminders body changes require fresh integrated checks.

The full `make check` baseline at `7586fc3` passed SDK and CLI race tests,
contracts, all 136 pinned Source tests, and the HTTP occurrence gate
(1,081 scenarios, 2,448 pairs, 76/76 routes). It failed the CLI replay coverage
threshold. Non-generated SDK coverage at that baseline was replay
10837/13021 (83.2%), unit 4395/13021 (33.8%), and combined 11235/13021 (86.3%).

The paired CLI suites now reside in `tests/replay` (GO-12, LIB-07), with new
Source-bound photo visitor, typed-input and private credential-export cases.
At isolated CLI test commit `3d32439`, replay coverage passed at 985/1225 (80.4%)
and combined coverage at 1013/1225 (82.7%); unit tests also passed with the
preceding public SDK pin. The complete public-pin CLI coverage command at
`f6d0c65`, using published SDK `0765205`, subsequently passed: replay 985/1225
(80.4%), unit 543/1225 (44.3%) and combined 1013/1225 (82.7%). Its command
package contributed 872/1085 replay and 900/1085 combined statements; saved-login
contributed 113/140 to each profile. The public CLI at `7d7d7c41`, pinned to
SDK `c111`, also passed all three profiles with those same totals. That CI run
failed only the Source recently-added-count expectation: one case expected 32
functions after an unrelated global replacement, while the measured value was
31/128. The `e638` correction passed focused Source checks. These CLI receipts
require a fresh run against the current SDK pin above. Generated files are excluded; test relocation did not
change the production denominator.

At the generated Photos baseline `03528225`, SDK replay coverage passed at
10901/13095 (83.2%). The complete SDK coverage command at runtime snapshot `b3de8aec` subsequently
passed against the frozen 13,122 non-generated statement denominator: replay
10932/13122 (83.3%), unit 4520/13122 (34.4%) and combined 11330/13122 (86.3%).
The replay and combined thresholds are 80%; the configured unit threshold is 0%.
These results exceed the enforced thresholds and remain below the preferred
90% coverage target. The web transport profiles are 86.5%/36.8%/88.6%, and
`pkg/icloud` profiles are 83.3%/18.8%/85.0%, in replay/unit/combined order.

The current HTTP corpus contains 1,092 scenarios and 2,467 pairs. Focused Source
checks for the new Photos and Reminders cases passed. The complete 139-test
pinned Source suite at `b3de8aec` passed in 372.252 seconds. Fresh final integrated
SDK/CLI gates and public-pin CLI profiles remain pending. The complete production
source/model/network audit and its blocking Make/CI integration remain open.
Focused proof controls passing does not establish actual-source conformance.

Local documentation verification passed with renderer `07bbcec`: 99 HTML files,
82 OpenAPI operations, both bridge directions and ten guides, including visible
exact protobuf source, navigation, local/schema links and representative cURL
snippets. The historical 45-file documentation baseline was inventoried and historical
receipts separated from current work. The workflow-only update `4b5b467` retains
those renderer bytes and passed blocking action CI with all seven focused
JavaScript suites and the smoke build. It was the library workflow pin for that receipt.
The current source inventory covers all 47 tracked Markdown/MDX files.
The downloaded documentation CI artifact at `7d7d7c41` passed all current page
expectations, local navigation/anchors, exact protobuf source and representative
request variant/snippet checks. Inspection exposed a response documentation gap:
known response fields, types, requiredness and alternatives are not presented as
a schema graph. The renderer repair at `bc213a7` passes focused canonical response SSR controls
and [blocking action CI](https://github.com/portpowered/api-docs-website-github-action/actions/runs/38046433431).
It is the current library workflow pin; the final library export remains pending. Response
discovery remains a template item 4 blocker until that proof; see
[the rendered inspection receipt](website.md). Final rendering and remote
library publication are separate gates.

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
