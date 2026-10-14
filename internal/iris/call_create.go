package iris

import (
	"context"
	"database/sql"
	"path/filepath"
)

func (s *store) callRecipient(ctx context.Context, tx *sql.Tx, selector string) (project, error) {
	recipient, err := exactProject(ctx, tx, selector)
	if err == nil || !hasCode(err, "project_not_found") {
		return recipient, err
	}

	if filepath.IsAbs(selector) {
		return project{}, err
	}

	id, err := newID()
	if err != nil {
		return project{}, err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO projects VALUES (?,NULL,?,?,NULL)", id, selector, s.now().UnixMilli())
	if err != nil {
		return project{}, err
	}

	return projectByID(ctx, tx, id)
}

func (s *store) checkConversationSlots(ctx context.Context, tx *sql.Tx, ids ...string) error {
	cutoff := s.now().Add(-s.policy.inactivityLifetime()).UnixMilli()

	for _, id := range ids {
		var count int

		err := tx.QueryRowContext(ctx, `SELECT count(*) FROM conversations c JOIN participants p ON p.conversation_id=c.id LEFT JOIN invitations i ON i.conversation_id=c.id
            WHERE p.project_id=? AND ((c.state='pending' AND COALESCE(i.expires_at,c.created_at+?)>?) OR (c.state='active' AND c.last_activity_at>?))`, id, s.policy.invitationLifetime().Milliseconds(), s.now().UnixMilli(), cutoff).Scan(&count)
		if err != nil {
			return err
		}

		if count >= maxActiveConversations {
			return problem("limit_exceeded", "project has too many pending or active conversations")
		}
	}

	return nil
}

func (s *store) createConversation(ctx context.Context, tx *sql.Tx, value *callCreation) error {
	low, high := value.initiator.ID, value.recipient.ID
	if low > high {
		low, high = high, low
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO pair_sequences VALUES (?,?,0) ON CONFLICT DO NOTHING`, low, high); err != nil {
		return err
	}

	err := tx.QueryRowContext(ctx, `UPDATE pair_sequences SET last_ordinal=last_ordinal+1 WHERE project_low=? AND project_high=? RETURNING last_ordinal`, low, high).Scan(&value.ordinal)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO conversations
        (id,project_low,project_high,pair_ordinal,topic,state,created_at,last_activity_at,next_sequence,message_count,body_bytes)
        VALUES (?,?,?,?,?,'pending',?,?,2,1,?)`, value.id, low, high, value.ordinal, value.plan.params.Topic, value.now, value.now, len(value.plan.params.Body))
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO participants VALUES (?,?,?,?)", value.id, value.initiator.ID, value.initiator.Alias, value.now); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO participants VALUES (?,?,?,NULL)", value.id, value.recipient.ID, value.recipient.Alias)

	return err
}

func (s *store) createInvitation(ctx context.Context, tx *sql.Tx, value callCreation) error {
	for range 16 {
		code, err := s.makeCode()
		if err != nil {
			return err
		}

		digest := textDigest(code)

		result, err := tx.ExecContext(ctx, "INSERT INTO invitation_code_tombstones VALUES (?,?) ON CONFLICT DO NOTHING", digest, value.now)
		if err != nil {
			return err
		}

		count, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if count == 0 {
			continue
		}

		expires := value.now + s.policy.invitationLifetime().Milliseconds()
		_, err = tx.ExecContext(ctx, "INSERT INTO invitations VALUES (?,?,?,?,?,NULL)", value.id, code, digest, value.recipient.ID, expires)

		return err
	}

	return problem("limit_exceeded", "could not allocate a fresh invitation code")
}

func createInitialMessage(ctx context.Context, tx *sql.Tx, value callCreation) error {
	id, err := newID()
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO messages
        (id,conversation_id,sequence,sender_project_id,sender_alias,created_at,kind,body,payload_digest)
        VALUES (?,?,1,?,?,?,'message',?,?)`, id, value.id, value.initiator.ID, value.initiator.Alias, value.now, value.plan.params.Body, textDigest(value.plan.params.Body))
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO message_recipients VALUES (?,?,?)", value.id, id, value.recipient.ID)

	return err
}
