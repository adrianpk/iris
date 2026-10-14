package iris

import (
	"testing"
	"time"
)

// Project and administrative counts reflect retained state, policy eligibility and logical SQLite allocation.
func TestHistoryStatistics(t *testing.T) {
	f, clock := retainedFixture(t)
	other := registeredProject(t, f.store, "gamma")
	mustCall(t, f.store, callFixture(t, f.store, f.beta, other.ID, "other-project"))

	cases := []struct {
		name                                                 string
		params                                               statsParams
		conversations, messages, retained, eligible, pending int64
	}{
		{"selected", statsParams{projectSelection: projectSelection{Project: f.alpha.ID}}, 1, 3, 1, 1, 0},
		{"other", statsParams{projectSelection: projectSelection{Project: other.ID}}, 1, 1, 0, 0, 1},
		{"administrative", statsParams{AllProjects: true}, 2, 4, 1, 1, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value, err := f.store.selectedStatistics(t.Context(), test.params)
			if err != nil {
				t.Fatal(err)
			}

			assertStatisticsCounts(t, value, test.conversations, test.messages, test.retained, test.eligible, test.pending)
			assertStatisticsStorage(t, value)
		})
	}

	clock.advance(-time.Millisecond)

	value, err := f.store.selectedStatistics(t.Context(), statsParams{projectSelection: projectSelection{Project: f.alpha.ID}})
	if err != nil || value.PruneEligible != 0 {
		t.Fatalf("retention boundary=%+v %v", value, err)
	}

	assertExpiredStatistics(t, f, clock)
}

func assertStatisticsCounts(t *testing.T, value statistics, conversations, messages, retained, eligible, pending int64) {
	t.Helper()

	if value.Conversations != conversations || value.Messages != messages || value.RetainedConversations != retained || value.PruneEligible != eligible || value.States.Pending != pending {
		t.Fatalf("statistics=%+v", value)
	}

	if value.BodyBytes <= 0 || value.Lifecycle.RetentionSeconds != 1 {
		t.Fatalf("retention/bytes=%+v", value)
	}
}

func assertStatisticsStorage(t *testing.T, value statistics) {
	t.Helper()

	storage := value.Storage
	if storage.Scope != "service" || storage.PageSize <= 0 || storage.PageCount <= 0 || storage.AllocatedBytes != storage.PageSize*storage.PageCount || storage.ReusableBytes != storage.PageSize*storage.FreePages {
		t.Fatalf("storage=%+v", storage)
	}
}

func assertExpiredStatistics(t *testing.T, f deliveryFixture, clock *lifecycleClock) {
	t.Helper()
	clock.advance(2*time.Hour + time.Second)

	if _, err := f.store.expireBatch(t.Context()); err != nil {
		t.Fatal(err)
	}

	value, err := f.store.selectedStatistics(t.Context(), statsParams{AllProjects: true})
	if err != nil || value.States.Expired != 1 || value.RetainedConversations != 2 || value.Messages != 5 {
		t.Fatalf("expired statistics=%+v %v", value, err)
	}
}
