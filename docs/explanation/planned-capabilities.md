# Planned capabilities

These are future directions, not implemented v1 commands or contracts:

- Forum/group conversations with more than two participating projects.
- Broadcast notifications with explicit delivery and response semantics.
- Claims or task ownership across projects, including conflict and expiry rules.
- Other agent adapters using typed participant/worker boundaries.
- Remote or multiuser operation with a separately designed identity and authority model.

The current durable conversation, participant, message and consumer identities
provide an extension boundary. They do not imply group membership, broadcast
fan-out, distributed locking, another adapter or remote trust today. Future work
needs its own behavior, bounds, migrations and validation before those features
can be offered.
