package iris

import (
	"context"
	"database/sql"
	"errors"
)

type lifecycleState struct {
	id           string
	state        string
	created      int64
	expires      int64
	activity     int64
	nextSequence int64
	messageCount int64
	bodyBytes    int64
}

func (s *store) loadLifecycle(ctx context.Context, tx *sql.Tx, id string) (lifecycleState, error) {
	value := lifecycleState{id: id}

	err := tx.QueryRowContext(ctx, `SELECT c.state,c.created_at,c.last_activity_at,c.next_sequence,c.message_count,c.body_bytes,coalesce(i.expires_at,c.created_at+?) FROM conversations c LEFT JOIN invitations i ON i.conversation_id=c.id WHERE c.id=?`, s.policy.invitationLifetime().Milliseconds(), id).
		Scan(&value.state, &value.created, &value.activity, &value.nextSequence, &value.messageCount, &value.bodyBytes, &value.expires)
	if errors.Is(err, sql.ErrNoRows) {
		return value, problem("not_found", "conversation is not retained")
	}

	return value, err
}

func (s *store) expireIfDue(ctx context.Context, tx *sql.Tx, value *lifecycleState) error {
	due := value.state == "pending" && s.now().UnixMilli() >= value.expires

	due = due || (value.state == "active" && s.now().UnixMilli() >= value.activity+s.policy.inactivityLifetime().Milliseconds())
	if !due {
		return nil
	}

	return s.finishConversation(ctx, tx, value, "expired")
}

func (s *store) finishConversation(ctx context.Context, tx *sql.Tx, value *lifecycleState, state string) error {
	if _, err := tx.ExecContext(ctx, "UPDATE conversations SET state=?,terminal_at=? WHERE id=?", state, s.now().UnixMilli(), value.id); err != nil {
		return err
	}

	if err := s.appendSystemEvent(ctx, tx, value, "Conversation "+state+".", true); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET stop_requested_at=COALESCE(stop_requested_at,?),updated_at=? WHERE conversation_id=? AND owner_token IS NOT NULL", s.now().UnixMilli(), s.now().UnixMilli(), value.id); err != nil {
		return err
	}

	value.state = state

	return nil
}

func (s *store) appendSystemEvent(ctx context.Context, tx *sql.Tx, value *lifecycleState, body string, terminal bool) error {
	maximum := int64(maxConversationMessages - 1)
	if terminal {
		maximum++
	}

	if value.messageCount >= maximum {
		return problem("limit_exceeded", "conversation has reached its message limit")
	}

	if value.bodyBytes+int64(len(body)) > maxConversationBodyBytes {
		body = ""
	}

	id, err := newID()
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO messages
        (id,conversation_id,sequence,sender_project_id,sender_alias,created_at,kind,body,payload_digest)
        VALUES (?,?,?,NULL,'iris',?,'system',?,?)`, id, value.id, value.nextSequence, s.now().UnixMilli(), body, textDigest(body))
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO message_recipients SELECT conversation_id,?,project_id FROM participants WHERE conversation_id=?`, id, value.id); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE conversations SET next_sequence=next_sequence+1,revision=revision+1,message_count=message_count+1,body_bytes=body_bytes+? WHERE id=?`, len(body), value.id)

	return err
}

func requireJoinedParticipant(ctx context.Context, tx *sql.Tx, id, projectID string) error {
	var count int

	err := tx.QueryRowContext(ctx, "SELECT count(*) FROM participants WHERE conversation_id=? AND project_id=? AND joined_at IS NOT NULL", id, projectID).Scan(&count)
	if err != nil {
		return err
	}

	if count != 1 {
		return problem("not_participant", "project has not joined this conversation")
	}

	return nil
}
