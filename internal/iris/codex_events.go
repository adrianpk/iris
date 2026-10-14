package iris

import (
	"context"
	"encoding/json"
)

type codexEvent struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Turn     struct {
		ID     string          `json:"id"`
		Status string          `json:"status"`
		Error  json.RawMessage `json:"error"`
	} `json:"turn"`
	Item struct {
		Type     string  `json:"type"`
		Text     string  `json:"text"`
		Phase    *string `json:"phase"`
		Status   string  `json:"status"`
		ExitCode *int    `json:"exitCode"`
	} `json:"item"`
	TokenUsage struct {
		Total struct {
			TotalTokens *int64 `json:"totalTokens"`
		} `json:"total"`
	} `json:"tokenUsage"`
}

func (progress *codexProgress) handle(ctx context.Context, frame codexFrame) error {
	if len(frame.ID) != 0 {
		progress.denied = true

		return progress.client.rejectRequests(ctx, frame)
	}

	var event codexEvent
	if err := json.Unmarshal(frame.Params, &event); err != nil {
		return problem("worker_uncertain", "invalid Codex event parameters")
	}

	if event.ThreadID != progress.threadID {
		return nil
	}

	if frame.Method == "turn/started" {
		return progress.started(ctx, frame.Params)
	}

	if event.TurnID != "" && event.TurnID != progress.turnID {
		return problem("worker_uncertain", "unexpected concurrent Codex turn")
	}

	return progress.acceptEvent(frame.Method, event)
}

func (progress *codexProgress) acceptEvent(method string, event codexEvent) error {
	switch method {
	case "item/completed":
		return progress.completedItem(event)
	case "thread/tokenUsage/updated":
		if event.TokenUsage.Total.TotalTokens != nil && *event.TokenUsage.Total.TotalTokens >= 0 {
			progress.usage = event.TokenUsage.Total.TotalTokens
		}
	case "turn/completed":
		if event.Turn.ID != progress.turnID {
			return problem("worker_uncertain", "completed turn does not match the invoked turn")
		}

		progress.completed = true

		if event.Turn.Status != "completed" {
			return codexTurnFailure(event)
		}
	}

	return nil
}

func (progress *codexProgress) completedItem(event codexEvent) error {
	if event.Item.Type == "agentMessage" && (event.Item.Phase == nil || *event.Item.Phase == "final_answer") {
		if len(event.Item.Text) > maxMessageBodyBytes+4096 {
			return problem("limit_exceeded", "Codex final result exceeds the response limit")
		}

		progress.final = event.Item.Text
	}

	if event.Item.Status == "failed" || event.Item.Status == "declined" {
		return problem("worker_failed", "Codex tool execution did not succeed")
	}

	return nil
}

func codexTurnFailure(event codexEvent) error {
	if event.Turn.Status == "interrupted" {
		return problem("worker_interrupted", "Codex turn was interrupted")
	}

	var failure struct {
		CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
	}

	if err := json.Unmarshal(event.Turn.Error, &failure); err == nil && string(failure.CodexErrorInfo) == `"usageLimitExceeded"` {
		return problem("provider_capacity", "Codex reported a usage limit")
	}

	return problem("worker_failed", "Codex turn failed")
}
