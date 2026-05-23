package search

type Filters struct {
	RoomID *string

	UserID *string

	TopicID *string

	MessageType *string

	HasAttachments *bool

	HasReactions *bool

	HasPoll *bool

	MentionsUserID *string

	FromTimestamp *int64

	ToTimestamp *int64
}
