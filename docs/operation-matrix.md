# iCloud operation evidence

Capture session: 2026-10-08 UTC (2026-10-07 local). Source revisions are pinned
in `tools/reference/source.json`. These findings describe one observed account,
not universal Apple contracts. Private recordings are outside the repository
under `%LOCALAPPDATA%\go-icloud\reference\captures`. No credentials, account
identifiers, locations, filenames, or photo contents appear in this inventory.

LIB-05 and library standard 15 require both outbound request matching and
semantic result verification. Completed read scenarios now replay offline from
private initial-session snapshots, with network fallback forbidden. Initial
login has HTTP captures but lacks the trusted-device binary socket transcript;
do not treat it as complete authentication replay.

Verification receipt: 17 completed private scenarios, 95 paired exchanges,
matching semantic results/failures, and 29 passing synthetic/offline tests.
An additional 65 portable synthetic service scenarios check Drive, account,
Reminders, and Find My
request shapes and results. They remain separate from this live observation
matrix; see `tests/replay/fixtures/synthetic/http` for their evidence labels.
The receipt measures this capture batch, not complete service coverage.

| Area / behavior | Reference entry point | Observed exchange | Evidence / result |
| --- | --- | --- | --- |
| Initial authentication | constructor/authenticate, validate_2fa_code | authorize/signin, signin/init, signin/complete, auth discovery, bridge step/code validation, trust, accountLogin | Account-owner login succeeded; sign-in challenge includes HTTP 409, trust HTTP 204. Socket gap remains. |
| Saved-session setup | authenticate | POST setup `/setup/ws/1/validate`, body `null` | HTTP 200; trusted session reused without a stored password. Repeated offline replay succeeds. |
| Account storage | account.storage | POST discovered setup `/setup/ws/1/storageUsageInfo` | HTTP 200; usage/quota/media envelopes. Library wrapper projects `storage.usage`, not direct storage fields. |
| Account devices | account.devices | GET discovered setup `/setup/web/device/getDevices` | HTTP 200; one account device. Payment metadata remains private. |
| Find My initialization | devices | POST discovered Find My `/fmipservice/client/web/initClient` | HTTP 200; multiple devices and server context. The manager exposes a `devices` mapping, not a `values` method. |
| Find My refresh | AppleDevice.data | POST Find My `/fmipservice/client/web/refreshClient` | HTTP 200; server-context continuation captured. Property access can trigger a refresh when its monitor is not running. |
| Web access gate | Drive/Photos/reminders service properties | POST setup `/setup/ws/1/requestWebAccessState` | HTTP 200; observed access allowed. Include this service-instantiation exchange in replay. |
| Drive root | drive.root.get_children | POST Drive `/retrieveItemDetailsInFolders` | HTTP 200; multiple root entries. |
| Drive nested folder | get_node_data, DriveNode.get_children | Same endpoint, selected folder ID | HTTP 200; zero children. Empty list retained as a successful result. |
| Photos setup/albums | photos.albums | CloudKit Photos `/records/query` | HTTP 200; indexing check, album queries, and zero-record query responses. Built-in albums plus provider records returned. |
| Photo counts | album iteration | Photos `/internal/records/query/batch` | HTTP 200; count lookup occurs before some album iteration. |
| Photo listing | photos.all iteration | Photos `/records/query` | HTTP 200; many results. The 150-item selection crosses a provider page and carries continuation state. |
| Smart-album selection | photos.albums[ID] iteration | Photos `/records/query` with smart-album record types/filters | Successful single-item selections across several smart albums, including different query record types. |
| Empty photo albums | photos.albums[ID] iteration | Photos count lookup and `/records/query` | HTTP 200; zero-result iteration observed for multiple built-in albums and replayed offline. |
| Reminder lists | reminders.lists | CloudKit Reminders `/changes/zone` | HTTP 200 envelope with nested `ZONE_NOT_FOUND` for zone `Reminders`; reference raises RemindersApiError. Failure replays offline. |
| Reminder zone discovery | reminders raw client's zones_list | CloudKit Reminders `/zones/list` | HTTP 200; one zone returned, but the requested `Reminders` zone is absent. This does not establish why it is absent. |

## Gaps in this historical live capture batch

- Authentication: complete socket transcript, expiry/invalid-session transitions,
  wrong/expired code, session-trust rejection, concurrent accounts, and security
  key/legacy two-step behavior. Do not provoke lockouts to collect failures.
- Account and Find My: zero/one/many variants, missing location and partial
  device fields, stale server context, malformed responses, and error mapping.
  Sound, Lost Mode, messages, and erase operations were not invoked.
- Drive: populated nested folders, files, download/content exchanges, pagination
  if observed, missing resource and partial-result semantics. No writes occurred.
- Photos: exhausted pagination, downloads/resource
  variants, indexing-not-ready, missing master/asset relationships, and errors.
- Reminders: successful lists/reminders, empty/multiple lists, completion/due
  dates, continuation, and correct service discovery for a provisioned zone.
- All areas: labeled synthetic malformed responses, auth failures, throttling,
  provider failures, timeouts/cancellation, and oversize limits as applicable.

These gaps describe this capture batch, not current SDK implementation status.
Private captures require sanitation and provenance review before portable export.
See [completion matrix](completion-matrix.md) for current implementation acceptance.
