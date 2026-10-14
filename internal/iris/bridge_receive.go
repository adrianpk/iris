package iris

import (
	"context"
	"database/sql"
	"time"
)

type bridgeReceiveParams struct {
	bridgeOwner
	WaitSeconds int `json:"wait_seconds"`
}

type bridgeDelivery struct {
	Page     receivePage `json:"page"`
	Active   bool        `json:"active"`
	Stopping bool        `json:"stopping"`
}

func (s *store) receiveBridge(ctx context.Context, params bridgeReceiveParams) (bridgeDelivery, error) {
	if params.WaitSeconds < 0 || params.WaitSeconds > 30 {
		return bridgeDelivery{}, problem("invalid_request", "bridge wait must be 0 through 30 seconds")
	}

	value, err := s.receiveBridgeOnce(ctx, params.bridgeOwner)
	if err != nil || params.WaitSeconds == 0 || bridgeDeliveryReady(value) {
		return value, err
	}

	return s.waitBridgeDelivery(ctx, params, value)
}

func (s *store) waitBridgeDelivery(ctx context.Context, params bridgeReceiveParams, value bridgeDelivery) (bridgeDelivery, error) {
	subscription, err := s.events.subscribe(value.Page.ConversationID, value.Page.ProjectID)
	if err != nil {
		return bridgeDelivery{}, err
	}
	defer s.events.remove(subscription)

	timer := time.NewTimer(time.Duration(params.WaitSeconds) * time.Second)
	defer timer.Stop()

	for {
		value, err = s.receiveBridgeOnce(ctx, params.bridgeOwner)
		if err != nil || bridgeDeliveryReady(value) {
			return value, err
		}

		select {
		case <-ctx.Done():
			return bridgeDelivery{}, ctx.Err()
		case <-timer.C:
			return s.receiveBridgeOnce(ctx, params.bridgeOwner)
		case <-subscription.ready:
		}
	}
}

func bridgeDeliveryReady(value bridgeDelivery) bool {
	return len(value.Page.Messages) > 0 || !value.Active || value.Stopping
}

func (s *store) receiveBridgeOnce(ctx context.Context, owner bridgeOwner) (bridgeDelivery, error) {
	var result bridgeDelivery

	err := s.write(ctx, func(tx *sql.Tx) error {
		value, err := s.ownedBridge(ctx, tx, owner)
		if err != nil {
			return err
		}

		if value.State != "waiting" && value.State != "acked" {
			return problem("worker_uncertain", "cannot receive during an invocation")
		}

		result.Active, err = s.bridgeActive(ctx, tx, value)
		if err != nil {
			return err
		}

		result.Stopping = value.StopRequestedAt != nil
		result.Page, err = readBridgePage(ctx, tx, value)

		return err
	})

	return result, err
}

func readBridgePage(ctx context.Context, tx *sql.Tx, value bridgeRecord) (receivePage, error) {
	position, err := loadConsumer(ctx, tx, consumerPosition{ConversationID: value.ConversationID, ProjectID: value.ProjectID, ConsumerID: value.ConsumerID})
	if err != nil {
		return receivePage{}, err
	}

	page, err := readDeliveryPage(ctx, tx, position, position.AckedSequence, 1)
	if err != nil {
		return receivePage{}, err
	}

	_, err = tx.ExecContext(ctx, "UPDATE consumer_cursors SET delivered_sequence=? WHERE conversation_id=? AND project_id=? AND consumer_id=?", page.DeliveredSequence, value.ConversationID, value.ProjectID, value.ConsumerID)

	return page, err
}
