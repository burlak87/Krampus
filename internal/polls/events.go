package polls

type VoteEvent struct {
	PollID string

	OptionID string

	UserID string

	Timestamp int64
}
