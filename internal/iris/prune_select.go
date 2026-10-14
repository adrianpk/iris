package iris

import (
	"context"
	"database/sql"
	"errors"
)

func selectPruneCandidates(ctx context.Context, tx *sql.Tx, params pruneParams, projectID *string, cutoff int64) (values []pruneCandidate, resultErr error) {
	query := `SELECT c.id,c.state,c.revision,c.terminal_at,c.message_count,c.body_bytes FROM conversations c WHERE c.state IN ('closed','expired') AND c.terminal_at<=?`
	args := []any{cutoff}

	if projectID != nil {
		query += " AND EXISTS (SELECT 1 FROM participants p WHERE p.conversation_id=c.id AND p.project_id=?)"

		args = append(args, *projectID)
	}

	if params.ConversationID != "" {
		query += " AND c.id=?"

		args = append(args, params.ConversationID)
	}

	if params.State != "" {
		query += " AND c.state=?"

		args = append(args, params.State)
	}

	if params.Before != nil {
		query += " AND c.terminal_at<=?"

		args = append(args, params.Before.UnixMilli())
	}

	query += " ORDER BY c.id LIMIT ?"

	args = append(args, maxPruneCandidates+1)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	values = []pruneCandidate{}

	for rows.Next() {
		var (
			value    pruneCandidate
			terminal int64
		)
		if err := rows.Scan(&value.ConversationID, &value.State, &value.Revision, &terminal, &value.MessageCount, &value.BodyBytes); err != nil {
			return nil, err
		}

		value.TerminalAt = milliseconds(terminal)
		values = append(values, value)
	}

	if len(values) > maxPruneCandidates {
		return nil, problem("limit_exceeded", "prune preview exceeds 1000 candidates; narrow the selection")
	}

	return values, rows.Err()
}

func (s *store) reservePrunePreview(ctx context.Context, tx *sql.Tx, value prunePreview) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM prune_previews WHERE expires_at<=?", s.now().UnixMilli()); err != nil {
		return err
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM prune_previews").Scan(&count); err != nil {
		return err
	}

	if count >= maxPrunePreviews {
		return problem("limit_exceeded", "too many live prune previews")
	}

	selection, err := pruneSelectionDigest(value.Candidates)
	if err != nil {
		return err
	}

	policy, err := jsonDigest(s.policy)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO prune_previews VALUES (?,?,?,?,?)", textDigest(value.Token), selection, policy, value.Cutoff.UnixMilli(), value.ExpiresAt.UnixMilli())

	return err
}
