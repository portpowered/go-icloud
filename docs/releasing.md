# Release procedure

The public SDK and separate CLI module already exist. Development pseudo-versions
are available; a final release requires every item in the
[migration checklist](migration-checklist.md) and two independent reviews of the
exact release commit under the [library standards](library-standards.md).

A reviewed library merge and a stable release have separate receipts. Before
merge, verify the final public SDK pseudo-version and the CLI module's exact SDK
pin without a workspace or replacement, pass blocking CI and both independent
reviews, and verify the complete documentation artifact and Pages workflow
configuration. The normal main push then deploys Pages; check the actual site,
navigation and coverage destinations after deployment. A passing pull-request
artifact does not establish that those URLs are already live. No stable SDK or
nested CLI tag is created by merging the implementation.

Before tagging:

1. Run blocking `make lint` and `make check` for the SDK and CLI, including generated
   drift, paired replay, race checks, coverage and separate consumer builds.
2. Verify the complete source/model/network inventories and their negative controls.
   Coverage percentages do not establish complete operation or transport coverage.
3. Build and inspect the rendered [Pages documentation](website.md). Check every
   customer guide against the current exported SDK and CLI, every reference variant,
   external destinations and the actual coverage/release badge destinations.
4. Have both independent reviewers disposition every finding on the final commit
   and confirm its blocking CI. Keep unresolved checklist items open.

Publish SDK tags as `vMAJOR.MINOR.PATCH`. Publish the SDK before the CLI version that
depends on it, then publish the nested module as
`cmd/go-icloud/vMAJOR.MINOR.PATCH`. Verify proxy downloads and
`go install github.com/portpowered/go-icloud/cmd/go-icloud@<version>` from an isolated
consumer without local replacements. Check public API compatibility against the
previous stable version when one exists.

The tag-triggered [release workflow](../.github/workflows/release.yml) accepts SDK
`v*` tags and nested `cmd/go-icloud/v*` tags. It checks the exact tagged commit,
reruns pinned generation, the default source/model/network gate, `make check`
(including all-enabled pinned lint and separate coverage), every public package's
API compatibility, the minimum and current Go versions on all three operating
systems, and the complete rendered documentation. Publication depends on all jobs.

An SDK tag also installs the candidate CLI at that exact commit through the public
proxy. After the SDK GitHub release succeeds, update the CLI's `go.mod` to that
published SDK version, tidy its module, repeat the reviews/checks, and tag the CLI.
The CLI release gate requires its SDK release to exist and rejects unreleased SDK
source changes in the CLI tag. A stable CLI must depend on a stable SDK. Clean
Windows, Linux and macOS consumers download the SDK and install the CLI without a
workspace, replacement or shared module cache; their installed binary must record
the declared SDK dependency. SDK and CLI versions may advance independently.

API comparison inventories the union of public `pkg/` packages in the baseline
and release trees, so a deleted dependency-model or transport package remains a
breaking change. New public packages are additive. Before v1, a minor version may
break compatibility; patch releases and stable minor releases must preserve it.
The workflow currently handles the unversioned v0/v1 module paths. A v2 release
requires updating the SDK and CLI module paths and release configuration first.

Release notes should state supported customer workflows and material differences
from the reference. Link to the deployed authentication, Photos, Reminders and CLI
guides under `https://portpowered.github.io/go-icloud/docs/guides/`; verify their
content before publishing the notes. Do not include capture logs, credentials,
signed URLs or locally generated executables in release artifacts.
