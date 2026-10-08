# Go paired replay

`HTTPTransport` consumes ordered portable `Exchange` values without a network
fallback. It snapshots its input, matches the method, authority, escaped path,
ordered repeated query values, headers and exact base64 entity before returning
the paired status, repeated response headers and bytes. A rejected request stays
rejected even if a caller catches its error. Request bodies close on every exit;
response bodies must be read and closed before `AssertConsumed` succeeds.

Go represents Content-Length separately from its header map. When the reference
fixture includes that header, replay binds it to the request's framing field and
actual entity size. Opaque URLs, URL credentials/fragments, host overrides,
unknown lengths, forced queries, trailers, chunked framing and the Close flag
reject because they are not represented by these exact-body exchanges (LIB-05).

This is the first transport port. JSON-pattern/redacted and multipart matching,
auth/socket timelines, account/session projections, source-pin scenario loading
and the complete Go SDK scenario drivers remain to be ported. Unsupported entity
encodings fail construction; no approximate match or fallback is provided.
The account fixture test verifies HTTP-client/portable-exchange interoperability,
not an implemented account SDK operation or semantic compatibility.

The types here describe verification artifacts, not provider production wire
models. Production schemas and generated models remain a separate requirement.
Run `go test -race ./tests/replay/...`; `make check` and CI also run these tests.
