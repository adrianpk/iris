package iris

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type recordedBridgeResult struct {
	SchemaVersion  int          `json:"schema_version"`
	InputMessageID string       `json:"input_message_id"`
	TurnID         string       `json:"turn_id"`
	Answer         bridgeAnswer `json:"answer"`
	UsageTokens    *int64       `json:"usage_tokens"`
}

func recordedResult(params bridgeCompletion) recordedBridgeResult {
	return recordedBridgeResult{SchemaVersion: 1, InputMessageID: params.InputMessageID, TurnID: params.TurnID, Answer: params.Answer, UsageTokens: params.UsageTokens}
}

func (value recordedBridgeResult) completion(owner bridgeOwner) bridgeCompletion {
	return bridgeCompletion{bridgeOwner: owner, InputMessageID: value.InputMessageID, TurnID: value.TurnID, Answer: value.Answer, UsageTokens: value.UsageTokens}
}

func (s *store) recordBridgeResult(ctx context.Context, params bridgeCompletion) (bridgeBinding, error) {
	if err := params.validate(); err != nil {
		return bridgeBinding{}, err
	}

	var result bridgeRecord

	err := s.write(ctx, func(tx *sql.Tx) error {
		value, err := s.ownedBridge(ctx, tx, params.bridgeOwner)
		if err != nil {
			return err
		}

		if err := s.recordBridgeResultTx(ctx, tx, value, params); err != nil {
			return err
		}

		result, err = readBridge(ctx, tx, "b.id=?", value.ID)

		return err
	})

	return result.bridgeBinding, err
}

func (s *store) recordBridgeResultTx(ctx context.Context, tx *sql.Tx, value bridgeRecord, params bridgeCompletion) error {
	if value.State == "result_recorded" {
		return matchRecordedResult(value, params)
	}

	if value.State != "invoking" && value.State != "uncertain" {
		return problem("worker_uncertain", "result does not match recoverable invocation")
	}

	if value.PendingRequestID != nil {
		return problem("approval_required", "disconnected approval cannot become a completed result")
	}

	input, err := nextBridgeInput(ctx, tx, value, params.InputMessageID)
	if err != nil {
		return err
	}

	invoking := value

	invoking.State = "invoking"
	if err := validateBridgeCompletion(invoking, params, input); err != nil {
		return err
	}

	encoded, err := json.Marshal(recordedResult(params))
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='result_recorded',pending_result_json=?,error_code=NULL,updated_at=? WHERE id=?", string(encoded), s.now().UnixMilli(), value.ID)

	return err
}

func matchRecordedResult(value bridgeRecord, params bridgeCompletion) error {
	if value.pendingResult == nil {
		return problem("worker_uncertain", "completed result checkpoint is missing")
	}

	original, err := jsonDigest(value.pendingResult)
	if err != nil {
		return err
	}

	incoming, err := jsonDigest(recordedResult(params))
	if err != nil {
		return err
	}

	if original != incoming {
		return problem("idempotency_conflict", "completed result checkpoint differs")
	}

	return nil
}

func (s *store) recoverBridgeClaim(ctx context.Context, params conversationParams) (bridgeClaim, error) {
	return s.claimBridge(ctx, bridgeClaimParams{conversationParams: params, Options: defaultBridgeOptions(), Recover: true})
}

func (worker *bridgeWorker) recover(ctx context.Context) error {
	if worker.claim.PendingResult != nil {
		return worker.publishRecorded(ctx, *worker.claim.PendingResult)
	}

	if worker.binding.State == "result_recorded" {
		return problem("worker_uncertain", "durable result checkpoint is missing")
	}

	if worker.binding.State != "invoking" && worker.binding.State != "uncertain" && worker.binding.State != "approval_required" {
		return nil
	}

	return worker.recoverInvocation(ctx)
}

func (worker *bridgeWorker) recoverInvocation(ctx context.Context) error {
	if worker.binding.PendingRequestID != nil {
		return problem("approval_required", "prior connected approval cannot be resumed")
	}

	if worker.binding.ThreadID == nil || worker.binding.TurnID == nil || worker.binding.InputMessageID == nil {
		return problem("worker_uncertain", "invocation has no durable thread and turn identity")
	}

	if err := worker.connectCodex(ctx); err != nil {
		return err
	}

	inspect, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := worker.client.readResult(inspect, worker.binding)
	if err != nil {
		return err
	}

	completion := bridgeCompletion{bridgeOwner: worker.owner, InputMessageID: *worker.binding.InputMessageID, TurnID: result.TurnID, Answer: result.Answer, UsageTokens: result.Usage}

	return worker.recordAndPublish(ctx, completion)
}

func (worker *bridgeWorker) recordAndPublish(ctx context.Context, completion bridgeCompletion) error {
	value, err := serviceValue[bridgeBinding](ctx, worker.config, "bridge.record", completion)
	if err != nil {
		return err
	}

	worker.binding = value

	return worker.publishRecorded(ctx, recordedResult(completion))
}

func (worker *bridgeWorker) publishRecorded(ctx context.Context, result recordedBridgeResult) error {
	value, err := serviceValue[bridgeBinding](ctx, worker.config, "bridge.complete", result.completion(worker.owner))
	if err == nil {
		worker.binding = value
	}

	return err
}
