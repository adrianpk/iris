package iris

import (
	"context"
	"encoding/json"
)

type bridgeAnswer struct {
	Action string `json:"action"`
	Body   string `json:"body"`
}

type codexResult struct {
	Answer bridgeAnswer
	TurnID string
	Usage  *int64
}

type codexProgress struct {
	client    *codexClient
	threadID  string
	turnID    string
	final     string
	usage     *int64
	completed bool
	denied    bool
	observe   func(context.Context, bridgeCheckpoint) error
}

func answerSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"action": map[string]any{"type": "string", "enum": []string{"reply", "ack"}}, "body": map[string]any{"type": "string"}}, "required": []string{"action", "body"}, "additionalProperties": false}
}

func (client *codexClient) runTurn(ctx context.Context, binding bridgeBinding, input string, observe func(context.Context, bridgeCheckpoint) error) (result codexResult, runErr error) {
	progress := codexProgress{client: client, threadID: *binding.ThreadID, observe: observe}
	defer func() { runErr = progress.interrupted(ctx, runErr) }()

	params := map[string]any{"threadId": progress.threadID, "cwd": binding.ProjectPath, "approvalPolicy": codexApprovalPolicy(binding.Options), "approvalsReviewer": "user", "sandboxPolicy": codexSandbox(binding), "input": []any{map[string]any{"type": "text", "text": input}}, "outputSchema": answerSchema()}

	raw, err := client.call(ctx, "turn/start", params, progress.handle)
	if err != nil {
		return codexResult{}, err
	}

	if err := progress.started(ctx, raw); err != nil {
		return codexResult{}, err
	}

	for !progress.completed {
		frame, err := client.read(ctx)
		if err != nil {
			return codexResult{}, err
		}

		if err := progress.handle(ctx, frame); err != nil {
			return codexResult{}, err
		}
	}

	return progress.result()
}

func (progress *codexProgress) started(ctx context.Context, raw json.RawMessage) error {
	var value struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}

	if err := json.Unmarshal(raw, &value); err != nil || value.Turn.ID == "" {
		return problem("worker_uncertain", "Codex did not confirm the invoked turn")
	}

	if progress.turnID != "" && progress.turnID != value.Turn.ID {
		return problem("worker_uncertain", "Codex turn identity changed")
	}

	progress.turnID = value.Turn.ID

	return progress.observe(ctx, bridgeCheckpoint{Action: "turn", TurnID: value.Turn.ID})
}

func (progress *codexProgress) result() (codexResult, error) {
	if progress.denied {
		return codexResult{}, problem("approval_denied", "Codex authority request was declined")
	}

	var answer bridgeAnswer
	if err := decodeJSON([]byte(progress.final), &answer); err != nil {
		return codexResult{}, problem("worker_failed", "Codex did not produce a valid final result")
	}

	if answer.Action == "ack" && answer.Body == "" {
		return codexResult{Answer: answer, TurnID: progress.turnID, Usage: progress.usage}, nil
	}

	if answer.Action != "reply" || !validText(answer.Body, maxMessageBodyBytes) {
		return codexResult{}, problem("worker_failed", "Codex final response violates the message contract")
	}

	return codexResult{Answer: answer, TurnID: progress.turnID, Usage: progress.usage}, nil
}
