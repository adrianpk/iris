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
