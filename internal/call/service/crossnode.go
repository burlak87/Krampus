package service

import (
	"context"
	"encoding/json"

	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/redis/go-redis/v9"
)

// crossNodeEnvelope wraps a relayed frame with its origin node so a publishing
// instance can ignore the copy it receives back from its own broadcast.
type crossNodeEnvelope struct {
	Origin string          `json:"origin"`
	Room   types.RoomID    `json:"room"`
	Frame  json.RawMessage `json:"frame"`
}

const crossNodeChannel = "call:signal"

// RedisCrossNode implements CrossNodePublisher over Redis Pub/Sub and runs a
// subscriber that delivers frames from other instances into the local Hub.
type RedisCrossNode struct {
	rdb    *redis.Client
	nodeID string
	hub    *Hub
	log    *logging.Logger
}

func NewRedisCrossNode(rdb *redis.Client, nodeID string) *RedisCrossNode {
	return &RedisCrossNode{rdb: rdb, nodeID: nodeID, log: logging.GetLogger()}
}

func (c *RedisCrossNode) Publish(ctx context.Context, roomID types.RoomID, frame []byte) error {
	env := crossNodeEnvelope{
		Origin: c.nodeID,
		Room:   roomID,
		Frame:  json.RawMessage(frame),
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return c.rdb.Publish(ctx, crossNodeChannel, payload).Err()
}

// Run subscribes to the cross-node channel and delivers foreign frames into the
// hub until ctx is cancelled. Call it in a goroutine after the hub is built.
func (c *RedisCrossNode) Run(ctx context.Context, hub *Hub) {
	c.hub = hub
	sub := c.rdb.Subscribe(ctx, crossNodeChannel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var env crossNodeEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				c.log.Errorf("call: bad cross-node payload: %v", err)
				continue
			}
			// Skip frames we published ourselves — they were already delivered
			// locally by Hub.Broadcast.
			if env.Origin == c.nodeID {
				continue
			}
			hub.DeliverFromCrossNode(env.Room, []byte(env.Frame))
		}
	}
}
