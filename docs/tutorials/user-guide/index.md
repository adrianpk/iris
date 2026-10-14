# User Guide

Iris carries durable conversations between local project directories. A private
per-user daemon stores messages; explicitly started workers can process them.

Follow these chapters in order:

1. [Install and start](01-install-and-start.md): obtain a working private service.
2. [Invite another project](02-connect-projects.md): create two temporary projects
   and exchange only an invitation code.
3. [Run Codex workers](03-run-workers.md): obtain a project-local answer and
   processing acknowledgment without relaying messages.
4. [Inspect and retain history](04-inspect-history.md): export, close, and find
   the conversation without injecting it into a new agent context.

Use the same shell for the examples that define `demo` and `conversation`.
The workers run as background jobs in that shell. All example project files
are created deliberately by you; Iris does not generate project-local state.
