package iris

import (
	"context"
	"database/sql"
)

// completeDelivery is the atomic persistence primitive for a result and its input acknowledgment.
// Bridge ownership and invocation policy are checked by the future bridge operation before using it.
func (s *store) completeDelivery(ctx context.Context, params sendParams, input ackParams) (message, error) {
	params, err := prepareSend(params)
	if err != nil {
		return message{}, err
	}

	input, err = prepareAck(input)
	if err != nil {
		return message{}, err
	}

	if !validCompletion(params, input) {
		return message{}, problem("invalid_request", "completion requires one conversation, an input reply target and stable result key")
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
		if err != nil {
			return err
		}

		if outcome.problem == nil {
			_, err = acknowledgeTx(ctx, tx, input, sender.ID)
		}

		return err
	})
	if err != nil {
		return message{}, err
	}

	s.notify(params.ConversationID)

	return outcome.value, outcome.problem
}

func validCompletion(params sendParams, input ackParams) bool {
	return params.ConversationID == input.ConversationID && params.IdempotencyKey != nil && params.ReplyTo != nil && *params.ReplyTo == input.MessageID
}
