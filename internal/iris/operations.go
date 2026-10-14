package iris

import (
	"context"
	"errors"

	"modernc.org/sqlite"
)

func problem(code, message string) *serviceError {
	return &serviceError{Code: code, Message: message}
}

func hasCode(err error, code string) bool {
	var service *serviceError

	return errors.As(err, &service) && service.Code == code
}

func operationResponse(ctx context.Context, value any, err error) response {
	if err == nil {
		return success(value)
	}

	if ctx.Err() != nil {
		return failure("cancelled", "request cancelled", false)
	}

	var service *serviceError
	if errors.As(err, &service) {
		return failure(service.Code, service.Message, service.Retryable)
	}

	var database *sqlite.Error
	if errors.As(err, &database) && (database.Code()&255 == 5 || database.Code()&255 == 6) {
		return failure("storage_busy", "storage is busy; retry the same request", true)
	}

	return failure("storage_unavailable", "operation could not access service storage", false)
}

func (s *store) handleProjects(ctx context.Context, req request) response {
	switch req.Operation {
	case "projects.register":
		var params projectRegistration
		if err := decodeJSON(req.Params, &params); err != nil {
			return failure("invalid_request", "invalid registration parameters", false)
		}

		value, err := s.registerProject(ctx, params)

		return operationResponse(ctx, value, err)
	case "projects.rename":
		var params projectRename
		if err := decodeJSON(req.Params, &params); err != nil {
			return failure("invalid_request", "invalid rename parameters", false)
		}

		value, err := s.renameProject(ctx, params)

		return operationResponse(ctx, value, err)
	case "projects.relocate":
		var params projectRelocation
		if err := decodeJSON(req.Params, &params); err != nil {
			return failure("invalid_request", "invalid relocation parameters", false)
		}

		value, err := s.relocateProject(ctx, params)

		return operationResponse(ctx, value, err)
	case "projects.list":
		var params projectListParams
		if err := decodeJSON(req.Params, &params); err != nil {
			return failure("invalid_request", "invalid project list parameters", false)
		}

		value, err := s.listProjects(ctx, params)

		return operationResponse(ctx, value, err)
	}

	return failure("unknown_operation", "unsupported project operation", false)
}
