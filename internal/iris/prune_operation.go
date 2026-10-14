package iris

import "context"

func (s *store) handlePrune(ctx context.Context, req request) response {
	switch req.Operation {
	case "prune.preview":
		return typedOperation(ctx, req, s.previewPrune)
	case "prune.confirm":
		return typedOperation(ctx, req, s.confirmPrune)
	}

	return failure("unknown_operation", "unsupported prune operation", false)
}
