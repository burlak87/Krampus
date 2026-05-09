package ctxmeta

import "context"

type contextKey string

const metadataKey contextKey = "request_metadata"

type Metadata struct {
	TraceID       string
	RequestID     string
	CorrelationID string
	UserID        string
}

func WithMetadata(
	ctx context.Context,
	meta Metadata,
) context.Context {

	return context.WithValue(
		ctx,
		metadataKey,
		meta,
	)
}

func Extract(ctx context.Context) Metadata {

	meta, ok := ctx.Value(metadataKey).(Metadata)

	if !ok {
		return Metadata{}
	}

	return meta
}
