# Documentation website requirements

The documentation website and its publishing workflow are pending. No generated
API reference or Pages deployment exists yet. The current CI workflow verifies
the reference capture bootstrap only.

Implement the website with the shared Fumadocs documentation action in
`portpowered/api-docs-website-github-action`. Select and pin a reviewed action
revision when the workflow is implemented. Generate references from canonical,
responsibility-specific iCloud schemas; do not use the template's widget schema.
Mark implementation-derived contracts and synthetic examples accurately.

Write customer-facing MDX guides under `docs/guides/` for authentication, saved
session lifecycle, each supported service, errors, caller configuration, and CLI
installation and workflows. Keep contributor inventories, migration notes,
coverage mechanics, and reviewer evidence in repository Markdown. Link guides
to matching generated references rather than duplicating wire definitions.

Build the whole site in pull request CI and publish reviewed `main` commits to
GitHub Pages. Include the non-generated SDK coverage HTML report and badge JSON
in that deployment. The currently available Python reference measurement is a
separate contributor tool and is not the public SDK coverage report.

Before release, inspect rendered fields, discriminated variants, complete
schema-valid examples, and generated request snippets. Audit every internal
link from every rendered page, including the root and generated reference;
verify runtime schema links such as `externalDocs`, external destinations, and
release-note links. Verify destination content rather than relying on HTTP 200.
Keep [the migration checklist](migration-checklist.md) open until deployment and
both independent reviewer audits establish these requirements.
