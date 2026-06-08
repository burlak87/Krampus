package storage

import (
	"context"
	"encoding/json"
	"strings"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type MessageDistributor struct {
	producer *kafka.Producer
	logger   logging.Logger
	topic    string
	ctx      context.Context
	cancel   context.CancelFunc
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

	ctx, cancel := context.WithCancel(context.Background())
	d := &MessageDistributor{
		producer: p,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
		ctx:      ctx,
		cancel:   cancel,
	}

	go d.handleDeliveryReports()

	return d
}

func (d *MessageDistributor) BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error {
	msg.RoomID = types.RoomIDFromString(roomID)
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) SendToUserClient(ctx context.Context, userID string, msg *domain.BaseMessage) error {
	msg.UserID = types.UserIDFromString(userID)
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) Broadcast(ctx context.Context, msg *domain.BaseMessage) error {
	event := KafkaMessage{
		ID:        msg.ID.String(),
		Type:      string(msg.Type),
		UserID:    msg.UserID.String(),
		RoomID:    msg.RoomID.String(),
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &d.topic,
			Partition: kafka.PartitionAny,
		},
		Value: data,
		Key:   []byte(msg.RoomID),
	}, nil)

	return nil
}

func (d *MessageDistributor) Publish(ctx context.Context, msg *domain.BaseMessage) error {
	return d.Broadcast(ctx, msg)
}

func (d *MessageDistributor) handleDeliveryReports() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case event := <-d.producer.Events():
			switch e := event.(type) {
			case *kafka.Message:
				if e.TopicPartition.Error != nil {
					d.logger.Errorf("kafka delivery failed topic=%s err=%v", *e.TopicPartition.Topic, e.TopicPartition.Error)
				} else {
					d.logger.Infof("kafka delivered topic=%s partition=%d offset=%v", *e.TopicPartition.Topic, e.TopicPartition.Partition, e.TopicPartition.Offset)
				}
			}
		}
	}
}

func (d *MessageDistributor) Close() {
	d.cancel()
	d.producer.Flush(15 * 1000)
	d.producer.Close()
}
