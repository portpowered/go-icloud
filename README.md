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
The Go client has not been implemented or released yet. See
[capture development notes](docs/reference-capture.md) for evidence status,
reference revisions, recording limitations, and verification commands.
