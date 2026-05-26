package cache

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Address  string
	Password string
	Db       int
}

type RedisClient struct {
	client *redis.Client
}

func InitRedis(cfg Config) (*RedisClient, error) {
	log := slog.Default().With("component", "redis")
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.Db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Error("redis ping failed", "address", cfg.Address, "db", cfg.Db, "pong", pong, "error", err)
		return nil, err
	}
	log.Info("redis ping succeeded", "address", cfg.Address, "db", cfg.Db, "pong", pong)
	return &RedisClient{client: rdb}, nil
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return val, err
}

func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
