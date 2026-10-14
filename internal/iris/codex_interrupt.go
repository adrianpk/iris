package iris

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

func (progress *codexProgress) interrupt(ctx context.Context) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	confirmed := false
	handle := func(ctx context.Context, frame codexFrame) error {
		if len(frame.ID) != 0 {
			return progress.client.rejectRequests(ctx, frame)
		}

		var event codexEvent
		if err := json.Unmarshal(frame.Params, &event); err != nil {
			return err
		}

		if progress.matchesInterruption(frame, event) {
			confirmed = true
		}

		return nil
	}

	_, err := progress.client.call(cleanup, "turn/interrupt", map[string]any{"threadId": progress.threadID, "turnId": progress.turnID}, handle)
	if err != nil {
		return err
	}

	for !confirmed {
		frame, err := progress.client.read(cleanup)
		if err != nil {
			return err
		}

		if err := handle(cleanup, frame); err != nil {
			return err
		}
	}

	return nil
}

func (progress *codexProgress) interrupted(ctx context.Context, err error) error {
	if err == nil || progress.completed || (progress.turnID == "" && errors.Is(err, errCodexRejected)) {
		return err
	}

	if progress.turnID == "" {
		return problem("worker_uncertain", "Codex invocation was not confirmed")
	}

	if interruptErr := progress.interrupt(ctx); interruptErr != nil {
		return problem("worker_uncertain", "Codex interruption could not be confirmed")
	}

	if hasCode(err, "worker_failed") || hasCode(err, "approval_denied") {
		return err
	}

	return &serviceError{Code: "worker_interrupted", Message: "owned Codex turn was interrupted before processing completed", cause: err}
}

func (progress *codexProgress) matchesInterruption(frame codexFrame, event codexEvent) bool {
	return frame.Method == "turn/completed" && event.ThreadID == progress.threadID && event.Turn.ID == progress.turnID && event.Turn.Status == "interrupted"
}
