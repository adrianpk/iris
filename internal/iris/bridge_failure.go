package iris

import "time"

func publicBridgeStatus(value bridgeRecord, now time.Time) bridgeBinding {
	result := value.bridgeBinding
	if value.LeaseUntil != nil && !now.Before(*value.LeaseUntil) && (value.State == "invoking" || value.State == "approval_required") {
		result.State = "uncertain"
		code := "worker_uncertain"
		result.ErrorCode = &code
	}

	hint := bridgeRecoveryHint(result.State)
	if hint != "" {
		result.RecoveryHint = &hint
	}

	return result
}

func bridgeRecoveryHint(state string) string {
	switch state {
	case "result_recorded":
		return "Restart the bridge to publish its saved result without another invocation."
	case "uncertain":
		return "Run bridge recover to inspect only the recorded turn. If completion remains unconfirmed, inspect external effects and close this conversation before creating a deliberate replacement request."
	case "failed", "interrupted", "capacity":
		return "Input remains unacknowledged. Inspect external effects, then close this conversation and create a deliberate replacement request; automatic replay is blocked."
	case "approval_required":
		return "Approval requires its original connected human. A disconnected request cannot be granted by recovery."
	}

	return ""
}

func failedBridgeClaim(value bridgeRecord) error {
	code := "worker_failed"
	if value.State == "capacity" {
		code = "provider_capacity"
	}

	if value.State == "interrupted" {
		code = "worker_interrupted"
	}

	return problem(code, "prior unsuccessful execution remains unacknowledged; inspect its effects before deliberate replacement")
}

func checkBridgeAuthority(value bridgeBinding) error {
	if value.ErrorCode != nil && *value.ErrorCode == "approval_denied" {
		return problem("approval_denied", "prior authority request was declined; input remains unacknowledged")
	}

	return nil
}
