package iris

import (
	"context"
	"encoding/json"
)

func requestService(ctx context.Context, cfg config, operation string, params any) (response, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return response{}, err
	}

	return callService(ctx, cfg, request{Protocol: protocolVersion, Operation: operation, Params: data})
}

func serviceValue[T any](ctx context.Context, cfg config, operation string, params any) (T, error) {
	var value T

	reply, err := requestService(ctx, cfg, operation, params)
	if err != nil {
		return value, err
	}

	if err := decodeJSON(reply.Result, &value); err != nil {
		return value, problem("internal_error", "invalid typed service result")
	}

	return value, nil
}
