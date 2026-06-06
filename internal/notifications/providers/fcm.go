package providers

import (
	"context"
	"log"
)

type FCMProvider struct{}

func NewFCMProvider() *FCMProvider {
	return &FCMProvider{}
}

func (p *FCMProvider) SendPush(ctx context.Context, token, title, body string) error {
	log.Printf("fcm push token=%s title=%s body=%s", token, title, body)
	return nil
}
