package iris

import (
	"encoding/json"
	"errors"
)

var errCodexRejected = errors.New("codex rejected request before execution")

func codexRequestFailure(value *codexRPCError) error {
	var data struct {
		CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
	}

	code := "worker_failed"
	if value.Code == -32001 {
		code = "provider_capacity"
	}

	if json.Unmarshal(value.Data, &data) == nil && codexCapacityInfo(data.CodexErrorInfo) {
		code = "provider_capacity"
	}

	return &serviceError{Code: code, Message: "Codex rejected the protocol request", Retryable: code == "provider_capacity", cause: errCodexRejected}
}

func codexCapacityInfo(raw json.RawMessage) bool {
	var info string
	if json.Unmarshal(raw, &info) != nil {
		return false
	}

	switch info {
	case "usageLimitExceeded", "rateLimitExceeded", "flexUnavailable", "serverOverloaded":
		return true
	}

	return false
}
