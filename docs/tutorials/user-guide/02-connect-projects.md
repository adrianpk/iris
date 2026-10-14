# Invite another project

Goal: deliver an initial request to a second project using only an invitation
code. Run these commands in one shell with Iris available on `PATH`:

```sh
umask 077
demo="$(mktemp -d)"
mkdir "$demo/alpha" "$demo/beta"
printf 'format=JSON; proof=local-demo\n' > "$demo/beta/answer.txt"
(cd "$demo/alpha" && iris projects register --json)
invitation="$(cd "$demo/alpha" && iris call --to beta --topic 'Accepted format' --message 'Read answer.txt in your own project directory. Reply with its exact complete contents. A completed answer needs acknowledgment only.' --idempotency-key guide-question --json)"
code="$(printf '%s\n' "$invitation" | jq -er '.result.invitation_code')"
conversation="$(printf '%s\n' "$invitation" | jq -er '.result.conversation_id')"
(cd "$demo/beta" && iris join --json "$code")
```

The join result contains the original topic and initial message immediately.
The code identifies the invitation; the daemon validates that beta is its
recipient. Alpha's call reserved beta's identity before its path was known.
Joining binds beta to its canonical directory. Repeating the same join does not
create another activation or acknowledge work.

`conversation` is a stable ID. Its readable label includes the unordered project
pair and a monotonically increasing pair ordinal. Without an idempotency key,
every call creates a separate conversation. A repeated key with identical
content returns the original; changed content conflicts.

Commands discover the nearest registered ancestor of cwd. Register your intended
root before calling from an unregistered nested directory. Use `--project` with
a registered ID, canonical path or unambiguous alias for explicit selection.
See [project identities](../../reference/projects.md) for aliases and moves.

Invitation codes expire after two hours by default. If a code is expired, create
a deliberate new call; a join never reopens a closed or expired conversation.

Next: [Run Codex workers](03-run-workers.md).
