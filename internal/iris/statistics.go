package iris

import (
	"context"
	"database/sql"
)

type conversationStates struct {
	Pending int64 `json:"pending"`
	Active  int64 `json:"active"`
	Closed  int64 `json:"closed"`
	Expired int64 `json:"expired"`
}

type storageStatistics struct {
	Scope          string `json:"scope"`
	PageSize       int64  `json:"page_size"`
	PageCount      int64  `json:"page_count"`
	FreePages      int64  `json:"free_pages"`
	AllocatedBytes int64  `json:"allocated_bytes"`
	ReusableBytes  int64  `json:"reusable_bytes"`
}

func (s *store) readStatistics(ctx context.Context, tx *sql.Tx, projectID *string) (statistics, error) {
	result := statistics{SchemaVersion: 1, Ready: true, ProjectID: projectID, Lifecycle: s.policy}
	if err := tx.QueryRowContext(ctx, "SELECT max(version) FROM schema_migrations").Scan(&result.StorageSchemaVersion); err != nil {
		return result, err
	}

	result.Projects = 1
	if projectID == nil {
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM projects WHERE canonical_path IS NOT NULL").Scan(&result.Projects); err != nil {
			return result, err
		}
	}

	if err := s.countHistory(ctx, tx, &result); err != nil {
		return result, err
	}

	result.Storage.Scope = "service"
	if err := readStorageStatistics(ctx, tx, &result.Storage); err != nil {
		return result, err
	}

	return result, nil
}

func (s *store) countHistory(ctx context.Context, tx *sql.Tx, result *statistics) error {
	query := `SELECT count(*),coalesce(sum(message_count),0),coalesce(sum(body_bytes),0),
 coalesce(sum(state='pending'),0),coalesce(sum(state='active'),0),
 coalesce(sum(state='closed'),0),coalesce(sum(state='expired'),0),
 coalesce(sum(terminal_at IS NOT NULL AND terminal_at<=?),0)
 FROM conversations c`
	args := []any{s.now().Add(-s.policy.retentionLifetime()).UnixMilli()}

	if result.ProjectID != nil {
		query += " WHERE EXISTS (SELECT 1 FROM participants p WHERE p.conversation_id=c.id AND p.project_id=?)"

		args = append(args, *result.ProjectID)
	}

	err := tx.QueryRowContext(ctx, query, args...).Scan(&result.Conversations, &result.Messages, &result.BodyBytes, &result.States.Pending, &result.States.Active, &result.States.Closed, &result.States.Expired, &result.PruneEligible)
	result.RetainedConversations = result.States.Closed + result.States.Expired

	return err
}

func readStorageStatistics(ctx context.Context, tx *sql.Tx, result *storageStatistics) error {
	for _, entry := range []struct {
		query  string
		target *int64
	}{
		{"PRAGMA page_size", &result.PageSize},
		{"PRAGMA page_count", &result.PageCount},
		{"PRAGMA freelist_count", &result.FreePages},
	} {
		if err := tx.QueryRowContext(ctx, entry.query).Scan(entry.target); err != nil {
			return err
		}
	}

	result.AllocatedBytes = result.PageSize * result.PageCount
	result.ReusableBytes = result.PageSize * result.FreePages

	return nil
}
