package redis

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

type CoreMemoryCache interface {
	GetCoreMemory(userId string, ctx context.Context) ([]types.Memory, error)
	SetCoreMemory(userId string, CoreMemories []types.Memory, ctx context.Context) error
	DeleteCoreMemory(CoreMemoryId string, ctx context.Context) error
}

type RedisCoreMemoryCache struct {
	RedisClient *redis.Client
}

var Tracer = otel.Tracer("Go_Memory")

func NewRedisCoreMemoryCache() *RedisCoreMemoryCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password
		DB:       0,  // use default DB
		Protocol: 2,
	})
	return &RedisCoreMemoryCache{
		RedisClient: rdb,
	}
}

// Get Core Memories from the Redis Cache. It return nil,nil if the user has currently no core memories.
func (r *RedisCoreMemoryCache) GetCoreMemory(userId string, ctx context.Context) ([]types.Memory, error) {
	ctx, span := Tracer.Start(ctx, "Getting Core Memories from Redis")
	defer span.End()
	res, err := r.RedisClient.Get(ctx, userId).Bytes()
	if err != nil {
		if err == redis.Nil {
			slog.Info("Cache miss! The user has no core memories!", "userId", userId)
			return nil, nil
		}
		slog.Error("Got this error while trying to get core memories of the user! redis error", "userId", userId, "error", err)
		return nil, err
	}
	v := &[]types.Memory{}
	err = json.Unmarshal(res, v)
	if err != nil {
		slog.Error("Got this error while trying to get core memories of the user (unmarshalling the json)", "userId", userId, "error", err)
		return nil, err
	}
	slog.Info("Got this as the core memories of the user", "userId", userId, "memories", *v)

	return *v, nil
}

func (r *RedisCoreMemoryCache) SetCoreMemory(userId string, CoreMemories []types.Memory, ctx context.Context) error {
	ctx, span := Tracer.Start(ctx, "Setting Core Memories in Redis")
	defer span.End()
	jsonBytes, err := json.Marshal(CoreMemories)
	if err != nil {
		slog.Error("Got this error while trying to set the core memories.. marshalling the json", "error", err, "userId", userId)
		return err
	}
	err = r.RedisClient.Set(ctx, userId, jsonBytes, 0).Err()
	if err != nil {
		slog.Error("Got this error while trying to set the core memories", "error", err, "userId", userId)
		return err
	}
	return nil
}

func (r *RedisCoreMemoryCache) DeleteCoreMemory(CoreMemoryId string, ctx context.Context) error {
	ctx, span := Tracer.Start(ctx, "Deleting Core Memories in Redis")
	defer span.End()
	//TODO: Setup a system for deleting core memories ..

	// err := r.RedisClient.del

	return nil
}
