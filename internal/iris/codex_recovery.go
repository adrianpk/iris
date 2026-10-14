package iris

import (
	"context"
	"encoding/json"
)

type codexStoredTurn struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Error     json.RawMessage   `json:"error"`
	Items     []codexStoredItem `json:"items"`
	ItemsView string            `json:"itemsView"`
}

type codexStoredItem struct {
	Type     string  `json:"type"`
	Text     string  `json:"text"`
	Phase    *string `json:"phase"`
	Status   string  `json:"status"`
	ExitCode *int    `json:"exitCode"`
	Success  *bool   `json:"success"`
}

func (client *codexClient) readResult(ctx context.Context, binding bridgeBinding) (codexResult, error) {
	if binding.ThreadID == nil || binding.TurnID == nil {
		return codexResult{}, problem("worker_uncertain", "recorded thread and turn are required")
	}

	raw, err := client.call(ctx, "thread/read", map[string]any{"threadId": *binding.ThreadID, "includeTurns": true}, client.rejectRequests)
	if err != nil {
		return codexResult{}, err
	}

	return recoverCodexResult(raw, binding)
}

func recoverCodexResult(raw json.RawMessage, binding bridgeBinding) (codexResult, error) {
	turn, err := recordedCodexTurn(raw, binding)
	if err != nil {
		return codexResult{}, err
	}

	if turn.Status != "completed" {
		return codexResult{}, storedTurnFailure(turn)
	}

	progress := codexProgress{threadID: *binding.ThreadID, turnID: turn.ID, completed: true}
	for _, item := range turn.Items {
		if err := validateStoredTool(item); err != nil {
			return codexResult{}, err
		}

		event := codexEvent{}
		event.Item.Type, event.Item.Text, event.Item.Phase = item.Type, item.Text, item.Phase

		event.Item.Status, event.Item.ExitCode = item.Status, item.ExitCode
		if err := progress.completedItem(event); err != nil {
			return codexResult{}, err
		}
	}

	return progress.result()
}

func storedTurnFailure(turn codexStoredTurn) error {
	if turn.Status == "inProgress" || turn.Status == "" {
		return problem("worker_uncertain", "recorded invocation has no confirmed completion")
	}

	event := codexEvent{}
	event.Turn.Status, event.Turn.Error = turn.Status, turn.Error

	return codexTurnFailure(event)
}

func validateStoredTool(item codexStoredItem) error {
	switch item.Type {
	case "commandExecution", "fileChange", "mcpToolCall", "dynamicToolCall", "collabAgentToolCall":
		if item.Status != "completed" || (item.Success != nil && !*item.Success) {
			return problem("worker_failed", "stored turn contains unfinished or failed tool work")
		}
	}

	return nil
}

func recordedCodexTurn(raw json.RawMessage, binding bridgeBinding) (codexStoredTurn, error) {
	var value struct {
		Thread struct {
			ID    string            `json:"id"`
			CWD   string            `json:"cwd"`
			Turns []codexStoredTurn `json:"turns"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return codexStoredTurn{}, problem("worker_uncertain", "invalid stored thread response")
	}

	if value.Thread.ID != *binding.ThreadID || value.Thread.CWD != binding.ProjectPath || len(value.Thread.Turns) == 0 {
		return codexStoredTurn{}, problem("worker_uncertain", "stored thread does not match the binding")
	}

	turn := value.Thread.Turns[len(value.Thread.Turns)-1]
	if turn.ID != *binding.TurnID || (turn.ItemsView != "" && turn.ItemsView != "full") {
		return codexStoredTurn{}, problem("worker_uncertain", "recorded turn is missing or incomplete")
	}

	return turn, nil
}
