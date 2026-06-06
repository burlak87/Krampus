package domain

type ModerationPayload struct {
	TargetUserID string `json:"target_user_id"`
	ActionType   string `json:"action_type"`
}
