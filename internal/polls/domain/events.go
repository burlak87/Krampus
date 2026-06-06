package domain

type VoteEvent struct {
	PollID string

	OptionID string

	UserID string

	Timestamp int64
}

type PollClosedEvent struct {
	PollID string

	ClosedAt int64
}
