package iris

import "context"

func (s *store) handleHistory(ctx context.Context, req request) response {
	switch req.Operation {
	case "conversations", "search":
		return typedOperation(ctx, req, s.listConversations)
	case "show":
		return typedOperation(ctx, req, s.showConversation)
	case "dump.open":
		return typedOperation(ctx, req, s.openDump)
	case "dump.page":
		return typedOperation(ctx, req, s.pageDump)
	case "dump.close":
		return typedOperation(ctx, req, s.closeDump)
	}

	return failure("unknown_operation", "unsupported history operation", false)
}
