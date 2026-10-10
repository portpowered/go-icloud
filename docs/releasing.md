# Release procedure

The public SDK and separate CLI module already exist. Development pseudo-versions
are available; a final release requires every item in the
[migration checklist](migration-checklist.md) and two independent reviews of the
exact release commit under the [library standards](library-standards.md).

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

Release notes should state supported customer workflows and material differences
from the reference. Link to the deployed authentication, Photos, Reminders and CLI
guides under `https://portpowered.github.io/go-icloud/docs/guides/`; verify their
content before publishing the notes. Do not include capture logs, credentials,
signed URLs or locally generated executables in release artifacts.
