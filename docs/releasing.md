# Release requirements

No SDK or CLI release has been published. The repository currently provides
reference capture and replay tooling. Keep release readiness open until every
item in [the migration checklist](migration-checklist.md) is independently
verified under [the library standards](library-standards.md).

Before tagging, implement the public SDK under `pkg/icloud` and the separate CLI
module under `cmd/go-icloud`. Use `github.com/portpowered/go-icloud` for the SDK
module and `github.com/portpowered/go-icloud/cmd/go-icloud` for the CLI module.
Require blocking checks for both modules, generation drift, complete endpoint
and model provenance, strict paired replay, coverage, consumer builds, rendered
website verification, and two independent audits of the exact release commit.
`make check` currently verifies the bootstrap only; it does not prove these
pending SDK requirements.

The release workflow is still to be implemented. It must rerun the pinned
all-linters and complete verification gates on the tag commit, check public API
compatibility against the prior stable version, and validate public Go module
proxy downloads in an isolated consumer without local replacements. Publish the
SDK before the CLI version that depends on it. CLI nested-module tags use the
form `cmd/go-icloud/vMAJOR.MINOR.PATCH`; SDK tags use `vMAJOR.MINOR.PATCH`.

A public proxy release requires public repository access. Confirm repository
visibility as part of publication; the account owner made this repository public
on 2026-10-08. Public visibility is not evidence of a published SDK or CLI. Test
`go install github.com/portpowered/go-icloud/cmd/go-icloud@<released-version>`
from a clean environment and record that command in the published guide. Verify
release-note links against the deployed guides and generated reference before
closing the checklist. Never publish raw captures, credentials, or executables
from local exploration.
