# Documentation publishing

Customer material lives under `docs/guides/` as MDX. Contributor contracts,
fixture provenance, generation commands and reviewer evidence remain repository
Markdown and are excluded from the customer navigation (LIB-17, template 12–13).

The Pages workflow builds all OpenAPI operations and AsyncAPI channels with the shared Fumadocs action,
pinned to `bc213a78372cbe8200ebd442103b454cbcf55966`. Pull requests build
and check the same export used for main deployment. GitHub Pages is configured
to use GitHub Actions; its `github-pages` environment permits the `main` branch.
Coverage HTML and badge JSON belong
under `coverage/` in the same Pages artifact.

The full-site gate is `go run ./tools/docsite`. It inventories every exported HTML
page, including the landing page and generated references, checks local links and
fragment destinations, inspects canonical schema `externalDocs` URLs, and checks
expected rendered text from `docs/site-expectations.json`. It also traverses
AsyncAPI `externalDocs` links, including nested channel documentation. Missing guide content
fails even when a fallback page exists. The gate is an automated aid; inspect
required fields, known variants, example values and request snippets visually
before final independent approval.

The renderer derives AsyncAPI 3 presentation documents from canonical AsyncAPI 2
without changing the source schemas. Publish maps to application send and
subscribe to receive. Server variables, channel parameters and extension links
remain available. Non-JSON payloads display their exact local source and original
schema format rather than an invented JSON model. Unsupported lossy adaptations
fail the build. Both generation and runtime consume the same derived documents.

The renderer opts into JSON encoding for canonical `plain/text`,
`text/plain;charset=UTF-8` and wildcard request media aliases. Logout retains its
exact parameterized Content-Type and JSON request fields in the rendered cURL
example. The reference graph schema view preserves conjunctions,
alternatives, required fields, examples and named component links without eagerly
expanding recursive intersections. Native request snippets and the playground
remain available. The pinned shared action commit is published and available to
the workflow. Deployment, external link checks and both independent
reviews remain release gates.

See [the documentation inventory](documentation-inventory.md), [release procedure](releasing.md),
and [completion matrix](completion-matrix.md).

The historical local full export using renderer commit
`6f7f713fefae28b2526f140b1cfe0f52c2980e0d` passed Next.js and TypeScript
compilation and the complete site gate: 99 HTML files, including 82 OpenAPI
operations, both bridge directions and ten customer guides. The bridge checks
require visible SEND/RECEIVE labels, the original schema format and concrete
protobuf fields; an additional source comparison verified both rendered code
blocks against the exact canonical protobuf bytes. Customer navigation links and
representative Photos and Reminders cURL snippets were checked. This is local
rendering evidence, not deployment or independent acceptance evidence.

The Windows local exporter in pinned Next.js 16.3.6 writes nested segment-prefetch
filenames while its browser client requests dotted filenames. Local preview may
therefore log prefetch 404s and fall back to document navigation. A browser click
between the bridge pages still loaded the correct direction and source panel.
The publishing workflow runs on Linux. At library commit `96d2aab`,
[documentation CI](https://github.com/portpowered/go-icloud/actions/runs/38033411380)
passed the complete remote export and site checks. The export contains 100 HTML
pages, including the coverage report, and preserves the exact Logout header/body
expectations. This receipt applies to that commit; later changes require their
own exact-commit checks.

As of 2026-10-10, no Pages deployment has run and the public landing, guide,
coverage HTML and badge JSON URLs return HTTP 404. Pull requests build and upload
the checked artifact but deliberately skip deployment. The supported deployment
path is a push to `main`, or `workflow_dispatch` on `main` once the workflow is
present there. A dispatch on a feature branch also skips deployment. After the
reviewed merge, verify the main deployment, all published navigation and anchors,
the coverage badge and public README destinations before recording publication
as complete. The renderer's draft
[PR 5](https://github.com/portpowered/api-docs-website-github-action/pull/5)
remains separate from library publication and requires its own reviewed merge.

## Rendered artifact inspection at `7d7d7c41`

The successful [documentation run 38043226718](https://github.com/portpowered/go-icloud/actions/runs/38043226718)
built library commit `7d7d7c41498febacc78555a0af853e16d2a9f6e1`. Inspection of its
downloaded `api-docs` artifact found 100 HTML pages: 82 OpenAPI operations, both
bridge directions and ten customer guides, with the coverage report and landing
pages. All 96 configured page expectations appeared in article content after
excluding scripts/styles, and 7,342 local navigation links and anchors resolved.
Both bridge source blocks matched the canonical protobuf exactly.

The request reference exposes nested query field alternatives, requiredness,
fixed values and examples; all six Photos mutation request alternatives and
their examples are present. SRP requests show `s2k`/`s2k_fo`, and the Logout
cURL example preserves `text/plain;charset=UTF-8` with its two JSON fields.
Representative Photos, SRP, Logout and Reminders request snippets are present.
The ten guide articles remain customer usage flows; contributor verification and
review records stay outside their navigation.

The same inspection found a template item 4 blocker: response sections provide
status/media selections and example JSON but omit schema field graphs. In
particular, `PhotosQueryRecords` does not expose the record/tombstone/error
alternatives, `PhotosUploadStatus` does not explain progress/error types and
requiredness, and `GetAuthChallenge` does not expose nested nullable/alternative
contracts. Example bodies alone do not establish complete response discovery.
The renderer repair and final integrated artifact must verify those contracts,
including non-success responses, without changing canonical schema semantics.
This scoped artifact inspection does not certify final rendering, interactive
variant selection, Pages deployment or independent acceptance.

The repair is published at renderer `bc213a78372cbe8200ebd442103b454cbcf55966`
and passed [blocking action CI](https://github.com/portpowered/api-docs-website-github-action/actions/runs/38046433431),
including the Next/TypeScript smoke export. Focused SSR through the pinned
Fumadocs public operation hook checks the actual canonical Photos query
record/tombstone/error alternatives, required error codes and nullable reason,
upload progress/error fields and typed 429 responses, and authentication nested
alternatives/default errors. All response statuses, media and header schemas
render outside collapsed selectors while the request graph, playground and
request/response example slots remain. Both library workflows now pin this exact
repair. The final library artifact must still verify those graphs together with
the existing examples, snippets, exact payloads and navigation; the earlier
`7d7d7c41` artifact cannot prove the repaired rendering.

## Response graph artifact receipt at `3f0cb80e`

The exact [documentation run 38046643223](https://github.com/portpowered/go-icloud/actions/runs/38046643223)
passed with renderer `bc213a7` at library commit
`3f0cb80e48a331d016e4a37dccb55a026cdf666b`. Its downloaded artifact contains
100 HTML pages, including all 82 operations, both bridge directions and ten
customer guides. Inspection excluding script/style content passed all 96 page
expectations, 7,342 local links/anchors, unique IDs throughout the export and
both exact canonical protobuf source blocks. The existing request variant and
representative cURL checks also passed.

Response-only table inspection confirms the repaired contracts: Photos query
records expose the record/tombstone/error alternatives, a required string
`serverErrorCode` and nullable `reason`; upload status exposes integer-or-null
progress/error codes and its typed 429 response; authentication exposes nested
fields, string/integer `sourceAppId` alternatives, nullable values and the
default failure model. The existing JSON response examples remain in separate
code blocks for all three pages. The checks distinguish response graph rows
from request data and serialized script content.

This receipt verifies the observed response graph omission is repaired in that
export. It does not award independent standards approval or establish Pages
deployment. New schemas, scenarios, models or renderer changes still require
exact-commit checks and a fresh artifact inspection before final acceptance.
