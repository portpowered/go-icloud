# Reference migration workflow

This is a contributor workflow for the pinned Python iCloud implementation.
The shared library requirements live in [library standards](library-standards.md);
the acceptance record lives in [completion matrix](completion-matrix.md).

Before changing behavior, identify the reference callable and its generated
route or channel. Build portable operation inputs, ordered request/response pairs
and observed return or error projections by executing that pinned implementation.
Record deterministic time, identity, cookies, random values and caller file state
as explicit scenario inputs. Keep live captures and implementation-derived
synthetic fixtures separate (LIB-04, LIB-05, LIB-12).

Implement the caller flow through the exported SDK and injected transport.
Require the same request sequence and result, including preceding response
metadata on later failures. Add controls which mutate meaningful request and
response fields and verify rejection, cancellation and cleanup. Update canonical
schemas before regenerating models; do not insert wire structs into business logic.

Use [reference capture](reference-capture.md) for instrumentation and artifact
formats, [client design](client-design.md) for public boundaries, and
[verification](verification.md) for the shipped gates. Refresh customer guides
when the public API changes. Keep unresolved behavior and intentional deviations
in the service contract and acceptance record with their evidence, rather than
copying the full standards into another task list.
