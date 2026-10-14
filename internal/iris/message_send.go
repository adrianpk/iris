package iris

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
)

type sendParams struct {
	conversationParams
	Body           string   `json:"body"`
	Kind           string   `json:"kind,omitempty"`
	Recipients     []string `json:"recipients,omitempty"`
	ReplyTo        *string  `json:"reply_to,omitempty"`
	CorrelationID  *string  `json:"correlation_id,omitempty"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
}

type sendOutcome struct {
	value   message
	problem error
}

type messageCreation struct {
	params sendParams
	sender project
	state  lifecycleState
	digest string
}

func prepareSend(params sendParams) (sendParams, error) {
	if params.Kind == "" {
		params.Kind = "message"
	}

	if !validID(params.ConversationID) || !validText(params.Body, maxMessageBodyBytes) || (params.Kind != "message" && params.Kind != "status") {
		return params, problem("invalid_request", "send requires a conversation, nonempty UTF-8 body and message/status kind within limits")
	}

	if err := validateMessageOptions(params); err != nil {
		return params, err
	}

	if len(params.Recipients) > 2 {
		return params, problem("recipient_mismatch", "recipients must belong to the conversation")
	}

	params.Recipients = slices.Clone(params.Recipients)
	slices.Sort(params.Recipients)

	if len(slices.Compact(slices.Clone(params.Recipients))) != len(params.Recipients) {
		return params, problem("invalid_request", "duplicate recipients are not allowed")
	}

	return params, nil
}

func (s *store) send(ctx context.Context, params sendParams) (message, error) {
	params, err := prepareSend(params)
	if err != nil {
		return message{}, err
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return message{}, err
	}

	var outcome sendOutcome

	err = s.write(ctx, func(tx *sql.Tx) error {
		sender, err := s.selectProjectTx(ctx, tx, plan)
		if err != nil {
			return err
		}

		outcome, err = s.sendTx(ctx, tx, params, sender)

		return err
	})
	if err != nil {
		return message{}, err
	}

	s.events.notify(params.ConversationID)

	return outcome.value, outcome.problem
}

func (s *store) sendTx(ctx context.Context, tx *sql.Tx, params sendParams, sender project) (sendOutcome, error) {
	state, err := loadLifecycle(ctx, tx, params.ConversationID)
	if err != nil {
		return sendOutcome{}, err
	}

	if err := requireJoinedParticipant(ctx, tx, state.id, sender.ID); err != nil {
		return sendOutcome{}, err
	}

	params, err = resolveMessageRecipients(ctx, tx, params, sender.ID)
	if err != nil {
		return sendOutcome{}, err
	}

	encoded, err := json.Marshal([]any{params.Kind, params.Body, params.Recipients, params.ReplyTo, params.CorrelationID})
	if err != nil {
		return sendOutcome{}, err
	}

	creation := messageCreation{params: params, sender: sender, state: state, digest: textDigest(string(encoded))}

	original, found, err := findSentMessage(ctx, tx, creation)
	if err != nil {
		return sendOutcome{}, err
	}

	if err := s.expireIfDue(ctx, tx, &state); err != nil {
		return sendOutcome{}, err
	}

	if found {
		return sendOutcome{value: original}, nil
	}

	if state.state != "active" {
		return sendOutcome{problem: problem("conversation_inactive", "ordinary messages require an active conversation")}, nil
	}

	value, err := s.insertParticipantMessage(ctx, tx, creation)

	return sendOutcome{value: value}, err
}

func validateMessageOptions(params sendParams) error {
	for _, value := range []*string{params.CorrelationID, params.IdempotencyKey} {
		if value != nil && !validText(*value, 128) {
			return problem("invalid_request", "keys and correlation IDs must contain 1 through 128 UTF-8 bytes")
		}
	}

	if params.ReplyTo != nil && !validID(*params.ReplyTo) {
		return problem("invalid_request", "reply target must be a message ID")
	}

	return nil
}
