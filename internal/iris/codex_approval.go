package iris

import (
	"context"
	"encoding/json"
)

type codexApproval func(context.Context, codexFrame) (bool, error)

func (progress *codexProgress) approval(ctx context.Context, frame codexFrame) error {
	var event codexEvent
	if err := json.Unmarshal(frame.Params, &event); err != nil || event.ThreadID != progress.threadID || event.TurnID != progress.turnID || progress.turnID == "" {
		progress.denied = true

		return progress.client.rejectRequests(ctx, frame)
	}

	pending := bridgeCheckpoint{Action: "approval", TurnID: progress.turnID, PendingRequestID: string(frame.ID), PendingMethod: frame.Method}
	if err := progress.observe(ctx, pending); err != nil {
		return err
	}

	accepted, err := progress.approvalDecision(ctx, frame)
	if err != nil {
		return err
	}

	if !accepted {
		progress.denied = true
	}

	if err := progress.approvalReply(ctx, frame, accepted); err != nil {
		return err
	}

	running := bridgeCheckpoint{Action: "running", TurnID: progress.turnID}
	if !accepted {
		running.ErrorCode = "approval_denied"
	}

	return progress.observe(ctx, running)
}

func (progress *codexProgress) approvalDecision(ctx context.Context, frame codexFrame) (bool, error) {
	if frame.Method != "item/commandExecution/requestApproval" && frame.Method != "item/fileChange/requestApproval" {
		return false, nil
	}

	if progress.client.approve == nil {
		return false, nil
	}

	return progress.client.approve(ctx, frame)
}

func (progress *codexProgress) approvalReply(ctx context.Context, frame codexFrame, accepted bool) error {
	switch frame.Method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		decision := "decline"
		if accepted {
			decision = "accept"
		}

		return progress.client.write(map[string]any{"id": frame.ID, "result": map[string]any{"decision": decision}})
	case "item/permissions/requestApproval":
		return progress.client.write(map[string]any{"id": frame.ID, "result": map[string]any{"permissions": map[string]any{}, "scope": "turn"}})
	}

	return progress.client.rejectRequests(ctx, frame)
}
