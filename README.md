# go-icloud

This repository is being built into a standalone Go iCloud client using
`go-third-party-template`. The first stage supplies an instrumented Python
reference CLI to collect evidence before implementing the Go client.

Scope: Photos, devices/Find My, Drive, reminders, and account. Contacts and
calendar are outside this migration.

## Log in and record requests

From a local PowerShell terminal in this repository:

```powershell
.\setup-reference.ps1
.\icloud.ps1 login --username your-apple-id@example.com
.\icloud.ps1 status
.\icloud.ps1 account
.\icloud.ps1 devices
.\icloud.ps1 drive --limit 20
.\icloud.ps1 albums --limit 20
.\icloud.ps1 photos --limit 20
.\icloud.ps1 reminders --limit 20
.\icloud.ps1 reminder-zones
```

Login prompts for your regular Apple ID password and any required verification
code. Neither is saved. Commands reuse saved session tokens and cookies without
prompting for the password. If Apple expires the session, run login again.
Mainland China accounts use `login --china`.

On Windows, credentials, private paired HTTP captures, and `result.json` files
are under `%LOCALAPPDATA%\go-icloud\reference`, restricted to your Windows user.
The CLI prints the result directory and item counts rather than personal data.
Read IDs from the local result file to select a Drive folder (`--node`), photo
album (`--album`), or reminder list (`--list-id`). Limits cap returned items;
the reference may fetch a larger provider page internally.

These commands read account data. Find My refresh may request updated device
locations. The CLI does not expose account-data writes or device-control actions.
Login and session trust perform the authentication exchanges needed for access.

Live login, saved-session reuse, account/device reads, Drive listing, and Photos
listing/pagination have been exercised. The observed Reminders account returned
`ZONE_NOT_FOUND` for the reference's requested zone; this is a recorded failure,
not successful reminder-list coverage.
The Go SDK implements the five account reads: devices, family, member photos,
storage and plan summaries. The other selected services and native Go
authentication are still being built; the SDK
has not been released. See
[capture development notes](docs/reference-capture.md) for evidence status,
reference revisions, recording limitations, and verification commands.

## Go account devices

Use `github.com/portpowered/go-icloud/pkg/icloud`. Authentication context belongs
to each request. For now, supply the account origin, account/client identifiers
and cookie/web headers from your caller-owned authenticated session. The SDK
does not import the reference CLI's private credential files automatically.

```go
client, err := icloud.New() // optional icloud.WithHTTPTransport for offline replay
if err != nil {
    return err
}
result, err := client.GetAccountDevices(ctx, icloud.GetAccountDevicesRequest{
    Auth: auth, // icloud.AuthContext from the caller's authenticated session
})
if err != nil {
    return err
}
for _, device := range result.Devices {
    if device.Name != nil {
        fmt.Println(*device.Name)
    }
}
```

Each call fetches fresh devices. Empty results are successful. Device/payment
metadata and unknown JSON values are retained. Response headers, including
Set-Cookie, are returned for caller-owned session updates. A shared client stores
no credentials or cookie jar and supports concurrent requests for separate
accounts. Set deadlines on `ctx`; automatic redirects are disabled.

Use `errors.As` with `*icloud.ClientError` to inspect `Kind`, `StatusCode`,
`ResponseBody` and `ResponseHeaders`; `errors.Is` preserves original causes.
Display messages omit provider content and credentials. Raw response details may
contain private data. See [account contracts](docs/account-wire-contracts.md)
for replay scope and remaining work.

## Other account reads

Use the same caller-owned `auth` context and deadline on `ctx` for each request.
Set `auth.ChinaMainland` to a pointer to true for a mainland China account;
plan summaries use the corresponding gateway. Omission selects the global one.

```go
family, err := client.GetAccountFamily(ctx, icloud.GetAccountFamilyRequest{Auth: auth})
if err != nil {
    return err
}
for _, member := range family.Members {
    if member.Dsid.IsSpecified() && !member.Dsid.IsNull() {
        photo, err := client.GetAccountMemberPhoto(ctx, icloud.GetAccountMemberPhotoRequest{
            Auth: auth, MemberID: member.Dsid.MustGet(),
        })
        if err != nil {
            return err
        }
        _ = photo.Content // exact bytes; use photo.Metadata for its reported media type
    }
}
```

Family records retain optional/nullable values and unknown metadata. An omitted
family list returns an empty slice. Member IDs come from the family response.
An empty photo body succeeds; byte content is preserved independently of its label.

```go
storage, err := client.GetAccountStorage(ctx, icloud.GetAccountStorageRequest{Auth: auth})
if err != nil {
    return err
}
fmt.Println(storage.Usage.UsedStorageInBytes, storage.Usage.TotalStorageInBytes)
```

Storage includes optional quota flags and media categories. Zero totals and
usage above quota remain absolute byte counts; no undefined percentage is added.

```go
plan, err := client.GetAccountPlanSummary(ctx, icloud.GetAccountPlanSummaryRequest{Auth: auth})
if err != nil {
    return err
}
_ = plan.Summary // json.RawMessage; retains provider fields and JSON number precision
```

Every account read is fresh and returns response metadata. The typed error
inspection described above applies to all five methods. These methods are
verified with synthetic paired replay; native Go live integration is still pending.
