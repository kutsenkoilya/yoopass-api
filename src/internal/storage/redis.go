package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
	ctx    context.Context
}

func New(addr string) (*Store, error) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Check connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis connection failed: %v", err)
	}

	return &Store{
		client: client,
		ctx:    ctx,
	}, nil
}

func (s *Store) Set(key string, value []byte, ttl time.Duration) error {
	return s.client.Set(s.ctx, key, value, ttl).Err()
}

func (s *Store) Fetch(key string) ([]byte, error) {
	object, err := s.client.Get(s.ctx, key).Result()

	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return []byte(object), nil
}

func (s *Store) Delete(key string) error {
	return s.client.Del(s.ctx, key).Err()
}
