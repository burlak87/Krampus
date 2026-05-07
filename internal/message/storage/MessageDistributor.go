package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type MessageDistributor struct {
	producer *kafka.Producer
	logger   logging.Logger
	topic    string
}

type KafkaMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	UserID    string          `json:"user_id"`
	RoomID    string          `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func NewMessageDistributor(cfg config.KafkaConfig, logger logging.Logger) *MessageDistributor {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(cfg.Brokers, ","),
		"client.id":          "krampus-msg-distributor",
		"acks":               "all",
		"enable.idempotence": true,
	})
	if err != nil {
		logger.Fatalf("Kafka producer failed: %v", err)
	}

	return &MessageDistributor{
		producer: p,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
	}
}

// BroadcastToRoom — отправка в конкретную комнату (реализация идентична Broadcast, но с подменой темы если нужно)
func (d *MessageDistributor) BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error {
	msg.RoomID = roomID
	return d.Broadcast(ctx, msg)
}

// SendToUserClient — отправка персонального сообщения (например, в топик уведомлений)
func (d *MessageDistributor) SendToUserClient(ctx context.Context, userID string, msg *domain.BaseMessage) error {
	msg.UserID = userID
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) Broadcast(ctx context.Context, msg *domain.BaseMessage) error {
	event := KafkaMessage{
		ID:        msg.ID,
		Type:      string(msg.Type),
		UserID:    msg.UserID,
		RoomID:    msg.RoomID,
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	deliveryChan := make(chan kafka.Event, 1)

	err = d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &d.topic, Partition: kafka.PartitionAny},
		Value:          data,
		Key:            []byte(msg.RoomID),
	}, deliveryChan)

	if err != nil {
		return fmt.Errorf("kafka produce error: %w", err)
	}

	go func() {
		e := <-deliveryChan
		m := e.(*kafka.Message)

		if m.TopicPartition.Error != nil {
			d.logger.Errorf("Failed to deliver message %s: %v", msg.ID, m.TopicPartition.Error)
		} else {
			d.logger.Infof("Message %s delivered [partition %d, offset %v]", msg.ID, m.TopicPartition.Partition, m.TopicPartition.Offset)
		}
		close(deliveryChan)
	}()

	return nil
}

func (d *MessageDistributor) Close() {
	d.producer.Flush(15 * 1000)
	d.producer.Close()
}
