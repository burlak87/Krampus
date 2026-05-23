package providers

import "context"

type Provider interface {
	SendPush(
		ctx context.Context,
		token string,
		title string,
		body string,
	) error
}
