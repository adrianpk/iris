package iris

import (
	"context"
	"database/sql"
)

type conversationParams struct {
	projectSelection
	ConversationID string `json:"conversation_id"`
}

func (s *store) closeConversation(ctx context.Context, params conversationParams) (conversation, error) {
	if !validID(params.ConversationID) {
		return conversation{}, problem("invalid_request", "close requires a conversation ID")
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return conversation{}, err
	}

	var outcome handshakeOutcome

	err = s.write(ctx, func(tx *sql.Tx) error {
		outcome, err = s.closeConversationTx(ctx, tx, plan, params.ConversationID)

		return err
	})
	if err != nil {
		return conversation{}, err
	}

	s.events.notify(params.ConversationID)

	return outcome.value, outcome.problem
}

func (s *store) closeConversationTx(ctx context.Context, tx *sql.Tx, plan selectionPlan, id string) (handshakeOutcome, error) {
	value, err := s.selectProjectTx(ctx, tx, plan)
	if err != nil {
		return handshakeOutcome{}, err
	}

	state, err := s.loadLifecycle(ctx, tx, id)
	if err != nil {
		return handshakeOutcome{}, err
	}

	if err := requireJoinedParticipant(ctx, tx, state.id, value.ID); err != nil {
		return handshakeOutcome{}, err
	}

	if err := s.expireIfDue(ctx, tx, &state); err != nil {
		return handshakeOutcome{}, err
	}

	switch state.state {
	case "expired":
		return handshakeOutcome{problem: problem("conversation_inactive", "conversation has expired")}, nil
	case "closed":
	default:
		if err := s.finishConversation(ctx, tx, &state, "closed"); err != nil {
			return handshakeOutcome{}, err
		}
	}

	result, err := readConversation(ctx, tx, state.id)

	return handshakeOutcome{value: result}, err
}
