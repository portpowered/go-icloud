# Find My description and monitor controls

This contributor note records semantic controls beyond ordinary HTTP route checks.
Customer lifecycle instructions live in the [Find My guide](guides/findmy.mdx).

Description scenarios call the pinned source's name/model/type, status, location
and capability getters and compare the complete public projection. Missing default
status fields remain explicit null. Additional requested metadata preserves nested
JSON and large integers. A location is available only with LOC and a non-null
location value; an empty location object remains available. Local Go descriptions
perform no hidden request and return copied data after close.

Timed streams run the actual Source monitor and actual Go owned loop. Injected wait
events bind requested durations, eligible completion times and one terminal stop.
They cover refusal followed by recovery, repeated refusal with cookie rotation,
partial replies and continuation reset. Family LOADING metadata during monitoring
does not trigger another readiness poll. Monitor requests omit new-location controls.

Negative controls reject changed waits, ineligible or non-finite timestamps,
premature/omitted terminal stop, unconsumed events, changed state/failure meaning
and altered final cookie scope. MonitorDone must close after teardown; a terminal
read timeout alone is not completion proof (LIB-05).

Command cases include binary and malformed-JSON acknowledgements, successful
non-object JSON and provider error objects under accepted JSON media types.
Generated parsers preserve exact raw body bytes. Successful acknowledgements
remain distinct from completed device action; the SDK preserves typed failures
and sends each requested control once.

All portable examples are implementation-derived synthetic evidence. Current
coverage and final acceptance are maintained in [verification](verification.md)
and the [completion matrix](completion-matrix.md).
