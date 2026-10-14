package iris

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"unicode/utf8"
)

type callParams struct {
	projectSelection
	To         string `json:"to"`
	Topic      string `json:"topic"`
	Body       string `json:"body"`
	RequestKey string `json:"request_key"`
}

type callPlan struct {
	params    callParams
	selection selectionPlan
	digest    string
}

type callCreation struct {
	plan      callPlan
	initiator project
	recipient project
	id        string
	ordinal   int64
	now       int64
}

func (s *store) prepareCall(ctx context.Context, params callParams) (callPlan, error) {
	if !validText(params.Topic, 256) || !validText(params.Body, maxMessageBodyBytes) || !validText(params.RequestKey, 128) || params.To == "" {
		return callPlan{}, problem("invalid_request", "call requires recipient, topic, nonempty UTF-8 body and request key within limits")
	}

	recipient, err := explicitSelection(params.To)
	if err != nil {
		return callPlan{}, err
	}

	params.To = recipient.selector
	if !filepath.IsAbs(params.To) && !validAlias(params.To) {
		return callPlan{}, problem("invalid_request", "recipient must be an alias, ID or canonical directory")
	}

	selection, err := s.planSelection(ctx, params.projectSelection, true)
	if err != nil {
		return callPlan{}, err
	}

	payload, err := json.Marshal([]string{params.To, params.Topic, params.Body})
	if err != nil {
		return callPlan{}, err
	}

	return callPlan{params: params, selection: selection, digest: textDigest(string(payload))}, nil
}

func validText(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum && utf8.ValidString(value)
}

func (s *store) call(ctx context.Context, params callParams) (conversation, error) {
	plan, err := s.prepareCall(ctx, params)
	if err != nil {
		return conversation{}, err
	}

	var result conversation

	err = s.write(ctx, func(tx *sql.Tx) error {
		initiator, err := s.selectProjectTx(ctx, tx, plan.selection)
		if err != nil {
			return err
		}

		id, err := s.callConversationTx(ctx, tx, plan, initiator)
		if err != nil {
			return err
		}

		result, err = readConversation(ctx, tx, id)

		return err
	})

	return result, err
}

func (s *store) callConversationTx(ctx context.Context, tx *sql.Tx, plan callPlan, initiator project) (string, error) {
	existing, found, err := findCallRequest(ctx, tx, initiator.ID, plan)
	if err != nil || found {
		return existing, err
	}

	recipient, err := s.callRecipient(ctx, tx, plan.params.To)
	if err != nil {
		return "", err
	}

	if recipient.ID == initiator.ID {
		return "", problem("recipient_mismatch", "conversation requires two distinct projects")
	}

	if err := s.checkConversationSlots(ctx, tx, initiator.ID, recipient.ID); err != nil {
		return "", err
	}

	id, err := newID()
	if err != nil {
		return "", err
	}

	creation := callCreation{plan: plan, initiator: initiator, recipient: recipient, id: id, now: s.now().UnixMilli()}
	if err := s.createConversation(ctx, tx, &creation); err != nil {
		return "", err
	}

	if err := s.createInvitation(ctx, tx, creation); err != nil {
		return "", err
	}

	if err := createInitialMessage(ctx, tx, creation); err != nil {
		return "", err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO call_requests VALUES (?,?,?,?,?)", initiator.ID, plan.params.RequestKey, plan.digest, id, creation.now)

	return id, err
}

func findCallRequest(ctx context.Context, tx *sql.Tx, initiator string, plan callPlan) (string, bool, error) {
	var (
		digest string
		id     sql.NullString
	)

	err := tx.QueryRowContext(ctx, "SELECT payload_digest,conversation_id FROM call_requests WHERE initiator_project_id=? AND request_key=?", initiator, plan.params.RequestKey).Scan(&digest, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}

	if err != nil {
		return "", false, err
	}

	if digest != plan.digest {
		return "", true, problem("idempotency_conflict", "call key was used with different content")
	}

	if !id.Valid {
		return "", true, problem("not_found", "original conversation is no longer retained")
	}

	return id.String, true, nil
}
