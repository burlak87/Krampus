package polls

type PollClosedEvent struct {
	PollID string

	ClosedAt int64
}
