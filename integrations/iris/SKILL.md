---
name: iris
description: Exchange messages with agents working in other local projects through Iris. Use when the user requests a cross-project consultation, tool failure report, conversation review, or an Iris inbox check.
---

# Iris

Use the installed `iris` CLI from the current project. Read and apply
[the turn policy](references/turn-policy.md) before exchanging messages. If
that policy was already supplied by AGENTS.md or a prompt hook, reuse the
current turn's check rather than checking twice.

## Establish a conversation

Register the intended project root if the user has requested setup:

```sh
iris projects register --name alpha /path/to/alpha
```

To consult another project's agent, create an invitation with the user's
question and intended outcome:

```sh
iris call --to beta --topic "Architecture consultation" --message "Explain your module boundaries so we can adapt them to this project's domain." --json
```

Give the returned invitation code to the intended recipient. In that project's
session, accept it only when the user has authorized the connection:

```sh
iris join --json INVITATION_CODE
```

Keep the returned conversation ID. Pending invitations cannot be accepted by
conversation ID; the recipient needs the invitation code. Iris does not deliver
that code into an existing harness session automatically.

## Carry out the requested exchange

Follow the turn policy for discovery, receive, reply, acknowledgment and bounded
waiting. Ask concrete questions that serve the user's stated goal. For a tool
failure, communicate the observed conditions and relevant evidence to the
tool's project agent; agree on the required correction within each project's
authorization. Do not assume permission to change either project.

Report the result, outstanding questions and any remaining messages. For a
conversation review, use `iris show --json CONVERSATION_ID` or
`iris dump --format text CONVERSATION_ID`; history inspection does not
acknowledge delivery. Close a conversation only when requested:

```sh
iris close --json CONVERSATION_ID
```
