package iris

import "time"

const bridgeLease = 15 * time.Second

type bridgeOptions struct {
	Model        string `json:"model"`
	Sandbox      string `json:"sandbox"`
	Approvals    string `json:"approvals"`
	TurnSeconds  int    `json:"turn_seconds"`
	MaxTurns     int    `json:"max_turns"`
	TotalSeconds int    `json:"total_seconds"`
	MaxTokens    int64  `json:"max_tokens"`
}

func defaultBridgeOptions() bridgeOptions {
	return bridgeOptions{Sandbox: "read-only", Approvals: "deny", TurnSeconds: 120, MaxTurns: 20, TotalSeconds: 600, MaxTokens: 100000}
}

func (options bridgeOptions) validate() error {
	if options.Sandbox != "read-only" && options.Sandbox != "workspace-write" {
		return problem("invalid_request", "bridge sandbox must be read-only or workspace-write")
	}

	if options.Approvals != "deny" && options.Approvals != "interactive" {
		return problem("invalid_request", "bridge approvals must be deny or interactive")
	}

	if options.Model != "" && !validText(options.Model, 128) {
		return problem("invalid_request", "invalid model identifier")
	}

	return options.validateLimits()
}

func (options bridgeOptions) validateLimits() error {
	if options.TurnSeconds < 1 || options.TurnSeconds > 3600 || options.TotalSeconds < 1 || options.TotalSeconds > 3600 {
		return problem("invalid_request", "bridge time limits must be 1 through 3600 seconds")
	}

	if options.MaxTurns < 1 || options.MaxTurns > 100 || options.MaxTokens < 1 {
		return problem("invalid_request", "invalid bridge turn or token limit")
	}

	return nil
}

type bridgeBinding struct {
	SchemaVersion    int           `json:"schema_version"`
	ID               string        `json:"binding_id"`
	ConversationID   string        `json:"conversation_id"`
	ProjectID        string        `json:"project_id"`
	ProjectPath      string        `json:"project_path"`
	ConsumerID       string        `json:"consumer_id"`
	ThreadID         *string       `json:"thread_id"`
	TurnID           *string       `json:"turn_id"`
	InputMessageID   *string       `json:"input_message_id"`
	ResultMessageID  *string       `json:"result_message_id"`
	State            string        `json:"state"`
	Options          bridgeOptions `json:"options"`
	UsageTokens      *int64        `json:"usage_tokens"`
	RecoveryHint     *string       `json:"recovery_hint"`
	UsageLimitation  *string       `json:"usage_limitation"`
	TurnsUsed        int           `json:"turns_used"`
	StartedAt        time.Time     `json:"started_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	LeaseUntil       *time.Time    `json:"lease_until"`
	StopRequestedAt  *time.Time    `json:"stop_requested_at"`
	ErrorCode        *string       `json:"error_code"`
	PendingRequestID *string       `json:"pending_request_id"`
	PendingMethod    *string       `json:"pending_method"`
}

type bridgeRecord struct {
	bridgeBinding
	ownerToken    *string
	pendingResult *recordedBridgeResult
}
type bridgeClaimParams struct {
	conversationParams
	Options bridgeOptions `json:"options"`
	Recover bool          `json:"recover,omitempty"`
}
type bridgeClaim struct {
	SchemaVersion int                   `json:"schema_version"`
	Binding       bridgeBinding         `json:"binding"`
	OwnerToken    string                `json:"owner_token"`
	Conversation  conversation          `json:"conversation"`
	PendingResult *recordedBridgeResult `json:"pending_result"`
}
type bridgeOwner struct {
	BindingID  string `json:"binding_id"`
	OwnerToken string `json:"owner_token"`
}
type bridgeOutcome struct {
	record  bridgeRecord
	problem error
}
