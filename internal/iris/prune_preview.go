package iris

import (
	"context"
	"database/sql"
)

func (s *store) previewPrune(ctx context.Context, params pruneParams) (prunePreview, error) {
	plan, err := s.planPrune(ctx, params)
	if err != nil {
		return prunePreview{}, err
	}

	result := prunePreview{SchemaVersion: 1}

	err = s.write(ctx, func(tx *sql.Tx) error {
		if plan.selection != nil {
			selected, err := s.selectProjectTx(ctx, tx, *plan.selection)
			if err != nil {
				return err
			}

			result.ProjectID = &selected.ID
		}

		now := s.now()
		result.Cutoff = now.Add(-s.policy.retentionLifetime())
		result.ExpiresAt = now.Add(prunePreviewLifetime)

		result.Candidates, err = selectPruneCandidates(ctx, tx, params, result.ProjectID, result.Cutoff.UnixMilli())
		if err != nil {
			return err
		}

		result.Token, err = newID()
		if err != nil {
			return err
		}

		if err := s.reservePrunePreview(ctx, tx, result); err != nil {
			return err
		}

		return insertPruneCandidates(ctx, tx, result)
	})
	if err == nil {
		for _, candidate := range result.Candidates {
			result.Messages += candidate.MessageCount
			result.BodyBytes += candidate.BodyBytes
		}

		s.wakeMaintenance()
	}

	return result, err
}

func insertPruneCandidates(ctx context.Context, tx *sql.Tx, value prunePreview) error {
	for _, candidate := range value.Candidates {
		_, err := tx.ExecContext(ctx, "INSERT INTO prune_candidates VALUES (?,?,?,?)", textDigest(value.Token), candidate.ConversationID, candidate.Revision, candidate.TerminalAt.UnixMilli())
		if err != nil {
			return err
		}
	}

	return nil
}
