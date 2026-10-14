package iris

import (
	"context"
	"database/sql"
	"errors"
)

func loadPruneConfirmation(ctx context.Context, tx *sql.Tx, token, digest string) (values []pruneCandidate, resultErr error) {
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.state,c.revision,COALESCE(c.terminal_at,0),c.message_count,c.body_bytes,p.revision,p.terminal_at
 FROM prune_candidates p JOIN conversations c ON c.id=p.conversation_id WHERE p.token_digest=? ORDER BY c.id LIMIT ?`, textDigest(token), maxPruneCandidates+1)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	values = []pruneCandidate{}

	for rows.Next() {
		var (
			value                             pruneCandidate
			terminal, revision, savedTerminal int64
		)
		if err := rows.Scan(&value.ConversationID, &value.State, &value.Revision, &terminal, &value.MessageCount, &value.BodyBytes, &revision, &savedTerminal); err != nil {
			return nil, err
		}

		if value.Revision != revision || terminal != savedTerminal {
			return nil, stalePrunePreview()
		}

		value.TerminalAt = milliseconds(terminal)
		values = append(values, value)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	actual, err := pruneSelectionDigest(values)
	if err != nil {
		return nil, err
	}

	if len(values) > maxPruneCandidates || actual != digest {
		return nil, stalePrunePreview()
	}

	return values, nil
}
