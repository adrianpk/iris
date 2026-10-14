package iris

import (
	"context"
	"database/sql"
	"errors"
)

func (s *store) expireBatch(ctx context.Context) (int, error) {
	var ids []string

	err := s.write(ctx, func(tx *sql.Tx) error {
		var err error

		ids, err = s.expirationCandidates(ctx, tx)
		if err != nil {
			return err
		}

		for _, id := range ids {
			value, err := s.loadLifecycle(ctx, tx, id)
			if err != nil {
				return err
			}

			if err := s.expireIfDue(ctx, tx, &value); err != nil {
				return err
			}
		}

		_, err = tx.ExecContext(ctx, "DELETE FROM prune_previews WHERE expires_at<=?", s.now().UnixMilli())

		return err
	})
	if err != nil {
		return 0, err
	}

	for _, id := range ids {
		s.events.notify(id)
	}

	return len(ids), nil
}

func (s *store) expirationCandidates(ctx context.Context, tx *sql.Tx) (ids []string, resultErr error) {
	now := s.now().UnixMilli()

	rows, err := tx.QueryContext(ctx, `SELECT id FROM (
 SELECT id FROM (SELECT c.id FROM invitations i JOIN conversations c ON c.id=i.conversation_id WHERE c.state='pending' AND i.expires_at<=? ORDER BY i.expires_at,c.id LIMIT ?)
 UNION ALL SELECT id FROM (SELECT id FROM conversations WHERE state='active' AND last_activity_at<=? ORDER BY last_activity_at,id LIMIT ?)
 ) ORDER BY id LIMIT ?`, now, maintenanceBatchSize, now-s.policy.inactivityLifetime().Milliseconds(), maintenanceBatchSize, maintenanceBatchSize)
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
