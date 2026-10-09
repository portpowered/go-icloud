# Reminder writes

`CreateReminder`, `UpdateReminder` and `DeleteReminder` accept caller-owned
credentials through `pkg/icloud`. They send once and return response evidence;
an uncertain acknowledgement is not retried automatically. Returned nullable
values and identifier slices do not share mutable storage with the input.

Creation accepts the list identifier, title, optional notes, completion, priority,
flag, due date, all-day flag, time zone and parent identifier. It creates the
record and then looks up the complete reminder. A lookup failure retains the
earlier modification response in `ClientError.PriorResponses()`. Cookie updates
apply within the operation and remain available in returned response metadata.

Update accepts a complete `Reminder` snapshot. It writes the title and notes,
completion state/date, priority, flag, all-day state, due date, time zone and
parent reference. Missing due date, time zone and parent values clear those
fields. The result contains an independent updated snapshot and the last
nonempty matching acknowledgement revision. Delete performs a revision-aware
soft update and returns the acknowledged deletion, modification instant and
revision. Raw reminder identifiers receive the reference's `Reminder/` prefix.

`WithRandomSource` injects the UUID entropy source and `WithClock` supplies the
clock. Both injected dependencies must support concurrent use. An already
canceled context returns before document encoding, entropy consumption or
transport. Entropy failures return a typed configuration error before any write.
Millisecond conversion matches the reference's truncation of floating-point
Unix seconds, including fractional and pre-epoch instants.

The provider request variants are generated from
`api/external/cloudkit-models.openapi.yaml` and bound to the modify operation in
`api/external/reminders.openapi.yaml`. Resolution-token strings explicitly name
their embedded JSON component. Document strings bind base64, zlib and the
generated `versioned_document.Document` containing `topotext.String`. Their
offsets count UTF-16 units; the encoder is checked against complete decoded
reference protobuf bytes, including empty and astral-character documents.

Five creation, four update and two deletion synthetic scenarios bind full
requests, results or failures, revisions and response evidence through the public
SDK. The creation cases include failed hydration; rejection cases preserve exact
provider bodies. These are implementation-derived synthetic fixtures, not live
captures (LIB-05). The explicit compressed-document matcher compares all decoded
bytes and rejects corruption, truncation, trailing data and changed surrounding
fields. Actual request framing remains validated.

`CreateReminderHashtag`, `UpdateReminderHashtag` and `DeleteReminderHashtag`
return independent hashtag snapshots and ordered response evidence. Create and
delete also return the updated parent reminder. They submit the parent ID-list
update and child operation atomically. Delete rejects a child linked to another
reminder before consuming entropy, removes every matching raw or prefixed ID,
and preserves the order and duplicates of retained IDs. An empty or unrelated
acknowledgement does not overwrite a supplied revision. No snapshot is returned
when either record is rejected.

Hashtag text uses base64 UTF-8 bytes rather than a CRDT document. Creation returns
an explicit null creation instant, matching the reference's returned model even
though the wire record includes a creation timestamp. Six additional synthetic
scenarios verify the complete requests, results, rejection evidence and immutable
inputs. Narrow generated hashtag variants replace their pending endpoint
fallback; negative controls reject malformed fields, missing required fields,
non-atomic writes and orphaned parent updates (SCHEMA-11).

`AddReminderLocationTrigger` creates the parent link, alarm and trigger atomically.
Attachment methods create URL attachments, update URL or image metadata, and
atomically unlink and soft-delete attachments. Recurrence methods create, update
and atomically unlink and soft-delete rules. Fifteen original paired scenarios
cover these operations; an additional Source-derived scenario verifies deletion
of an empty attachment ID. Each method preserves caller snapshots, ordered raw
IDs and duplicates, and validates all acknowledgements before applying matching
nonempty revisions. Advancing-clock controls verify Source clock order.

All selected Reminder write requests now use narrow generated endpoint variants;
no generic pending-child fallback remains (SCHEMA-11). These milestones do not
establish full Photos, authentication, CLI or template conformance.
