# go-icloud

[![Go](https://img.shields.io/github/go-mod/go-version/portpowered/go-icloud)](go.mod)
[![CI](https://github.com/portpowered/go-icloud/actions/workflows/ci.yml/badge.svg)](https://github.com/portpowered/go-icloud/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fportpowered.github.io%2Fgo-icloud%2Fcoverage%2Fbadge.json)](https://portpowered.github.io/go-icloud/coverage/)
[![Release](https://img.shields.io/github/v/release/portpowered/go-icloud)](https://github.com/portpowered/go-icloud/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/portpowered/go-icloud/pkg/icloud.svg)](https://pkg.go.dev/github.com/portpowered/go-icloud/pkg/icloud)
[![License](https://img.shields.io/github/license/portpowered/go-icloud)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://portpowered.github.io/go-icloud/docs/guides/)

A Go iCloud client for account information, Find My devices, Drive, Photos, and
Reminders. Install the public SDK with Go 1.25 or newer:

The pinned security-key HID backend supports Windows, Linux and macOS without
a cgo toolchain and sets this minimum version.

```sh
go get github.com/portpowered/go-icloud/pkg/icloud@v0.0.0-20261010112114-c49557a2ffca
```

This development version contains the APIs described below. Final conformance
and a stable release remain pending; `@latest` may select an earlier release.

Authenticate with caller-owned credentials before selecting a discovered service:

```go
client, err := icloud.New()
if err != nil { return err }
login, err := client.Authenticate(ctx, icloud.AuthenticateRequest{
    AccountName: accountName, Password: password,
})
if err != nil { return err }
if login.State.RequiresMFA {
    return errors.New("complete account verification before service calls")
}
devices, err := client.GetAccountDevices(ctx, icloud.GetAccountDevicesRequest{
    Auth: login.State.Auth,
})
if err != nil { return err }
_ = devices.Devices
```

Keep the complete returned `NativeAuthState` in your private store and persist
updates explicitly. The authentication guide covers SMS, trusted-device, two-step
and security-key verification. `ResumeSession` remains available for saved web
credentials.
One reusable client can serve several accounts; each request supplies its own
authentication. Use contexts for deadlines and cancellation. Drive and Find My
sessions own their account state and must be closed when finished.
`WithHTTPTransport`, `WithClock`, and `WithRandomSource` support caller configuration
and deterministic offline operation.

Customer guides explain [authentication and renewal](https://portpowered.github.io/go-icloud/docs/guides/authentication),
[account reads](https://portpowered.github.io/go-icloud/docs/guides/account),
[Find My](https://portpowered.github.io/go-icloud/docs/guides/findmy),
[Drive](https://portpowered.github.io/go-icloud/docs/guides/drive),
[Photos](https://portpowered.github.io/go-icloud/docs/guides/photos),
[Reminders](https://portpowered.github.io/go-icloud/docs/guides/reminders),
[configuration and errors](https://portpowered.github.io/go-icloud/docs/guides/configuration),
and the [standalone CLI](https://portpowered.github.io/go-icloud/docs/guides/cli).

Pages publication is pending. Until deployment, read the same customer guides
in [the repository](docs/guides/index.mdx).

Synthetic paired replay establishes
compatibility with the pinned Python reference; it does not establish successful
live operation for every account. See the contributor [completion matrix](docs/completion-matrix.md)
for remaining implementation and release acceptance work.
