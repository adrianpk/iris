# Integrate an existing agent session

Use this guide to let an agent working in one project communicate with an
agent working in another. You configure the harness that runs each agent;
installing Iris alone does not add instructions, hooks or skills to it.

## Choose an integration

| Method | Choose it when | When Iris is checked |
| --- | --- | --- |
| CLI only | You want to control each exchange manually. | When you or the agent explicitly run a command. |
| AGENTS.md | Your harness loads project instructions and you want a check on every user turn. | The agent follows the startup directive. |
| Prompt hook | Your harness supports a hook that adds instructions before a user prompt. | The hook supplies the directive on each prompt; the agent runs the CLI. |
| Skill | You want a reusable communication workflow loaded for relevant tasks. | When the skill is invoked or selected by the harness. |
| AGENTS.md or hook, plus skill | You want regular checks and a discoverable workflow for starting consultations. | One startup check, with the skill available for the exchange. |
| Iris bridge | You need workers that wait for messages and invoke agents without new user prompts. | An explicitly started worker receives and processes messages. |

AGENTS.md and the prompt hook are alternatives for the startup directive. Both
are usable; installing both repeats instructions without adding a new trigger.
A skill alone does not guarantee a check on every turn. For regular checks in
an existing session, choose AGENTS.md for portability or the hook for an
explicit harness lifecycle event. Add the skill for its reusable workflow.

The AGENTS.md fragment and skill use ordinary CLI commands and can be used by
any compatible harness. The supplied hook configuration targets Codex. Other
harnesses need their own event and output format; copying Codex's JSON into
another harness is not a supported setup.

The three supplied recipes were tested with Codex CLI 0.161.0 and a real Iris
service: one prompt caused discovery, receive, a linked reply and acknowledgment
in each case. The hook test disabled skill discovery to exercise the hook
independently. Other harnesses require the verification steps below.

## Prepare each project

First [install and activate Iris](linux-operation.md). The service and projects
must belong to the same operating-system user on the same machine. Make `iris`
available on the harness's `PATH`; hooks also require Python 3 and the supplied
Codex hook command requires Git.

Register the intended roots before launching sessions in their subdirectories:

```sh
iris projects register --name alpha /path/to/alpha
iris projects register --name beta /path/to/beta
```

Project identity lives in the service registry. No `.iris.toml`, hook,
AGENTS.md or skill is required for registration itself. An optional
[`.iris.toml`](../reference/projects.md) only supplies a registration name.

For the following installation commands, use a local checkout of Iris and
replace these paths:

```sh
iris_source=/path/to/iris
project_root=/path/to/alpha
```

Repeat the chosen setup for the other project. Do not replace an existing
AGENTS.md, hooks file or skill directory without reviewing its contents.

## Use AGENTS.md

The [bundled turn policy](../../integrations/iris/references/turn-policy.md)
is the complete fragment. Append it once to the root AGENTS.md after reviewing
the policy, including its reply authority and waiting bounds:

```sh
printf '\n' >> "$project_root/AGENTS.md"
cat "$iris_source/integrations/iris/references/turn-policy.md" >> "$project_root/AGENTS.md"
```

Launch a new session from the project root, or use your harness's documented
instruction reload mechanism. Confirm that it loads AGENTS.md. For a harness
that uses another instruction filename, place the same fragment there.

The fragment instructs the agent to list current-project conversations, receive
messages, keep the sender's project ID, answer within the authorized exchange,
and acknowledge processed messages. The file does not run commands by itself.

## Import the skill

The self-contained [Iris skill](../../integrations/iris/SKILL.md) includes its
policy and hook helper. In Codex, install it for a project by copying the entire
directory into `.agents/skills`:

```sh
mkdir -p "$project_root/.agents/skills"
cp -R "$iris_source/integrations/iris" "$project_root/.agents/skills/iris"
```

For all of your Codex projects, use the user skill location instead:

```sh
mkdir -p "$HOME/.agents/skills"
cp -R "$iris_source/integrations/iris" "$HOME/.agents/skills/iris"
```

Choose one scope to avoid duplicate discovery. Codex detects skill changes;
restart it if the skill does not appear. Invoke it explicitly with, for example:

```text
$iris Ask beta's agent about its architecture so we can adapt it to this project.
```

Its description also permits implicit selection for relevant communication
requests. Selection is a harness decision; it is not an inbox subscription.
For another Agent Skills-compatible harness, import the entire `iris` directory
using that harness's skill location or importer. The workflow requires only the
Iris CLI; `scripts/prompt-hook.py` is used only by the Codex hook recipe.

## Configure the Codex prompt hook

Import the skill directory into the project as above, then review
[the hook helper](../../integrations/iris/scripts/prompt-hook.py) and
[the hook definition](../../integrations/codex/hooks.json). The helper emits
the same static turn policy as AGENTS.md. It does not fetch messages, call a
model, send replies or acknowledge delivery. The agent performs those CLI
operations under the session's existing permissions.

If the project has no hooks file:

```sh
mkdir -p "$project_root/.codex"
cp "$iris_source/integrations/codex/hooks.json" "$project_root/.codex/hooks.json"
```

If `.codex/hooks.json` already exists, merge the supplied
`hooks.UserPromptSubmit` group into that file, preserving its other events and
handlers. Codex also merges user and project hooks, so install the Iris handler
at only one level. The resulting Iris entry is:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "python3 \"$(git rev-parse --show-toplevel)/.agents/skills/iris/scripts/prompt-hook.py\"",
            "timeout": 5,
            "statusMessage": "Loading Iris communication instructions"
          }
        ]
      }
    ]
  }
}
```

This command works from subdirectories of a Git repository. For a non-Git
project, replace it with `python3 "/absolute/path/to/iris/scripts/prompt-hook.py"`
pointing at the copied bundle. To use one user-wide hook instead, put the
definition in `~/.codex/hooks.json`, import the skill at user scope, and set
the command to `python3 "$HOME/.agents/skills/iris/scripts/prompt-hook.py"`.
That policy will run in all sessions using the user hook, including directories
that have not been registered with Iris.

Launch Codex in the project. Trust the project configuration through Codex's
normal project trust flow. Open `/hooks`, review the Iris `UserPromptSubmit`
definition and trust it. New or changed definitions are skipped until reviewed.
Submit a new prompt and confirm the hook runs. A `Stop` or `PostToolUse` hook is
not needed for this recipe.

Check current harness behavior against the official
[Codex hooks documentation](https://learn.chatgpt.com/docs/hooks) and
[skill discovery documentation](https://learn.chatgpt.com/docs/build-skills)
when upgrading Codex.

## Establish the first exchange

In alpha's session, ask the agent to create a consultation with beta and state
the outcome you need. With the skill installed, an example is:

```text
$iris Open a conversation with beta. Investigate its architecture through concrete questions, then propose how to apply those boundaries to alpha's domain.
```

The underlying command is:

```sh
iris call --to beta --topic "Architecture consultation" --message "Explain your module boundaries and their rationale so we can adapt them to alpha's domain." --json
```

Give the returned invitation code to beta's session and authorize acceptance:

```text
$iris Join INVITATION_CODE. Answer alpha's architecture questions using this project's code and continue the exchange until its questions are resolved.
```

The command in beta is `iris join --json INVITATION_CODE`. Keep the returned
conversation ID. Subsequent messages need no new invitation; each agent replies
in that conversation and sees the sender's project identity.

If alpha's turn finished while you passed the invitation to beta, submit one
prompt in alpha to continue the consultation. Joining in beta does not wake
alpha's existing session. Both sessions need active, instructed turns to carry
on the exchange without further prompts.

During a requested exchange, a receive timeout can be followed by another
receive in the same active harness turn. The bundled policy permits five
consecutive empty 60-second waits and at most 20 outgoing replies per turn.
It stops earlier when the requested outcome is complete and reports remaining
work when a bound is reached.

Once a session finishes its turn, neither AGENTS.md nor this prompt hook nor a
skill wakes it for a later message. For continued processing without new user
prompts, start [Iris bridge workers](../tutorials/user-guide/03-run-workers.md)
on both sides. Those workers own separate Codex threads; they do not attach to
your existing interactive sessions.

## Verify the setup

From each project root, check `iris conversations --active --json`. An operating
error is not evidence of an empty inbox. Then use a joined conversation for a
delivery test:

1. In beta, run `iris send --message "Reply with integration-ok." CONVERSATION_ID`.
2. Submit one prompt in alpha's harness: `Check Iris and handle the test request.`
   For the skill-only recipe, use `$iris Check Iris and handle the test request.`
3. Confirm alpha actually invokes `conversations`, `receive`, `send` and `ack`
   with consumer `iris-agent`. For the hook recipe, also confirm its successful
   `UserPromptSubmit` event; the model following a direct prompt alone does not
   prove that the hook ran.
4. In beta, use `iris show --json CONVERSATION_ID` to confirm alpha's reply links
   the test message with `reply_to`. History inspection does not consume work.
5. Repeat alpha's inbox check. The handled test message must not be delivered
   again to `iris-agent`.

Do not run an Iris bridge and this session integration as competing processors
for the same project's conversation. Keep one responder responsible for it;
independent consumers can receive the same work and produce duplicate replies.

To remove the integration, remove only its AGENTS.md section, hook handler and
copied skill directory. Removing instructions does not delete project identity,
conversations or messages.
