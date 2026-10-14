package iris

import (
	"context"
	"database/sql"
	"time"
)

const (
	maxExportLeases     = 16
	exportLifetime      = 120 * time.Second
	maxExportSpoolBytes = 128 << 20
)

// Runtime leases are accessed only inside serialized store transactions.
type exportLease struct {
	conversationID string
	expiresAt      time.Time
	snapshot       historyConversation
	highest        int64
}

type dumpSnapshot struct {
	SchemaVersion   int                 `json:"schema_version"`
	LeaseID         string              `json:"lease_id"`
	ExpiresAt       time.Time           `json:"expires_at"`
	Conversation    historyConversation `json:"conversation"`
	Participants    []participant       `json:"participants"`
	HighestSequence int64               `json:"highest_sequence"`
}

type dumpLeaseParams struct {
	LeaseID string `json:"lease_id"`
	Cursor  string `json:"cursor,omitempty"`
}

type dumpPage struct {
	SchemaVersion int       `json:"schema_version"`
	Messages      []message `json:"messages"`
	NextCursor    *string   `json:"next_cursor"`
	Truncated     bool      `json:"truncated"`
}

type dumpClosed struct {
	SchemaVersion int  `json:"schema_version"`
	Closed        bool `json:"closed"`
}

func (s *store) openDump(ctx context.Context, params historyParams) (dumpSnapshot, error) {
	if params.ConversationID == "" && (len(params.Pair) != 2 || params.Ordinal <= 0) {
		return dumpSnapshot{}, problem("invalid_request", "dump requires a conversation ID or pair and ordinal")
	}

	if params.Cursor != "" {
		return dumpSnapshot{}, problem("invalid_request", "dump opens a new snapshot without a cursor")
	}

	plan, err := s.planHistory(ctx, params)
	if err != nil {
		return dumpSnapshot{}, err
	}

	result := dumpSnapshot{SchemaVersion: 1}
	err = s.write(ctx, func(tx *sql.Tx) error {
		resolved, err := s.resolveHistory(ctx, tx, plan)
		if err != nil {
			return err
		}

		result, err = s.acquireDump(ctx, tx, resolved)

		return err
	})

	return result, err
}

func (s *store) acquireDump(ctx context.Context, tx *sql.Tx, plan historyPlan) (dumpSnapshot, error) {
	now := s.now()
	s.reapExports(now)

	if len(s.exports) >= maxExportLeases {
		return dumpSnapshot{}, problem("limit_exceeded", "export lease limit reached")
	}

	ids, err := historyIDs(ctx, tx, plan)
	if err != nil {
		return dumpSnapshot{}, err
	}

	if len(ids) != 1 {
		return dumpSnapshot{}, problem("not_found", "conversation is not visible in this scope")
	}

	value, err := readHistoryConversation(ctx, tx, ids[0])
	if err != nil {
		return dumpSnapshot{}, err
	}

	var highest int64
	if err := tx.QueryRowContext(ctx, "SELECT max(sequence) FROM messages WHERE conversation_id=?", value.ID).Scan(&highest); err != nil {
		return dumpSnapshot{}, err
	}

	id, err := newID()
	if err != nil {
		return dumpSnapshot{}, err
	}

	expires := now.Add(exportLifetime)
	s.exports[id] = exportLease{conversationID: value.ID, expiresAt: expires, snapshot: value, highest: highest}

	return dumpSnapshot{SchemaVersion: 1, LeaseID: id, ExpiresAt: expires, Conversation: value, Participants: value.Participants, HighestSequence: highest}, nil
}

func (s *store) reapExports(now time.Time) {
	for id, lease := range s.exports {
		if !now.Before(lease.expiresAt) {
			delete(s.exports, id)
		}
	}
}

func (s *store) pageDump(ctx context.Context, params dumpLeaseParams) (dumpPage, error) {
	if !validID(params.LeaseID) || len(params.Cursor) > 2048 {
		return dumpPage{}, problem("invalid_request", "invalid export lease or cursor")
	}

	result := dumpPage{SchemaVersion: 1}
	err := s.write(ctx, func(tx *sql.Tx) error {
		lease, ok := s.exports[params.LeaseID]
		if !ok || !s.now().Before(lease.expiresAt) {
			return problem("export_expired", "export lease expired or service restarted")
		}

		after, err := historySequence(params.Cursor, params.LeaseID, lease.highest)
		if err != nil {
			return err
		}

		result, err = readDumpPage(ctx, tx, params.LeaseID, lease, after)

		return err
	})

	return result, err
}

func readDumpPage(ctx context.Context, tx *sql.Tx, id string, lease exportLease, after int64) (dumpPage, error) {
	messages, last, err := readHistoryMessages(ctx, tx, lease.conversationID, after, lease.highest, 100)
	if err != nil {
		return dumpPage{}, err
	}

	if last < lease.highest && len(messages) == 0 {
		return dumpPage{}, problem("internal_error", "export snapshot is incomplete")
	}

	result := dumpPage{SchemaVersion: 1, Messages: messages, Truncated: last < lease.highest}
	if result.Truncated {
		result.NextCursor, err = encodeHistorySequence(id, last)
	}

	return result, err
}

func (s *store) closeDump(ctx context.Context, params dumpLeaseParams) (dumpClosed, error) {
	if !validID(params.LeaseID) || params.Cursor != "" {
		return dumpClosed{}, problem("invalid_request", "close requires only an export lease")
	}

	err := s.write(ctx, func(_ *sql.Tx) error { delete(s.exports, params.LeaseID); return nil })

	return dumpClosed{SchemaVersion: 1, Closed: true}, err
}
