# Acceptance coverage

The ten groups below define the v1 acceptance boundary. Unit fixtures use real
SQLite/Unix sockets and controlled clocks where needed. Process tests build the
actual executable. User-systemd and Codex checks require their real external
boundaries; they fail rather than skip missing prerequisites.

| Group | Decisive invariants | Tests | Gates |
| --- | --- | --- | --- |
| 1. Identity | Canonical/nested/symlink discovery, alias collision and deliberate move preserve identity. | `TestProjectRegistration`, `TestProjectChangesRejectCollisions`, `TestProcessHandshake` | `make verify`, `make test-integration` |
| 2. Handshake | Correct/wrong/repeated/concurrent/expired join returns only the original initial request. | `TestJoinConcurrent`, `TestJoinInvitationExpiry`, `TestCallConcurrentRetry`, `TestProcessHandshake` | `make verify`, `make test-integration` |
| 3. Numbering | Atomic unordered-pair allocation, rollback, tombstones and non-reuse after deletion. | `TestCallConcurrentOrdinals`, `TestCallTombstones`, `TestProcessLifecycleRecovery` | `make verify`, `make test-integration` |
| 4. Messages | Ordered retries, rollback, independent consumers, processing acknowledgment and restart. | `TestSendConcurrent`, `TestSendRollback`, `TestAckOrdering`, `TestProcessDelivery` | `make verify`, `make test-integration` |
| 5. Isolation | Scoped visibility, fresh post-prune content and fresh owned Codex threads/consumers. | `TestHistoryScope`, `TestLifecycleFreshConversation`, `TestBridgeLiveExchange` | `make verify`, `make test-bridge` |
| 6. Lifecycle | Exact policy/time boundaries, retained export, stale confirmation and confirmed deletion. | `TestLifecyclePolicyBoundaries`, `TestPruneStaleConfirmations`, `TestProcessLifecycleRecovery` | `make verify`, `make test-integration` |
| 7. Output | JSON through jq, human output, Unicode/multiple pages and complete frozen transcripts. | `TestDumpCLIComplete`, `TestProcessHistoryExport`, `TestProcessUserGuide` | `make verify`, `make test-integration` |
| 8. Operation | Private inherited socket, actual activation/restart/disconnect/shutdown and consistent restore. | `TestActivationDescriptors`, `TestProcessBackupRestore`, `TestSystemdOperation` | `make verify`, `make test-integration`, `make test-systemd` |
| 9. Bridge | Real project-local proof, reply plus durable acknowledgment, lost-result recovery, resumption and idle counts. | `TestBridgeLiveExchange` | `make test-bridge` |
| 10. Security | Invalid/oversized input, scope/ownership, peer injection, denied approval and private modes. | `TestSecurityPeerInput`, `TestSecurityBridgeScope`, `TestSecurityRecoveryFrames`, `TestBridgeDeclinedRecovery`, `TestSystemdOperation` | `make verify`, `make test-systemd` |

`scripts/check-acceptance.py` checks that every named case exists and belongs
to the stated gate. It checks the coverage map, not runtime success. The owning
[report](../../ops/default/report/slices/iris-v1/slice-10-acceptance-documentation.md)
and exact-commit validation ledger record execution results. The complete gate
runs against one immutable integrated candidate; focused checks do not replace it.

## Reproduce the live exchange

From a source checkout with the verified Codex CLI and existing authentication:

```sh
make test-bridge
```

The fixture creates two private temporary projects with a random proof present
only in the receiving project's file. Two real workers exchange the proof and
acknowledge processing without a relay. It then recovers a deliberately dropped
completed result without another invocation, restarts the same bindings to
process pending work, and creates a second conversation with fresh owned threads
and consumers while the first history remains retained. Distinct random proofs,
scoped IDs, durable acknowledgments and idle turn/message counts are assertions.
The one-second CPU/RSS sample is a bounded observation, not a performance promise.

No ordinary user data, pre-existing provider thread or installed service unit is
used as fixture state. Only owned processes/units are stopped after checks.
Provider authentication, installed schema and available capacity are prerequisites.
No credentials or transcript contents belong in a validation report.
