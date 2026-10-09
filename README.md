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
storage and plan summaries, plus Drive node/application-library reads and folder/item mutations. The other selected services and native Go
authentication are still being built; the SDK
has not been released. See
[capture development notes](docs/reference-capture.md) for evidence status,
reference revisions, recording limitations, and verification commands.
Generated [Find My wire contracts](docs/findmy-wire-contracts.md) now cover the
reference exchanges, and the [internal transport](docs/findmy-transport.md) replays
all 140 pairs. The [public Find My session](docs/findmy-session.md) now executes all 71 current
portable scenarios, including device descriptions and timed background refresh.

## Supply session cookies

```go
auth.Cookies = []icloud.AuthCookie{{
    Name: "session", Value: sessionCookie,
    Domain: "icloud.com", Path: "/", Secure: true,
}}
```

Set `HostOnly: true` when a cookie belongs only to the exact supplied host.
Leave `Domain` empty only when binding to the operation's first request host.
Keep credentials in your private session store.

## Go account devices

The separate Go CLI module reads account data and resumes an existing saved
login. Import the Python reference login once, then reuse the resulting private
Go session file. Password login and MFA completion are still pending, and live
Go account access has not yet been verified.

```powershell
cd cmd/go-icloud
go run . --help
go run . --reference-state "$env:LOCALAPPDATA/go-icloud/reference" --save-session C:/private/icloud-session.json resume
go run . --session C:/private/icloud-session.json account-devices
go run . --session C:/private/icloud-session.json resume
go run . --session C:/private/icloud-auth.json account-devices
go run . --session C:/private/icloud-auth.json account-family
go run . --session C:/private/icloud-auth.json account-storage
go run . --session C:/private/icloud-auth.json account-plan
go run . --session C:/private/icloud-auth.json drive-libraries
go run . --session C:/private/icloud-auth.json --node YOUR_NODE_ID drive-node
go run . --session C:/private/icloud-auth.json --family findmy
```

`resume` saves a private `ResumeSessionResult` file and prints only its path and
authentication flags. Without `--save-session`, reference import writes
`go-session.json` beside the original account files; native resume updates its
input Go session file. The original reference session and cookie files remain
unchanged. Windows credential files are restricted to the current user; on other
platforms they use mode `0600`. Use an existing protected directory for storage.

Read commands also accept a private `AuthContext` JSON file. This JSON uses
the public field names, including `accountID`,
`clientID`, service URLs, `headers`, and `cookies`. Keep this file private and out
of Git. Commands print service results as JSON, omit response authentication
metadata, and use a configurable `--timeout` (default one minute). Find My closes
its session after returning the initial device list. Photos, Reminders and write
operations are not yet CLI commands.

CLI replay tests use the root canonical synthetic fixture tree and public SDK.
Coverage for every handwritten CLI package under `internal/` is measured
separately from SDK coverage; the
process entry point is outside that measurement. `make check` verifies both Go
modules, including their pinned all-linter checks and race tests.

The SDK can now validate cookies and refresh a saved web session token with
`ResumeSession`. Supply `SavedSessionCredentials`, the saved account country and
trust token. The returned authentication context includes discovered Account,
Drive and Find My URLs and copied cookie/token updates:

```go
resumed, err := client.ResumeSession(ctx, icloud.ResumeSessionRequest{
    Auth: savedCredentials,
    AccountCountryCode: savedCountry,
    TrustToken: savedTrustToken,
    ForceRefresh: false,
    AllowUntrusted: false,
})
if err != nil {
    return err
}
auth := resumed.Auth // credentials: persist privately and do not print
```

`authentication-required` requests interactive login; `terms-required` requests
terms acceptance. This operation performs neither. `AllowUntrusted` can return
paused MFA discovery. The CLI imports existing reference credentials and resumes
native session files; password login and MFA completion remain pending. Native
session reuse has offline replay evidence and has not yet been verified against
a live account. If the reference login has no stored client ID or session token,
run the reference login command again before importing it.

Use `github.com/portpowered/go-icloud/pkg/icloud`. Authentication context belongs
to each request. For now, supply the account origin, account/client identifiers
and cookie/web headers from your caller-owned authenticated session. Supply structured cookies in `AuthContext.Cookies` when requests should apply provider cookie updates between stages. The SDK
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
accounts. Each operation copies structured cookies into a temporary jar. Updates
apply within that operation; subsequent calls use the cookies you supply again.
`ResponseMetadata.CookieScopeURL` supplies the issuer origin and escaped path
without query credentials for saving Set-Cookie updates. For a host-only update,
set `AuthCookie.Domain` to that URL host and `HostOnly: true`; derive an omitted
cookie path from the response request path. An explicit Cookie header takes
precedence over structured cookies. Set deadlines on `ctx`; automatic redirects are disabled.

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

## Go Drive reads

Supply `auth.DriveServiceURL` from authenticated service discovery, together with
the account/client identifiers and web headers. Each call fetches fresh data.
An account-service origin is unnecessary for these Drive requests.

```go
node, err := client.GetDriveNode(ctx, icloud.GetDriveNodeRequest{
    Auth: auth, NodeID: "FOLDER::com.apple.CloudDocs::root", // provider node ID
})
if err != nil {
    return err
}
if node.Node.Items != nil {
    for _, child := range *node.Node.Items {
        if child.Name != nil {
            fmt.Println(*child.Name)
        }
    }
}
```

A missing `Items` pointer means the provider omitted folder contents; an empty
slice means it returned no children. For a shared node, supply the provider's
sharing descriptor in `ShareID`. An empty descriptor is omitted as in the
reference. Optional node fields and unknown metadata remain available, including
recursive children, nulls and large JSON numbers.

```go
libraries, err := client.ListDriveLibraries(ctx, icloud.ListDriveLibrariesRequest{Auth: auth})
if err != nil {
    return err
}
_ = libraries.Libraries // an empty slice succeeds
_ = libraries.AdditionalMetadata // unknown envelope values
```

Both methods return response metadata and the typed errors described above.
Nine synthetic paired scenarios verify these two reads against the pinned
reference. The explicit Drive session also covers navigation, caching and uploads;
see [Drive contracts](docs/drive-wire-contracts.md).

## Go Drive downloads

Set `auth.DriveDocumentServiceURL` to the document-service origin returned by
authenticated discovery. Supply the document identifier from node metadata;
it differs from the Drive node identifier. Pass the node's zone when available;
omission uses the reference private document zone.

```go
download, err := client.DownloadDriveFile(ctx, icloud.DownloadDriveFileRequest{
    Auth: auth, DocumentID: documentID, Zone: &zone,
})
if err != nil {
    return err
}
_ = download.Content // exact bytes, including an empty file
_ = download.TokenMetadata // token response, including authentication updates
_ = download.Metadata // content response
```

The SDK selects the provider's data URL before its package URL and preserves
escaped paths and repeated provider query values. The content request reuses
your supplied headers. Structured cookies apply intermediate Set-Cookie updates
through an operation-local jar; explicit Cookie headers retain precedence.
Both response metadata objects include the cookie issuer origin/path.
If a later exchange fails, inspect `ClientError.PriorResponses()` for earlier
response metadata as well as its current response headers and body.
Thirteen synthetic paired scenarios verify download results, failures and cookie
rotation/scoping. Account-session persistence remains caller-owned.

## Go Drive uploads

Supply the document-service URL in `auth.DriveDocumentServiceURL` and the
caller-owned `X-APPLE-WEBAUTH-VALIDATE` structured cookie in `auth.Cookies`.
Preparation extracts its upload token before making a request.

```go
file, err := os.Open(filename)
if err != nil {
    return err
}
defer file.Close() // the caller owns this file
uploaded, err := client.UploadDriveFile(ctx, icloud.UploadDriveFileRequest{
    Auth: auth, ParentID: parentID, Filename: file.Name(), Content: file,
})
if err != nil {
    return err
}
_ = uploaded.DocumentID
_ = uploaded.UploadToken // secret caller-owned session parameter
_ = uploaded.UploadedFile
_ = uploaded.PreparationMetadata
_ = uploaded.TransferMetadata
_ = uploaded.RegistrationMetadata
```

Content must implement `io.ReadSeeker`. The declared size is the whole file;
bytes are transferred from the current cursor. An empty file is supported.
The SDK leaves the reader open and at its resulting cursor, including on failure.
Filename determines the MIME type and multipart field; registration uses the
host platform's filename basename. Omitted zone uses the private document zone.
Optional `ModificationTime` and `CreationTime` override the client clock;
`icloud.WithClock` injects a concurrency-safe clock for offline tests.

The SDK prepares the upload, transfers to the provider-issued URL without
appending document-service parameters, then registers the returned checksums
and key. Incomplete preparation or receipt replies stop before the next write.
It preserves 2xx statuses, receipt/registration metadata and unknown fields.
Acknowledgement does not prove background processing has completed. Uploads
are never retried automatically. `ClientError.PriorResponses()` retains earlier
stage responses; `UploadToken()` retains the extracted secret on a failed stage.
The shared client never saves account parameters or cookies. Keep returned
updates in your private session store. `DriveSession` carries those updates
into later operations within its account-bound lifecycle.

Nine synthetic service scenarios bind happy, empty, cursor/zone and stage-refusal
behavior through the public SDK; focused controls reject incomplete provider
replies before another write. These tests perform no live uploads.

## Go Drive navigation sessions

Open one session per account with the authenticated Drive origins. Origins must
be bare HTTPS origins without paths or queries. The opening context bounds the
session lifetime; `Close` cancels current and queued work and can be repeated.
The shared client remains reusable across accounts.

```go
session, err := client.OpenDriveSession(ctx, icloud.OpenDriveSessionRequest{Auth: auth})
if err != nil {
    return err
}
defer session.Close()
root, err := session.Root(ctx, icloud.DriveLocationRequest{})
if err != nil {
    return err
}
children, err := root.Children(ctx, icloud.DriveChildrenRequest{})
if err != nil {
    return err
}
for _, entry := range children.Entries {
    snapshot, err := entry.Snapshot()
    if err != nil {
        return err
    }
    _ = snapshot.Name
    _ = snapshot.Data // copied metadata, including provider identifiers
}
_ = session.Authentication() // copied secrets; persist privately if needed
_ = session.LastResponses() // copied evidence from the latest network operation
```

`session.Trash` opens the trash root. `Root` and `Trash` accept
`DriveLocationRequest{Refresh: true}` to replace the cached root; an earlier
entry pointer keeps its earlier state. `Children` accepts
`DriveChildrenRequest{Force: true}` to refresh a folder's metadata and children.
`session.Lookup` and `entry.Lookup` accept `DriveLookupRequest{Name: name}` for
exact display-name lookup. `session.Directory` and `entry.Directory` accept
`DriveEntryRequest{}` and return ordered names. File navigation returns a typed
`NotDirectory` error; a missing child returns `NotFound`.

Entries expose `CreateFolder(ctx, DriveFolderRequest{Name: name})`,
`Rename(ctx, DriveRenameRequest{Name: name})`, and `Trash`, `Delete`, `Restore`
and `PermanentlyDelete` with `DriveEntryRequest{}`. Recovery and permanent
deletion require recorded trash metadata and otherwise return `NotInTrash`.
Acknowledgements preserve cached node data; request a refresh for fresh state.
These operations do not retry writes automatically.

`entry.Download(ctx, DriveEntryRequest{})` derives the document ID and zone
from its node. A recorded numeric zero size returns empty content with
`LocalEmpty: true` and absent HTTP metadata. `entry.Upload(ctx,
DriveUploadRequest{Filename: filename, Content: reader})` uses the folder's
document ID and zone; the caller owns the seekable reader. The session retains
cookie and upload-token updates, including updates received before a failure.
Cached calls and local failures leave `LastResponses` unchanged.

Computed date properties preserve the pinned reference's offset arithmetic,
including its negative fractional-hour offset quirk. `snapshot.Data` retains
the parsed provider timestamps for callers that need standard UTC conversion.
Thirty-two node/session scenarios plus forty-seven service scenarios exercise
all 79 portable Drive cases through the SDK. The wider migration remains open.

## Go Drive changes

Supply the Drive authentication context described above. For item changes, pass
only the node identifier and its current version token from a fresh listing:

```go
selected := icloud.DriveNodeSelector{NodeID: nodeID, ETag: etag}
created, err := client.CreateDriveFolder(ctx, icloud.CreateDriveFolderRequest{
    Auth: auth, ParentID: parentID, Name: "New folder",
})
if err != nil {
    return err
}
_ = created.Folders
```

```go
renamed, err := client.RenameDriveNode(ctx, icloud.RenameDriveNodeRequest{
    Auth: auth, Node: selected, Name: "New name.txt",
})
if err != nil {
    return err
}
_ = renamed.Items
```

```go
moved, err := client.MoveDriveNodes(ctx, icloud.MoveDriveNodesRequest{
    Auth: auth, DestinationID: destinationID, Nodes: []icloud.DriveNodeSelector{selected},
})
if err != nil {
    return err
}
_ = moved.Items // an empty selection is also valid
```

```go
trashed, err := client.TrashDriveNode(ctx, icloud.TrashDriveNodeRequest{Auth: auth, Node: selected})
if err != nil {
    return err
}
_ = trashed.Items
```

Read a fresh trash entry and its version token before restoring or permanently
deleting it. Ordinary deletion and permanent trash deletion are distinct calls:

```go
restored, err := client.RestoreDriveNode(ctx, icloud.RestoreDriveNodeRequest{Auth: auth, Node: trashEntry})
if err != nil {
    return err
}
_ = restored.Items
```

```go
deleted, err := client.DeleteDriveNode(ctx, icloud.DeleteDriveNodeRequest{Auth: auth, Node: selected})
if err != nil {
    return err
}
_ = deleted.Items
```

```go
permanent, err := client.PermanentlyDeleteDriveNode(ctx, icloud.PermanentlyDeleteDriveNodeRequest{
    Auth: auth, Node: trashEntry,
})
if err != nil {
    return err
}
_ = permanent.Items
```

Results acknowledge the provider request; background movement may still be
processing. The SDK does not automatically retry these changes. Each result
includes response metadata and unknown provider fields. A missing item/folder
list remains distinct from an empty list. Sixteen synthetic paired scenarios
verify these calls; no live account writes were performed for this port.

## Find My sessions

Supply `AuthContext.FindMyServiceURL` from authenticated service discovery,
account/client IDs, and caller-owned cookies. Remote erase additionally needs
`SetupServiceURL` and the secret `SessionToken`. Authentication precedes service
operations; the native Go login flow remains in development.

`OpenFindMySession` discovers devices, optionally includes family devices,
and owns one background refresh loop. Keep its opening context alive until
finished and defer `Close`. Set `WithFindMyMonitorInterval(0)` for manual refresh.
The default interval is five minutes; `WithFindMyFamilyPolling` changes the
500-millisecond wait and five-retry readiness limit.

```go
session, err := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{
    Auth: auth, IncludeFamily: true,
}, icloud.WithFindMyMonitorInterval(0))
if err != nil { return err }
defer session.Close()

cached, err := session.Snapshot() // Copied cache; no request.
if err != nil { return err }
updated, err := session.Refresh(ctx, icloud.RefreshFindMyRequest{Locate: true})
if err != nil { return err }
_ = cached
_ = updated
```

`PlaySound`, `SendMessage`, `MarkLost`, and `Erase` accept a discovered device ID
and check its advertised capabilities. Invoke only the action you intend:

```go
sound, err := session.PlaySound(ctx, icloud.FindMySoundRequest{DeviceID: deviceID})
message, err := session.SendMessage(ctx, icloud.FindMyMessageRequest{
    DeviceID: deviceID, Text: &text, Sound: true,
})
lost, err := session.MarkLost(ctx, icloud.FindMyLostRequest{
    DeviceID: deviceID, PhoneNumber: contactNumber, Text: &text,
})
erased, err := session.Erase(ctx, icloud.FindMyEraseRequest{DeviceID: deviceID})
```

Check each call's error before proceeding. Command results contain response
metadata and acknowledgement bytes; they do not establish completed device
behavior. Commands are sent once. These command examples and replays are
synthetic; live exploration has performed reads only.

`Authentication()` returns copied session cookies and credentials for caller
storage. `LastResponses()` returns copied HTTP evidence, `LastError()` reports
the latest refresh/command/monitor failure, and `MonitorDone()` signals monitor
termination. `Close()` cancels active requests, queued calls, and waits;
`Snapshot()` remains available afterward. See the [session guide](docs/findmy-session.md)
for failure, scheduling, and remaining coverage limits.

### Find My device descriptions

`DescribeDevice` reads the copied session cache, including after Close. It returns
metadata strings, advertised capabilities, available location, the raw device,
and the reference status subset. Missing metadata strings default to empty;
missing status fields become explicit null. A location is available only when
LOC is true and the device contains a non-null location. An empty location object
remains available. Additional status fields preserve unknown JSON and large
integers without rounding.

```go
description, err := session.DescribeDevice(icloud.FindMyDeviceDescriptionRequest{
    DeviceID: deviceID, AdditionalStatus: []string{"future"},
})
if err != nil { return err }
_ = description.Capabilities.Location
_ = description.Location
_ = description.Status
```

Call `Refresh` explicitly before describing a device when fresh data is needed.
The [behavior replay notes](docs/findmy-behavior.md) explain the reference getter
and timed-monitor cases and the remaining acceptance work.

The [Reminders wire contracts](docs/reminders-wire-contracts.md) bind its 78
reference scenarios/86 paired exchanges to six active operations and shared
generated CloudKit models. Public Reminders service orchestration and semantic
SDK replay remain work in progress; these contract checks are a separate layer.
