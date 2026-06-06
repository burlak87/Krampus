package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Consumer struct {
	consumer *kafka.Consumer
	logger   logging.Logger
	topic    string
	handlers []func(*domain.BaseMessage)
	mu       sync.RWMutex
}

func NewConsumer(cfg config.KafkaConfig, logger logging.Logger) (*Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":     strings.Join(cfg.Brokers, ","),
		"group.id":              "krampus-consumer-group",
		"auto.offset.reset":     "latest",
		"enable.auto.commit":    true,
		"session.timeout.ms":    6000,
		"heartbeat.interval.ms": 3000,
	})
	if err != nil {
		return nil, err
	}

	if err := c.SubscribeTopics([]string{cfg.Topics.Incoming}, nil); err != nil {
		return nil, err
	}

	return &Consumer{
		consumer: c,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
	}, nil
}

func (c *Consumer) AddHandler(handler func(*domain.BaseMessage)) {
	c.mu.Lock()
	c.handlers = append(c.handlers, handler)
	c.mu.Unlock()
}

func (c *Consumer) Consume(ctx context.Context) {
	defer c.consumer.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				continue
			}

			var event struct {
				ID        string          `json:"id"`
				Type      string          `json:"type"`
				UserID    string          `json:"user_id"`
				RoomID    string          `json:"room_id"`
				Timestamp int64           `json:"timestamp"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				c.logger.Errorf("Failed to unmarshal Kafka msg: %v", err)
				continue
			}

			msgObj := &domain.BaseMessage{
				ID:        types.MessageID(event.ID),
				Type:      domain.MessageType(event.Type),
				UserID:    types.UserID(event.UserID),
				RoomID:    types.RoomID(event.RoomID),
				Timestamp: event.Timestamp,
				Payload:   event.Payload,
			}

			c.mu.RLock()
			for _, handler := range c.handlers {
				go handler(msgObj)
			}
			c.mu.RUnlock()
		}
	}
}
