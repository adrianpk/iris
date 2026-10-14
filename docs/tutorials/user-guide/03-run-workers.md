# Run Codex workers

Goal: let beta read its own file, send the answer through Iris, and let alpha
acknowledge processing without a human relaying the answer.

Keep the `demo` and `conversation` variables from the previous chapter. Confirm
that `codex --version` works and the CLI has its normal authentication. Start
one explicitly bounded worker per project:

```sh
iris bridge start --project "$demo/alpha" --sandbox read-only --approvals deny --turn-seconds 90 --total-seconds 210 --max-turns 2 --max-tokens 150000 --json "$conversation" > "$demo/alpha-worker.log" 2>&1 &
alpha_worker=$!
iris bridge start --project "$demo/beta" --sandbox read-only --approvals deny --turn-seconds 90 --total-seconds 210 --max-turns 2 --max-tokens 150000 --json "$conversation" > "$demo/beta-worker.log" 2>&1 &
beta_worker=$!
```

Alpha initially waits without invoking a model. Beta receives the initial
request and runs Codex in beta's directory. A completed final result can send a
reply and acknowledge the request atomically. Alpha then processes that reply;
a completed answer can be acknowledged without another reply. Status/system
messages do not invoke a model. No repeated model polling is needed while idle.

Inspect completion from the same shell:

```sh
iris bridge status --project "$demo/alpha" --json "$conversation"
iris bridge status --project "$demo/beta" --json "$conversation"
iris show --project "$demo/alpha" --json "$conversation"
```

Expect the answer from beta's `answer.txt` and processing progress in both worker
statuses. A status with `state=waiting` after completion is normal. Stop both
workers after inspecting the result, before the later history chapter:

```sh
iris bridge stop --project "$demo/alpha" --json "$conversation"
iris bridge stop --project "$demo/beta" --json "$conversation"
wait "$alpha_worker"
wait "$beta_worker"
```

A failed or uncertain invocation leaves its input unacknowledged. Inspect
`recovery_hint` and follow the [bridge recovery contract](../../reference/bridge.md)
before taking another action. Do not treat a reply's absence as permission to
repeat an external tool. The two local logs contain worker output and should
remain private.

Each project/conversation gets its own Codex thread. This mode starts separate
owned workers; it does not wake or attach to your existing interactive Codex
session. Only the binding's own thread can resume. Peer text never grants tool
permissions. The example denies all approval requests and uses read-only
sandboxes. Interactive approvals require a foreground terminal and a deliberate
once-only decision; see the [bridge reference](../../reference/bridge.md).

Next: [Inspect and retain history](04-inspect-history.md).
