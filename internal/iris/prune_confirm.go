package iris

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type storedPrunePreview struct {
	selectionDigest string
	policyDigest    string
	cutoff          int64
	expires         int64
}

func (s *store) confirmPrune(ctx context.Context, params pruneConfirmation) (pruneResult, error) {
	if !validID(params.Token) {
		return pruneResult{}, problem("invalid_request", "prune confirmation requires its exact preview token")
	}

	result := pruneResult{SchemaVersion: 1, ConversationIDs: []string{}}
	err := s.write(ctx, func(tx *sql.Tx) error {
		preview, err := readPrunePreview(ctx, tx, params.Token)
		if err != nil {
			return err
		}

		if err := s.validatePrunePolicy(preview); err != nil {
			return err
		}

		values, err := loadPruneConfirmation(ctx, tx, params.Token, preview.selectionDigest)
		if err != nil {
			return err
		}

		if err := s.checkPruneEligibility(values, preview.cutoff); err != nil {
			return err
		}

		result, err = deletePruneCandidates(ctx, tx, params.Token, values)

		return err
	})

	return result, err
}

func readPrunePreview(ctx context.Context, tx *sql.Tx, token string) (storedPrunePreview, error) {
	var value storedPrunePreview

	err := tx.QueryRowContext(ctx, "SELECT selection_digest,policy_digest,eligibility_cutoff,expires_at FROM prune_previews WHERE token_digest=?", textDigest(token)).Scan(&value.selectionDigest, &value.policyDigest, &value.cutoff, &value.expires)
	if errors.Is(err, sql.ErrNoRows) {
		return value, stalePrunePreview()
	}

	return value, err
}

func stalePrunePreview() error {
	return problem("preview_stale", "prune preview is stale; request another preview")
}

func (s *store) validatePrunePolicy(value storedPrunePreview) error {
	policy, err := jsonDigest(s.policy)
	if err != nil {
		return err
	}

	if value.policyDigest != policy || s.now().UnixMilli() >= value.expires {
		return stalePrunePreview()
	}

	return nil
}

func (s *store) checkPruneEligibility(values []pruneCandidate, cutoff int64) error {
	now := s.now()
	currentCutoff := now.Add(-s.policy.retentionLifetime()).UnixMilli()

	for _, value := range values {
		if value.State != "closed" && value.State != "expired" {
			return stalePrunePreview()
		}

		if value.TerminalAt.UnixMilli() > cutoff || value.TerminalAt.UnixMilli() > currentCutoff || s.hasExportLease(value.ConversationID, now) {
			return stalePrunePreview()
		}
	}

	return nil
}

func (s *store) hasExportLease(id string, now time.Time) bool {
	for _, lease := range s.exports {
		if lease.conversationID == id && now.Before(lease.expiresAt) {
			return true
		}
	}

	return false
}

func deletePruneCandidates(ctx context.Context, tx *sql.Tx, token string, values []pruneCandidate) (pruneResult, error) {
	result := pruneResult{SchemaVersion: 1, ConversationIDs: []string{}}

	for _, value := range values {
		deleted, err := tx.ExecContext(ctx, "DELETE FROM conversations WHERE id=?", value.ConversationID)
		if err != nil {
			return result, err
		}

		count, err := deleted.RowsAffected()
		if err != nil {
			return result, err
		}

		if count != 1 {
			return result, stalePrunePreview()
		}

		result.ConversationIDs = append(result.ConversationIDs, value.ConversationID)
		result.Messages += value.MessageCount
		result.BodyBytes += value.BodyBytes
	}

	_, err := tx.ExecContext(ctx, "DELETE FROM prune_previews WHERE token_digest=?", textDigest(token))

	return result, err
}
