package iris

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	defaultInvitationLifetime = 2 * time.Hour
	defaultInactivityLifetime = 24 * time.Hour
	defaultRetentionLifetime  = 30 * 24 * time.Hour
	maxLifecycleSeconds       = 10 * 365 * 24 * 60 * 60
)

type lifecyclePolicy struct {
	InvitationSeconds int64 `json:"invitation_seconds"`
	InactivitySeconds int64 `json:"inactivity_seconds"`
	RetentionSeconds  int64 `json:"retention_seconds"`
}

func defaultLifecyclePolicy() lifecyclePolicy {
	return lifecyclePolicy{InvitationSeconds: int64(defaultInvitationLifetime / time.Second), InactivitySeconds: int64(defaultInactivityLifetime / time.Second), RetentionSeconds: int64(defaultRetentionLifetime / time.Second)}
}

func defaultSettings() settings {
	return settings{SchemaVersion: 1, MaxConnections: 128, RequestTimeoutSeconds: 10, Lifecycle: defaultLifecyclePolicy()}
}

func (policy lifecyclePolicy) validate() error {
	for _, value := range []int64{policy.InvitationSeconds, policy.InactivitySeconds, policy.RetentionSeconds} {
		if value < 1 || value > maxLifecycleSeconds {
			return fmt.Errorf("lifecycle durations must be 1 through %d seconds", maxLifecycleSeconds)
		}
	}

	return nil
}

func (policy lifecyclePolicy) invitationLifetime() time.Duration {
	return time.Duration(policy.InvitationSeconds) * time.Second
}

func (policy lifecyclePolicy) inactivityLifetime() time.Duration {
	return time.Duration(policy.InactivitySeconds) * time.Second
}

func (policy *lifecyclePolicy) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeJSON(data, &fields); err != nil {
		return err
	}

	value := *policy

	for name, raw := range fields {
		var seconds *int64
		if err := json.Unmarshal(raw, &seconds); err != nil {
			return err
		}

		if seconds == nil {
			return fmt.Errorf("lifecycle durations cannot be null")
		}

		switch name {
		case "invitation_seconds":
			value.InvitationSeconds = *seconds
		case "inactivity_seconds":
			value.InactivitySeconds = *seconds
		case "retention_seconds":
			value.RetentionSeconds = *seconds
		default:
			return fmt.Errorf("unknown lifecycle policy field")
		}
	}

	*policy = value

	return nil
}
