package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SlotsCache interface {
	GetSlots(ctx context.Context, roomID, date string) ([]Slot, bool, error)
	SetSlots(ctx context.Context, roomID, date string, slots []Slot, ttl time.Duration) error
	InvalidateRoom(ctx context.Context, roomID string) error
}

type NoopSlotsCache struct{}

func (NoopSlotsCache) GetSlots(_ context.Context, _, _ string) ([]Slot, bool, error) {
	return nil, false, nil
}
func (NoopSlotsCache) SetSlots(_ context.Context, _, _ string, _ []Slot, _ time.Duration) error {
	return nil
}
func (NoopSlotsCache) InvalidateRoom(_ context.Context, _ string) error { return nil }

type RedisSlotsCache struct {
	client *redis.Client
	prefix string
}

func NewRedisSlotsCache(addr, password string, db int) (*RedisSlotsCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &RedisSlotsCache{client: client, prefix: "slots"}, nil
}

func (c *RedisSlotsCache) Close() error { return c.client.Close() }

func (c *RedisSlotsCache) GetSlots(ctx context.Context, roomID, date string) ([]Slot, bool, error) {
	raw, err := c.client.Get(ctx, c.key(roomID, date)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var slots []Slot
	if err := json.Unmarshal(raw, &slots); err != nil {
		return nil, false, err
	}
	return slots, true, nil
}

func (c *RedisSlotsCache) SetSlots(ctx context.Context, roomID, date string, slots []Slot, ttl time.Duration) error {
	raw, err := json.Marshal(slots)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.key(roomID, date), raw, ttl).Err()
}

func (c *RedisSlotsCache) InvalidateRoom(ctx context.Context, roomID string) error {
	pattern := fmt.Sprintf("%s:%s:*", c.prefix, roomID)
	iter := c.client.Scan(ctx, 0, pattern, 1000).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (c *RedisSlotsCache) key(roomID, date string) string {
	return fmt.Sprintf("%s:%s:%s", c.prefix, roomID, date)
}
