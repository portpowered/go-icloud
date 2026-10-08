# Reminders text protocol

Reminder title and notes fields carry Apple's topotext CRDT documents. The
selected source decodes a current versioned envelope, a legacy version or a bare
nonempty string, after trying zlib and gzip decompression. The internal Go
`reminderstext` adapter now follows that order using generated protobuf models.
It accepts empty text inside an envelope and ignores unknown fields during text
extraction; generated models retain those fields when used independently.
Unreadable documents return a typed `DecodeError` with its underlying cause.
Truncated gzip preserves the unexpected-EOF cause and reference failure message;
corrupt DEFLATE preserves its compression cause and source exception family.
Native DEFLATE error text is intentionally normalized to Go's message rather than
Python zlib's implementation-specific wording. Bad gzip headers/checksums follow
the source's raw-document fallback. A gzip header is recognized before reading it,
so short uncompressed protobuf data is not misclassified as truncated gzip.
Concatenated members are decoded one at a time, accepting zero padding and
distinguishing a truncated next-member header from trailing non-header bytes.

`api/external/reminders-proto/reminders.proto` and `versioned_document.proto`
are byte-identical copies of timlaing/pyicloud's source files at
`e2e44ab875d47dab4475096021da60030f26c35e`. Their source attribution comments
remain intact. Generation maps their Go package names through the configuration,
without editing the pinned protocols (SCHEMA-01, SCHEMA-10, SCHEMA-16).
`make generate-proto` uses Buf v1.47.2 with a local protoc-gen-go v1.36.11 plugin.
The runtime is also pinned to v1.36.11. The generation contract compares both
source files with the pinned checkout and regenerates both Go files in a temporary
directory, requiring exact equality. It does not require a global protoc install
or send protocol files to a remote generation service.

The portable `tests/replay/fixtures/synthetic/binary/reminders-text.json` corpus
contains 52 unique input documents and their reference-decoded byte results or
errors. Current, legacy and bare forms include empty, ASCII, Unicode and astral
text with raw, zlib and gzip storage. It also binds unknown fields, zlib trailing
bytes, concatenated gzip members, truncated/corrupt/checksum failures, unreadable
documents and the distinct title
document from the existing synthetic HTTP response inventory. These are offline
implementation-derived examples, not captured Apple account data (LIB-04,
LIB-12). Each input/result is portable base64 data; the Python reference test
rechecks every stored result and error, and Go replay checks the same bytes and
typed failure families and retained causes. Source replay checks full reference
error text; Go checks the matching error text for document/truncation failures
and the native compression cause for corrupt DEFLATE.

Source function coverage runs this corpus under its own `synthetic:document`
context and reports 52 documents separately from HTTP exchanges and socket
events. Document decoding is not counted as a new endpoint or paired network call.
Unit and combined coverage select all internal library packages, so the decoder's
statements remain in the unit denominator even when no unit test imports it.
The document corpus is replay evidence; unit coverage for this adapter is zero.

The reference's proto2 decoder can return bytes for invalid UTF-8 text. The corpus
records that source type as well as the exact result bytes. The internal Go
adapter preserves those bytes in a Go string. It does not claim Python runtime
type parity. The future Reminders domain mapper must implement the source's
replacement-character policy when converting such bytes to public text.

This milestone ports decoding infrastructure. It does not implement CRDT writes,
UTF-16 substring lengths, resolution tokens, public Reminders methods or complete
service result/state parity. The 52 cases are document decoding evidence rather
than additional HTTP endpoints or full SDK replay acceptance. Native auth,
Photos, Reminders orchestration, publication and final full independent audits
remain open (LIB-05, LIB-07, LIB-18).
