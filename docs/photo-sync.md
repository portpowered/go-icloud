# Local Photos synchronization

`pkg/photosync` implements the pinned Python `photos_cloudkit.sync` operation
surface. `Engine.Run` performs one account-scoped run and `Engine.Watch` repeats
runs with a caller-supplied callback, bounded iteration count and cancellable wait.
The engine owns no goroutines. The caller owns the source, output directory and
filesystem dependency.

Use `DefaultOptions` to obtain the Source defaults, then select albums, rendition
sizes, date folders and optional filters. `NewSDKSource` binds a copied native
session to the public SDK; `Snapshot` returns updated credentials and ordered
response receipts. The SDK itself stays stateless. A source session rejects a
different account identity. Custom `Source` implementations can inject offline
photo enumeration, transfers and deletion. Every request supplies `AuthContext`.

```go
source, err := photosync.NewSDKSource(ctx, client, authenticated, libraries)
if err != nil { return err }
engine, err := photosync.New(source, photosync.Configuration{
    Files: photosync.OSFileSystem{},
})
if err != nil { return err }
options := photosync.DefaultOptions("./photos")
options.DryRun = true
preview, err := engine.Run(ctx, photosync.Request{
    Auth: authenticated.Auth, Options: options,
})
```

Run returns file-level downloaded, skipped, listed and deleted actions. It selects
the first available requested/original/medium/thumb resource. Live Photos also
select a video rendition unless videos are skipped. RAW alignment chooses the
requested RAW versus JPEG representation without mutating the input asset map.
Recent filtering uses added time with capture time fallback. `UntilFound` stops
after consecutive already-current resources. It cannot be combined with stale
cleanup or remote retention.
An `UntilFound` stop preserves the prior manifest cursor while retaining any
successfully downloaded resources. A later unrestricted run must enumerate the
remaining assets before its cursor can enable a restart shortcut. This deliberately
corrects the pinned Source's unconditional cursor advance after a limited scan;
the stop threshold and visited-resource behavior remain the same.

Persistent manifests track asset/resource identity, relative path, size, checksum,
download time and the last successful cursor. A restarted engine may skip an
unchanged cursor only after checking every tracked file and its expected size.
Preview runs never create directories, update manifests or delete local or remote
content. Missing transfer data prevents advancing the cursor and prevents remote
deletion. Remote retention requires every selected resource to be locally ready.
Empty, successfully downloaded files remain valid content.

Known metadata can produce an XMP sidecar and fill missing JPEG EXIF timestamps.
Foreign or malformed sidecars are preserved. Existing EXIF capture timestamps are
preserved, including big-endian TIFF data. Generated local projections and
embedded property-list/adjustment metadata are owned by
`api/photo-sync-models.openapi.yaml` and `api/photo-materialize.openapi.yaml`.

Resource names and date folders are sanitized. Every file access resolves existing
symlink ancestors and verifies that the target remains beneath the resolved output
root. Writes use a same-directory temporary file, flush it, close it and replace
the destination. A failed write cannot record the resource as downloaded. The
engine rejects overlapping runs; callers must give different engines distinct
output targets.

The Go manifest uses a generated JSON document and SHA-256 target identity rather
than Python's SQLite file and truncated SHA-1 filename. The target identity also
includes account ID, preventing two accounts from sharing a manifest. The state
format is intentionally Go-owned; resource and cursor behavior follows Source.
The Go implementation returns typed local I/O failures rather than silently
ignoring filesystem failures during materialization. `errors.Is` and `errors.As`
recover the original cause without logging credentials.

Verification uses the pinned Source commit
`e2e44ab875d47dab4475096021da60030f26c35e`. The synthetic filesystem oracle executes
`run_photo_sync` using deterministic photo operations, a fixed clock, real
temporary files and SQLite state. Paired fixtures compare action/results, exact
file bytes and download/deletion call transcripts. They are implementation-derived
synthetic evidence, not captures. Local materialization fixtures separately compare
exact Source EXIF bytes and XMP output. Cancellation, restart, size corruption,
preview safety, missing data and retention have independent negative controls.

The local operation denominator is the active pinned module, rather than network
routes. Its two orchestration entry points, `run_photo_sync` and
`watch_photo_sync`, map to `Engine.Run` and `Engine.Watch`. The option normalization,
target identity and state-root/path helpers map to generated `Options`,
`normalizedAlbums`, `targetIdentity` and `manifestPath`. Result/item projections
map to generated `Result` and `Item`.

| Active Source helper group | Go implementation and evidence |
| --- | --- |
| Library selection, cursor, asset feeds and deduplication | `SDKSource.selection`, `Cursor`, streaming `Visit`, `runState.visit`; selected read receipt test and account mismatch control |
| Resource selection and fallback, RAW alignment and live video | `selectResources`, materialization RAW helpers; filesystem RAW/live/filter scenarios |
| Date rendering, unique paths, filename and folder sanitization, root confinement | Date formatter and `paths.go`; paired date-format paths/errors, collision and traversal controls |
| Current-file detection and atomic writes | `isCurrent`, `OSFileSystem.WriteAtomic`; restart, size corruption, failed/missing transfer controls |
| Local XMP/EXIF, asset dates and remote-retention eligibility | `localMetadata`, `includeAsset`, `retention`; exact byte materialization oracle and retention scenario |
| Persistent state get/upsert/remove, file enumeration and cursor advancement | Generated JSON manifest helpers and successful-run reconciliation; restart, cleanup and unavailable-content controls |

The SQLite database file and target filename are a deliberate native-state
adaptation, not binary compatibility claims. Legacy shared-stream selection is
rejected by both the pinned sync entry point and the Go engine. Shared CloudKit
libraries use the discovered SDK selection and remain syncable. Paired fixtures
are kept under `tests/replay/fixtures/synthetic`; they never contain private
captures or credentials.

Date folders accept the deterministic C-locale `strftime` directives, ISO week and
year fields, microseconds, Windows numeric/composite flags, and Python's single
`datetime` `str.format` fields. Positional/default fields, brace escaping,
conversions, value attributes and nested numeric specifications have paired Source
path/error controls through `Engine.Run`. Valid Python formats that print bound
method memory addresses or depend on process-local CRT timezone representations
return `UnsupportedDateFormatError`; they are recorded separately as native
adaptations and excluded from parity claims. The paired tests inject lexical path
resolution so formatter output is assessed independently of Windows filename
restrictions.

The exact `sync.py` callable inventory is 20 module functions (two orchestration
entry points and 18 helpers) plus seven model methods: five option/state identity
helpers and two result serializers. The helper groups above cover the active
calls. `_sanitize_name` has no active caller in the pinned module; its filename
purpose is superseded by the explicitly documented native manifest naming.
This denominator is independent of the HTTP route inventory.

The date renderer intentionally fixes its dialect to CPython 3.12 on Windows with
`LC_TIME=C`, the runtime stated in the checked-in Source oracle. It produces the
same deterministic date paths on every Go host. The replay runner uses explicit
fixed-offset timestamps and an injected lexical filesystem; it does not recompute
Windows CRT expectations using Ubuntu's Python/libc. Linux/macOS-specific
`strftime` extensions and locale-dependent text are outside this dialect. A
successful replay on Ubuntu therefore proves portable Go execution of the pinned
Windows/C semantics, not parity with arbitrary native Python runtimes or locales.
