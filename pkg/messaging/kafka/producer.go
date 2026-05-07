package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"krampus/internal/message/domain"
	"krampus/pkg/config"
	"krampus/pkg/logging"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Producer struct {
	producer *kafka.Producer
	logger   logging.Logger
	topic    string
	wg       sync.WaitGroup
}

func NewProducer(cfg config.KafkaConfig, logger logging.Logger) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(cfg.Brokers, ","),
		"client.id":          "krampus-producer",
		"acks":               "all",
		"retries":            5,
		"enable.idempotence": true,
		"linger.ms":          10,
		"batch.size":         16384,
	})
	if err != nil {
		return nil, err
	}

	producer := &Producer{
		producer: p,
		logger:   logger,
		topic:    cfg.Topics.Incoming,
	}
	producer.wg.Add(1)
	go producer.deliveryReport()
	return producer, nil
}

func (p *Producer) Publish(ctx context.Context, msg *domain.BaseMessage) error {
	// data, err := json.Marshal(msg)
	// if err != nil {
	// 	return err
	// }

	event := struct {
		ID        string          `json:"id"`
		Type      string          `json:"type"`
		UserID    string          `json:"user_id"`
		RoomID    string          `json:"room_id"`
		Timestamp int64           `json:"timestamp"`
		Payload   json.RawMessage `json:"payload"`
	}{
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

	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &p.topic, Partition: kafka.PartitionAny},
		Key:            []byte(msg.RoomID),
		Value:          data,
		Headers:        []kafka.Header{{Key: "source", Value: []byte("krampus-api")}},
	}, nil)

	return err
}

func (p *Producer) deliveryReport() {
	defer p.wg.Done()
	for e := range p.producer.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				p.logger.Errorf("Delivery failed: %v", ev.TopicPartition.Error)
			} else {
				p.logger.Infof("Delivered to %s [%d] at offset %v", *ev.TopicPartition.Topic, ev.TopicPartition, ev.TopicPartition.Offset)
			}
		}
	}
}

func (p *Producer) Close() {
	p.producer.Flush(15 * 1000)
	p.producer.Close()
	p.wg.Wait()
}
