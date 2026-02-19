package redis

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

type CoreMemoryCache interface {
	CreateUser(context.Context) (string, error)
	GetCoreMemory(userId string, ctx context.Context) ([]types.Memory, error)
	SetCoreMemory(userId string, CoreMemories []types.Memory, ctx context.Context) error
	DeleteCoreMemory(CoreMemoryIds []string, userId string, ctx context.Context) error
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

func (r *RedisCoreMemoryCache) CreateUser(ctx context.Context) (string, error) {
	userId := uuid.New()
	err := r.SetCoreMemory(userId.String(), []types.Memory{}, ctx)
	if err != nil {
		return "", err
	}
	slog.Info("User Id generated is", "userId", userId.String())
	return userId.String(), nil
}

// Get Core Memories from the Redis Cache. It return nil,nil if the user has currently no core memories.
func (r *RedisCoreMemoryCache) GetCoreMemory(userId string, ctx context.Context) ([]types.Memory, error) {
	ctx, span := Tracer.Start(ctx, "Getting Core Memories from Redis")
	defer span.End()
	res, err := r.RedisClient.Get(ctx, userId).Bytes()
	if err != nil {
		if err == redis.Nil {
			slog.Info("Cache miss! The user has no core memories!", "userId", userId)
			return nil, types.ErrUserNotFound
		}
		slog.Error("Got this error while trying to get core memories of the user! redis error", "userId", userId, "error", err)
		return []types.Memory{}, err
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

func (r *RedisCoreMemoryCache) DeleteCoreMemory(CoreMemoryIds []string, userId string, ctx context.Context) error {
	ctx, span := Tracer.Start(ctx, "Deleting Core Memories in Redis")
	defer span.End()
	core_mems, err := r.GetCoreMemory(userId, ctx)
	if err != nil {
		slog.Warn("Got this error while trying to delete core memories", "error", err)
		return err
	}
	idsToRemove := make(map[string]bool, len(CoreMemoryIds))
	for _, id := range CoreMemoryIds {
		idsToRemove[id] = true
	}
	newCoreMem := make([]types.Memory, 0, len(CoreMemoryIds))
	for _, mem := range core_mems {
		if !idsToRemove[mem.Memory_Id] {
			newCoreMem = append(newCoreMem, mem)
		}
	}
	err = r.SetCoreMemory(userId, newCoreMem, ctx)
	if err != nil {
		slog.Warn("Got this error while trying to delete core memories", "error", err)
		return err
	}
	if len(newCoreMem) == len(core_mems) {
		slog.Info("Core Memory Ids provided don't exist for the user.")
	}

	return nil
}
