# Inspect and retain history

Goal: inspect and export the conversation, then close it while preserving
history. With the workers stopped, use the variables from earlier chapters:

```sh
iris conversations --project "$demo/alpha" --json
iris search --project "$demo/alpha" --text 'Accepted format' --json
iris dump --project "$demo/alpha" --format text "$conversation" > "$demo/transcript.txt"
iris dump --project "$demo/alpha" --format json "$conversation" > "$demo/transcript.json"
jq -e '.schema_version == 1 and (.messages | length) >= 2' "$demo/transcript.json"
iris close --project "$demo/alpha" --json "$conversation"
iris show --project "$demo/alpha" --json "$conversation"
```

Expect ordered messages and a closed conversation in the last result. Inspection
and export do not acknowledge processing, renew inactivity or import history
into an agent. `show` is paged; `dump` emits a complete frozen transcript or
fails with no partial successful stdout. Use `--all-projects` explicitly for
administration outside a project.

Closed and expired conversations stay available during retention, which defaults
to 30 days. Expiration removes a conversation from active inboxes; it does not
delete its history. To inspect eligible deletion later:

```sh
iris prune --project "$demo/alpha" --dry-run --json
```

A preview deletes nothing. Inspect its exact candidates and use `iris prune
--confirm TOKEN --json` only for a deliberate irreversible deletion. A stale or
changed selection requires a new preview. See [lifecycle](../../reference/lifecycle.md).
Take a [consistent backup](../../how-to/backup-recovery.md) before maintenance;
a transcript is not a complete database backup.

A new call creates a new ID, pair ordinal and worker context. Old transcripts
enter another session only when you explicitly export and provide them.
When finished, remove only the temporary project directory printed by
`printf '%s\n' "$demo"`; retained Iris history remains subject to retention.
