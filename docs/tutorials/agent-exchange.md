# A conversation in practice

Two project agents write a verse together, alternating lines that begin with
the previous line's last word. Each agent receives one initial user prompt.

Use two registered projects with [agent integration](../how-to/integrate-an-agent.md)
configured. This illustrative transcript uses fictional projects `alpha` and
`beta`, and an invented verse. Start in alpha, then pass the invitation code
from its intermediate update to beta's initial prompt. Replace
`INVITATION_CODE` with the actual code.

```text
YOU · PROJECT ALPHA
Write a four-line verse with beta. Take lines 1 and 3.
Each new line must start with the previous line's last word.
Show the invitation, then keep the exchange going.
Close the conversation after receiving line 4.

ALPHA · PROGRESS
Invitation ready. Waiting for beta.

YOU · PROJECT BETA
Join INVITATION_CODE from alpha and write lines 2 and 4.
Start each line with the previous line's last word.
Keep waiting between turns.

                         THE EXCHANGE

alpha → beta   [1/4] The lantern guards the harbor through the night.
beta  → alpha  [2/4] Night carries distant songs across the sea.
alpha → beta   [3/4] Sea winds guide the travelers toward home.
beta  → alpha  [4/4] Home waits beneath the lantern's steady light.

ALPHA · COMPLETE
Verse complete. Conversation closed.
```

Alpha publishes the invitation without ending its turn. Once beta accepts,
the agents continue the exchange within their active turns. The result is
four linked lines and a closed conversation, without another prompt to alpha.
Waiting follows the [integration policy's bounds](../how-to/integrate-an-agent.md#establish-the-first-exchange).
