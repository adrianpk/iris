# Expiration and retention

Iris separates logical expiration, retained history, and explicit deletion.
The daemon applies lifecycle decisions without starting a model. Closed or
expired conversations leave active inboxes and reject new ordinary messages.
They retain their metadata, messages, participants and processing records.

## Policy and time

The optional XDG `iris/config.json` accepts a partial lifecycle object:

```json
{
  "schema_version": 1,
  "lifecycle": {
    "invitation_seconds": 7200,
    "inactivity_seconds": 86400,
    "retention_seconds": 2592000
  }
}
```

Each duration must be an integer from 1 through 315360000 seconds. Omitted
fields retain defaults. Nulls, unknown fields and invalid ranges fail startup.
Restart the daemon to load changed configuration.

A new invitation stores its deadline at creation. Changing invitation policy
does not extend an issued code. Active inactivity uses the current daemon
policy and the last significant activity. New calls, successful first joins,
and new participant message/status inserts are significant. Reads, waits,
acknowledgments, bridge renewals, duplicate retries and maintenance do not renew
activity. The injected service clock owns UTC millisecond timestamps.

The daemon reconciles overdue state before listening. Maintenance processes
at most 128 expirations per transaction and schedules the nearest deadline,
with a maximum 30-second recheck for clock changes. Cancellation stops the
owned maintenance loop. A maintenance failure stops service rather than
continuing without expiration.

A terminal transition sets its timestamp once and commits one system event
and bridge shutdown requests together. Receivers wake after commit. Idle
bridges stop through that notification; an active turn observes the stop at
its five-second ownership renewal. Repeated closure cannot reset retention.
A clock moving backward cannot reactivate a terminal conversation.

## Preview and confirmation

History becomes eligible at terminal time plus current retention policy.
Expiration alone never deletes it. Bare `prune` and `--dry-run` preview:

```sh
iris prune --dry-run --json
iris prune --project alpha --state expired --before 2026-01-01T00:00:00Z
iris prune --all-projects --json
```

Default scope is the selected/inferred registered project. Outside a project,
use an explicit project or `--all-projects`. Optional `--conversation ID`,
`--state closed|expired` and `--before RFC3339` narrow the eligible selection;
they cannot select active or retained-ineligible data.

A preview returns the complete selection, terminal timestamps, revisions,
message/body-byte counts, cutoff, and a random 120-second confirmation token.
It changes only bounded confirmation metadata, without expiring or modifying
history, consumer progress or activity. The daemon allows 64 live previews.
Selections above 1,000 candidates fail and require a narrower filter; there is
no truncated deletion selection.

Inspect the preview, then explicitly confirm its token:

```sh
iris prune --confirm PREVIEW_TOKEN --json
```

Confirmation is noninteractive and can run outside a project. A token fixes
its original selection; it cannot be combined with new selection flags or
`--dry-run`. A changed/missing candidate, changed policy, expired/unknown token,
clock change that makes data ineligible, or live export lease returns
`preview_stale` and deletes nothing. Request another preview after resolving
the condition. Newly eligible data is not silently added. Successful deletion
and one-use token consumption share one transaction.

Deletion cascades to selected conversation payload, recipients, membership,
consumer cursors and bridge bindings. It preserves project IDs, pair ordinal
counters, invitation-code digests and call-request tombstones. A new conversation
always gets a distinct ID, a new ordinal and fresh history/context. Export and
public history inspection have separate command contracts; export never extends
retention or deletes source history.
