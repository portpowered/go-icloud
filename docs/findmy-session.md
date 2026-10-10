# Find My lifecycle verification

Contributor replay obligations for the public lifecycle are:

- Opening discovers devices and performs bounded family readiness polling.
- Refresh merges provider state, retains cached devices for missing/empty content,
  preserves order and applies cookie updates to the account-bound session.
- Monitor waits are injected, cancellation-aware and owned by the session.
- Concurrent close cancels active and queued requests without closing caller transports.
- Snapshot and descriptions return copied cache data, including after close.
- Capability and missing-token failures stop before a device command.
- Acknowledgements retain exact response evidence and do not establish physical completion.

The source reference, schema ownership and paired scenario population are described
in [Find My wire contracts](findmy-wire-contracts.md). Scheduler and description
projection controls are described in [behavior replay](findmy-behavior.md).
Customer examples belong in the [Find My MDX guide](guides/findmy.mdx).
Current acceptance and reviewer receipts belong in the
[completion matrix](completion-matrix.md) and [independent review](independent-review.md).
