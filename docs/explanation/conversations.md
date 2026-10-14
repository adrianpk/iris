# Conversations and workers

A project identity is durable even when its alias or directory changes. A
conversation is one independent question/exchange between a project pair,
identified by a stable ID and a readable pair ordinal. An invitation binds a
recipient without making the code a remote authorization mechanism.

Delivery and execution are separate. SQLite stores ordered messages before a
sender receives success. Each consumer records delivery independently and
acknowledges only completed processing. A worker can therefore recover pending
work after a restart without making an inbox read imply success.

Codex workers are optional owners of that processing. They wait outside the
model and run only for ordinary addressed inputs. Separate project/conversation
threads prevent one retained exchange from becoming another conversation's
implicit context. Resuming an owned thread preserves its local execution
history; creating a new conversation starts fresh.

Expiration ends active delivery, retention keeps explicit human history, and
confirmed pruning removes eligible payload. Those are separate transitions.
Exports are deliberate snapshots for inspection or reuse; they do not reactivate
history or acknowledge work. See [lifecycle](../reference/lifecycle.md),
[storage](../reference/storage.md), and [trust](../reference/trust.md).
