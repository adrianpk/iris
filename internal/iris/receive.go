package iris

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

func (s *store) receive(ctx context.Context, params receiveParams) (receivePage, error) {
	params, err := prepareReceive(params)
	if err != nil {
		return receivePage{}, err
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return receivePage{}, err
	}

	page, err := s.receiveOnce(ctx, params, plan)
	if err != nil {
		return receivePage{}, err
	}

	return s.waitForDelivery(ctx, params, plan, page)
}

func (s *store) receiveOnce(ctx context.Context, params receiveParams, plan selectionPlan) (receivePage, error) {
	var outcome receiveOutcome

	err := s.write(ctx, func(tx *sql.Tx) error { var err error; outcome, err = s.receiveTx(ctx, tx, params, plan); return err })
	if err != nil {
		return receivePage{}, err
	}

	if outcome.problem != nil {
		s.notify(params.ConversationID)
	}

	return outcome.page, outcome.problem
}

func (s *store) receiveTx(ctx context.Context, tx *sql.Tx, params receiveParams, plan selectionPlan) (receiveOutcome, error) {
	selected, err := s.selectProjectTx(ctx, tx, plan)
	if err != nil {
		return receiveOutcome{}, err
	}

	state, err := s.loadLifecycle(ctx, tx, params.ConversationID)
	if err != nil {
		return receiveOutcome{}, err
	}

	if err := requireJoinedParticipant(ctx, tx, state.id, selected.ID); err != nil {
		return receiveOutcome{}, err
	}

	if err := s.expireIfDue(ctx, tx, &state); err != nil {
		return receiveOutcome{}, err
	}

	if state.state == "closed" || state.state == "expired" {
		return receiveOutcome{problem: problem("conversation_inactive", "closed or expired history is unavailable to receive")}, nil
	}

	position, err := loadConsumer(ctx, tx, consumerPosition{ConversationID: state.id, ProjectID: selected.ID, ConsumerID: params.ConsumerID})
	if err != nil {
		return receiveOutcome{}, err
	}

	after, err := cursorAfter(params.Cursor, position, state.nextSequence-1)
	if err != nil {
		return receiveOutcome{}, err
	}

	page, err := readDeliveryPage(ctx, tx, position, after, params.Limit)
	if err != nil {
		return receiveOutcome{}, err
	}

	_, err = tx.ExecContext(ctx, "UPDATE consumer_cursors SET delivered_sequence=? WHERE conversation_id=? AND project_id=? AND consumer_id=?", page.DeliveredSequence, state.id, selected.ID, params.ConsumerID)

	return receiveOutcome{page: page}, err
}

func deliveryMessageIDs(ctx context.Context, tx *sql.Tx, position consumerPosition, after int64, limit int) (ids []string, resultErr error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.id FROM messages m JOIN message_recipients r ON r.message_id=m.id
 WHERE m.conversation_id=? AND r.project_id=? AND m.sequence>? ORDER BY m.sequence LIMIT ?`, position.ConversationID, position.ProjectID, after, limit)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func readDeliveryPage(ctx context.Context, tx *sql.Tx, position consumerPosition, after int64, limit int) (receivePage, error) {
	ids, err := deliveryMessageIDs(ctx, tx, position, after, limit+1)
	if err != nil {
		return receivePage{}, err
	}

	page := receivePage{consumerPosition: position, Messages: []message{}}
	size := 0

	for _, id := range ids {
		value, err := readMessage(ctx, tx, id)
		if err != nil {
			return receivePage{}, err
		}

		encoded, err := json.Marshal(value)
		if err != nil {
			return receivePage{}, err
		}

		if len(page.Messages) == limit || size+len(encoded) > maxResponseBytes-(64<<10) {
			break
		}

		size += len(encoded) + 1

		page.Messages = append(page.Messages, value)
		page.DeliveredSequence = max(page.DeliveredSequence, value.Sequence)
	}

	page.Truncated = len(ids) > len(page.Messages)
	if len(page.Messages) > 0 {
		cursor, err := encodeCursor(position, page.Messages[len(page.Messages)-1].Sequence)
		if err != nil {
			return receivePage{}, err
		}

		page.NextCursor = &cursor
	}

	return page, nil
}
