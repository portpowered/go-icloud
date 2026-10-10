# Documentation publishing

Customer material lives under `docs/guides/` as MDX. Contributor contracts,
fixture provenance, generation commands and reviewer evidence remain repository
Markdown and are excluded from the customer navigation (LIB-17, template 12–13).

The Pages workflow builds all OpenAPI operations and AsyncAPI channels with the shared Fumadocs action,
pinned to `ca7008e2b4de6e600ec5157f9adb015fafd54961`. Pull requests build
and check the same export used for main deployment. Configure GitHub Pages to use
GitHub Actions before the first deployment. Coverage HTML and badge JSON belong
under `coverage/` in the same Pages artifact.

The full-site gate is `go run ./tools/docsite`. It inventories every exported HTML
page, including the landing page and generated references, checks local links and
fragment destinations, inspects canonical schema `externalDocs` URLs, and checks
expected rendered text from `docs/site-expectations.json`. It also traverses
AsyncAPI `externalDocs` links, including nested channel documentation. Missing guide content
fails even when a fallback page exists. The gate is an automated aid; inspect
required fields, known variants, example values and request snippets visually
before final independent approval.

The renderer opts into JSON encoding for canonical `plain/text` and wildcard
request media aliases. The reference graph schema view preserves conjunctions,
alternatives, required fields, examples and named component links without eagerly
expanding recursive intersections. Native request snippets and the playground
remain available. The shared action commit must be published before GitHub can
resolve the workflow pin. Deployment, external link checks, final API guide
updates and both independent reviews remain release gates.

See [the documentation inventory](documentation-inventory.md), [release procedure](releasing.md),
and [completion matrix](completion-matrix.md).
