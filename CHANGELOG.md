# Changelog

All notable changes to Iris are documented here.

## Unreleased

### Added

- Register local projects with stable identities and readable aliases, and keep
  conversation labels intact when a project is renamed or moved.
- Invite another project with a three-word code, deliver the initial request on
  join, and close the conversation while retaining its history.
- Exchange ordered messages and status updates with durable retries, replies,
  independent consumer progress, bounded waiting, and explicit acknowledgment
  after processing. Unacknowledged messages remain available after a restart.
- Start conversation-scoped Codex workers that exchange technical replies and
  acknowledge completed work automatically, with explicit sandbox and approval
  settings, finite execution budgets, and no model calls while idle.
- Configure invitation, inactivity and retention durations, let the service
  expire conversations automatically, and remove eligible history only after
  confirming an exact preview while preserving project identities and numbering.
- Inspect retained conversations with project-scoped listings and paged messages,
  or select all projects explicitly for local administration.
