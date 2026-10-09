# Go paired replay

`HTTPTransport` consumes ordered portable `Exchange` values without a network
fallback. It snapshots its input, matches the method, authority, escaped path,
ordered repeated query values, headers and the declared entity rule before returning
the paired status, repeated response headers and bytes. A rejected request stays
rejected even if a caller catches its error. Request bodies close on every exit;
response bodies must be read and closed before `AssertConsumed` succeeds.

Go represents Content-Length separately from its header map. When the reference
fixture includes that header, replay binds it to the request's framing field and
actual entity size. Opaque URLs, URL credentials/fragments, host overrides,
unknown lengths, forced queries, trailers, chunked framing and the Close flag
reject because they are not represented by these exact-body exchanges (LIB-05).

JSON-pattern rules match strings at explicit object/array paths using a full
regular expression. Equivalent paths reject, and every pattern checks the
original request before any sample replacement. All other fields and array
contents remain exact. Redacted JSON accepts nonempty string passwords and
six ASCII digit codes only at the named credential fields; other fields remain
exact. Actual Content-Length still binds bytes before the recorded redacted
length is omitted. JSON number spelling is retained without float64 conversion;
duplicate keys, trailing values, invalid UTF-8 and unknown rule fields reject.
These duplicate/ambiguous-rule checks deliberately strengthen reference replay.

The explicit `base64-zlib-exact` rule binds a named string path to the complete
decompressed bytes of its recorded value. It rejects invalid base64, checksum
errors, truncated streams, trailing bytes and changed protobuf content. Go and
Python may produce different DEFLATE encodings for the same bytes. This rule
adjusts the recorded Content-Length only after validating the actual entity size;
all surrounding JSON fields and request headers remain bound (LIB-05).

Multipart rules bind a full Content-Type pattern, its boundary and ordered parts.
Part header values, filenames and unencoded binary bytes remain exact; framing
requires CRLF delimiters with no preamble, epilogue or premature boundary.
Nested and transfer-encoded parts are explicitly unsupported; the fifteen selected
upload artifacts use unencoded parts. Only the declared Content-Type variation
is normalized; actual lengths and other request headers remain bound.

All 456 portable HTTP scenarios (1030 exchanges) now instantiate in Go; all fifteen
multipart upload pairs match Go's writer with a different declared boundary.
These tests prove fixture decoding and matcher interoperability, not SDK
operation semantics. Auth/socket timelines, account/session projections, source-pin scenario loading
and the complete Go SDK scenario drivers remain to be ported. Unsupported entity
encodings fail construction; no approximate match or fallback is provided.
The generated account-client and public SDK drivers each execute 34 account
scenarios/40 exchanges, binding results or typed errors plus response evidence.
Family/photo flows use returned member identifiers. Storage binds usage, quota,
media and unknown fields. Opaque plans cover global/China gateways, arrays and
null. Concurrent two-account device replays verify isolation and repeated fresh
reads. Full authentication, other selected services and live SDK integration
remain open.

The types here describe verification artifacts, not provider production wire
models. Production schemas and generated models remain a separate requirement.
Run `go test -race ./tests/replay/...`; `make check` and CI also run these tests.

Find My transport replay executes 37 scenario streams/70 paired exchanges using
generated route builders, scenario command inputs and preceding received context.
Read/token projections and exact response metadata/bytes are checked. A strict
Source-derived reverse-order context case preserves provider member order while
clearing theftLoss. These checks follow fixture route sequencing and do not prove
SDK cache, polling, capability guards or monitor behavior; Find My public SDK
semantic replay remains 0/37. See [transport evidence](../../docs/findmy-transport.md).
