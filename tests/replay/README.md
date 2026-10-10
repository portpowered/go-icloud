# Paired replay

`HTTPTransport` consumes ordered portable exchanges without network fallback.
It snapshots inputs and binds method, authority, escaped path, repeated query
values, headers and the declared entity rule before returning paired status,
repeated headers and bytes. Rejected traffic remains a failure even if a caller
catches the error. Request bodies close on every exit; response bodies must be
read and closed before `AssertConsumed` succeeds (LIB-05).

Content-Length binds Go's framing field to actual entity bytes. JSON rules retain
number spelling and reject duplicate keys, trailing values, invalid UTF-8 and
unknown matcher fields. Pattern or credential redactions apply only at explicitly
named paths; surrounding JSON and array contents remain exact. The
`base64-zlib-exact` rule permits distinct DEFLATE encodings only after proving
the complete decompressed protobuf bytes and validating entity size. Multipart
rules bind ordered parts, filenames, headers and binary contents; only the
declared boundary variation is normalized.

Fixture loading and matcher interoperability are distinct from SDK behavior.
Public SDK drivers execute the actual operation against these same pairs and
compare return values or typed errors, cookies, rotations and response evidence.
Socket timelines and binary document cases remain separate artifact families.
Run `go test -race ./tests/replay/...` for current acceptance; use the endpoint
audit for current occurrence counts instead of copying changing totals here.

These types describe verification artifacts. Production models come from
canonical schemas. Synthetic artifacts are derived from reference execution;
captured fixtures represent recorded account behavior; historical artifacts
provide context without proving current acceptance. Their provenance is documented
in the respective fixture READMEs. See [verification](../../docs/verification.md),
[reference capture](../../docs/reference-capture.md) and
[completion matrix](../../docs/completion-matrix.md) for current scope.

Modern Photos uploads use the pinned Python implementation's reservation,
signed byte transfer, registration, ingest status and CloudKit hydration flows.
The selected upload scenarios are synthetic and implementation-derived;
they do not establish independently captured Apple behavior. The old
`uploadimagews` endpoint is a historical reference and is not used by these SDK
operations.

The composable uploader and library-service uploader preserve their different
query contexts. Upload receipts retain unknown JSON values without converting
large integers to floating-point values. Explicit byte framing, duplicate
registration, indexing deadlines, read backoff, cancellation and response
cookie scope are part of the paired replay boundary. File readers remain
caller-owned, and uncertain upload writes are never automatically repeated.
