package iris

import (
	"context"
	"database/sql"
	"errors"
)

type joinParams struct {
	projectSelection
	Code string `json:"code"`
}

type invitation struct {
	conversationID    string
	expectedProjectID string
	acceptedAt        sql.NullInt64
}

func readInvitation(ctx context.Context, tx *sql.Tx, code string) (invitation, error) {
	var value invitation

	err := tx.QueryRowContext(ctx, "SELECT conversation_id,expected_project_id,accepted_at FROM invitations WHERE code=?", code).
		Scan(&value.conversationID, &value.expectedProjectID, &value.acceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return value, problem("not_found", "invitation is not retained")
	}

	return value, err
}

func (s *store) join(ctx context.Context, params joinParams) (conversation, error) {
	if !validInvitationCode(params.Code) {
		return conversation{}, problem("invalid_request", "invitation must contain three lowercase words")
	}

	plan, err := s.planSelection(ctx, params.projectSelection, true)
	if err != nil {
		return conversation{}, err
	}

	var outcome handshakeOutcome

	err = s.write(ctx, func(tx *sql.Tx) error { outcome, err = s.joinTx(ctx, tx, plan, params.Code); return err })
	if err != nil {
		return conversation{}, err
	}

	s.events.notify(outcome.value.ID)

	return outcome.value, outcome.problem
}

type handshakeOutcome struct {
	value   conversation
	problem error
}

func (s *store) joinTx(ctx context.Context, tx *sql.Tx, plan selectionPlan, code string) (handshakeOutcome, error) {
	invite, err := readInvitation(ctx, tx, code)
	if err != nil {
		return handshakeOutcome{}, err
	}

	if err := validateJoinProject(ctx, tx, plan, invite.expectedProjectID); err != nil {
		return handshakeOutcome{}, err
	}

	state, err := loadLifecycle(ctx, tx, invite.conversationID)
	if err != nil {
		return handshakeOutcome{}, err
	}

	if err := s.expireIfDue(ctx, tx, &state); err != nil {
		return handshakeOutcome{}, err
	}

	if state.state == "closed" || state.state == "expired" {
		return handshakeOutcome{value: conversation{ID: state.id}, problem: joinStateError(state, invite)}, nil
	}

	if state.state == "pending" {
		if err := s.acceptInvitation(ctx, tx, plan, invite, &state); err != nil {
			return handshakeOutcome{}, err
		}
	}

	result, err := readConversation(ctx, tx, state.id)

	return handshakeOutcome{value: result}, err
}

func joinStateError(state lifecycleState, invite invitation) error {
	if state.state == "expired" && !invite.acceptedAt.Valid {
		return problem("invitation_expired", "invitation has expired")
	}

	if state.state == "closed" || state.state == "expired" {
		return problem("conversation_inactive", "conversation is closed or expired")
	}

	return nil
}

func validateJoinProject(ctx context.Context, tx *sql.Tx, plan selectionPlan, expected string) error {
	var (
		value project
		err   error
	)
	if plan.selector != "" {
		value, err = exactProject(ctx, tx, plan.selector)
	} else {
		value, err = nearestProject(ctx, tx, plan.directory)
	}

	if hasCode(err, "project_not_found") && plan.registration != nil {
		value, err = projectByID(ctx, tx, expected)
		if err != nil {
			return err
		}

		if value.Path == nil && value.Alias == plan.registration.Name {
			return nil
		}

		return problem("recipient_mismatch", "project does not match the invited recipient")
	}

	if err != nil {
		return err
	}

	if value.ID != expected || value.Path == nil {
		return problem("recipient_mismatch", "project does not match the invited recipient")
	}

	return nil
}

func (s *store) acceptInvitation(ctx context.Context, tx *sql.Tx, plan selectionPlan, invite invitation, state *lifecycleState) error {
	value, err := s.selectProjectTx(ctx, tx, plan)
	if err != nil {
		return err
	}

	if value.ID != invite.expectedProjectID {
		return problem("recipient_mismatch", "project does not match the invited recipient")
	}

	now := s.now().UnixMilli()
	if _, err := tx.ExecContext(ctx, "UPDATE participants SET joined_at=? WHERE conversation_id=? AND project_id=? AND joined_at IS NULL", now, state.id, value.ID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE invitations SET accepted_at=? WHERE conversation_id=? AND accepted_at IS NULL", now, state.id); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE conversations SET state='active',last_activity_at=? WHERE id=? AND state='pending'", now, state.id); err != nil {
		return err
	}

	return s.appendSystemEvent(ctx, tx, state, "Conversation joined.", false)
}
