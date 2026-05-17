package domain

import "time"

type PipelineJob struct {
	Message     *BaseMessage
	Attempt     int
	CreatedAt   time.Time
	NextRetryAt time.Time
}
