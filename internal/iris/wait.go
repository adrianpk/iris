package iris

import (
	"context"
	"sync"
	"time"
)

type waitHub struct {
	mu      sync.Mutex
	waiters map[*waitSubscription]struct{}
}
type waitSubscription struct {
	conversationID, projectID string
	ready                     chan struct{}
}

func (hub *waitHub) subscribe(conversationID, projectID string) (*waitSubscription, error) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	count := 0

	for waiter := range hub.waiters {
		if waiter.projectID == projectID {
			count++
		}
	}

	if len(hub.waiters) >= 64 || count >= 16 {
		return nil, &serviceError{Code: "server_busy", Message: "receive waiter limit reached", Retryable: true}
	}

	if hub.waiters == nil {
		hub.waiters = map[*waitSubscription]struct{}{}
	}

	waiter := &waitSubscription{conversationID: conversationID, projectID: projectID, ready: make(chan struct{}, 1)}
	hub.waiters[waiter] = struct{}{}

	return waiter, nil
}

func (hub *waitHub) remove(waiter *waitSubscription) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	delete(hub.waiters, waiter)
}

func (hub *waitHub) notify(conversationID string) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for waiter := range hub.waiters {
		if waiter.conversationID == conversationID {
			select {
			case waiter.ready <- struct{}{}:
			default:
			}
		}
	}
}

func (s *store) waitForDelivery(ctx context.Context, params receiveParams, plan selectionPlan, page receivePage) (receivePage, error) {
	if params.WaitSeconds == 0 || len(page.Messages) > 0 {
		return page, nil
	}

	subscription, err := s.events.subscribe(params.ConversationID, page.ProjectID)
	if err != nil {
		return receivePage{}, err
	}
	defer s.events.remove(subscription)

	timer := time.NewTimer(time.Duration(params.WaitSeconds) * time.Second)
	defer timer.Stop()

	for {
		// Subscribe before rechecking SQLite: a commit on either side of subscription cannot be missed.
		page, err = s.receiveOnce(ctx, params, plan)
		if err != nil || len(page.Messages) > 0 {
			return page, err
		}

		select {
		case <-ctx.Done():
			return receivePage{}, ctx.Err()
		case <-timer.C:
			return s.receiveOnce(ctx, params, plan)
		case <-subscription.ready:
		}
	}
}
