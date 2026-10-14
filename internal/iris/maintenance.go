package iris

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"time"
)

const (
	maintenanceBatchSize = 128
	maintenanceMaxWait   = 30 * time.Second
)

func (s *store) notify(id string) {
	s.events.notify(id)

	select {
	case s.maintenanceWake <- struct{}{}:
	default:
	}
}

func (s *store) reconcile(ctx context.Context) error {
	for {
		count, err := s.expireBatch(ctx)
		if err != nil {
			return err
		}

		if count < maintenanceBatchSize {
			return nil
		}
	}
}

func (s *store) maintain(ctx context.Context) error {
	for {
		if _, err := s.expireBatch(ctx); err != nil {
			return maintenanceError(err)
		}

		delay, err := s.maintenanceDelay(ctx)
		if err != nil {
			return maintenanceError(err)
		}

		if !s.waitMaintenance(ctx, delay) {
			return nil
		}
	}
}

func (s *store) waitMaintenance(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	case <-s.maintenanceWake:
		return true
	}
}

func maintenanceError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}

	return storageFailure(err)
}

func serveWithMaintenance(ctx context.Context, cfg config, s *store, listener *net.UnixListener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)

	go func() { err := s.maintain(ctx); cancel(); done <- err }()

	service := server{config: cfg, handle: s.handle}
	err := service.serve(ctx, listener)

	cancel()

	return errors.Join(err, <-done)
}

func (s *store) maintenanceDelay(ctx context.Context) (time.Duration, error) {
	var next sql.NullInt64

	err := s.db.QueryRowContext(ctx, `SELECT min(deadline) FROM (
 SELECT min(i.expires_at) AS deadline FROM invitations i JOIN conversations c ON c.id=i.conversation_id WHERE c.state='pending'
 UNION ALL SELECT min(last_activity_at)+? FROM conversations WHERE state='active')`, s.policy.inactivityLifetime().Milliseconds()).Scan(&next)
	if err != nil {
		return 0, err
	}

	if !next.Valid {
		return maintenanceMaxWait, nil
	}

	delay := milliseconds(next.Int64).Sub(s.now())
	if delay > maintenanceMaxWait {
		return maintenanceMaxWait, nil
	}

	if delay < time.Millisecond {
		return time.Millisecond, nil
	}

	return delay, nil
}
