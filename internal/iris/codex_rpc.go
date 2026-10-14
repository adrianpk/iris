package iris

import (
	"context"
	"encoding/json"
)

type codexHandler func(context.Context, codexFrame) error

func (client *codexClient) write(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if len(encoded) > maxResponseBytes {
		return problem("limit_exceeded", "Codex request exceeds the frame limit")
	}

	_, err = client.input.Write(append(encoded, '\n'))

	return err
}

func (client *codexClient) read(ctx context.Context) (codexFrame, error) {
	select {
	case <-ctx.Done():
		return codexFrame{}, ctx.Err()
	case value, ok := <-client.frames:
		if !ok {
			return codexFrame{}, problem("worker_uncertain", "Codex disconnected before confirmation")
		}

		if value.err != nil {
			return codexFrame{}, problem("worker_uncertain", "invalid Codex frame")
		}

		return value.frame, nil
	}
}

func (client *codexClient) call(ctx context.Context, method string, params any, handle codexHandler) (json.RawMessage, error) {
	client.nextID++

	id := client.nextID
	if err := client.writeContext(ctx, map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}

	for {
		frame, err := client.read(ctx)
		if err != nil {
			return nil, err
		}

		if frame.Method != "" {
			if err := handle(ctx, frame); err != nil {
				return nil, err
			}

			continue
		}

		if string(frame.ID) != jsonNumber(id) {
			return nil, problem("worker_uncertain", "unexpected Codex response identity")
		}

		if frame.Error != nil {
			return nil, codexRequestFailure(frame.Error)
		}

		return frame.Result, nil
	}
}

func jsonNumber(value int) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return string(encoded)
}

func (client *codexClient) rejectRequests(ctx context.Context, frame codexFrame) error {
	if len(frame.ID) == 0 {
		return nil
	}

	return client.writeContext(ctx, map[string]any{"id": frame.ID, "error": codexRPCError{Code: -32601, Message: "unsupported client request"}})
}
