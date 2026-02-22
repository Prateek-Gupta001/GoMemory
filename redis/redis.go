package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

type OperationalStore interface {
	CreateUser(context.Context) (string, error)
	GetCoreMemory(userId string, ctx context.Context) ([]types.Memory, error)
	SetCoreMemory(userId string, CoreMemories []types.Memory, ctx context.Context) error
	DeleteCoreMemory(CoreMemoryIds []string, userId string, ctx context.Context) error
	CreateReq(reqId string, ctx context.Context) error
	ChangeReqStatus(ctx context.Context, reqId string, Error string, status types.ReqStatus) error
	GetReqStatus(ctx context.Context, reqId string) (map[string]string, error)
}

type RedisOperationalStore struct {
	RedisClient *redis.Client
}

var Tracer = otel.Tracer("Go_Memory")

func NewRedisOperationalStore() *RedisOperationalStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password
		DB:       0,  // use default DB
		Protocol: 2,
	})
	return &RedisOperationalStore{
		RedisClient: rdb,
	}
}

func (r *RedisOperationalStore) CreateUser(ctx context.Context) (string, error) {
	userId := uuid.New()
	err := r.SetCoreMemory(userId.String(), []types.Memory{}, ctx)
	if err != nil {
		return "", err
	}
	slog.Info("User Id generated is", "userId", userId.String())
	return userId.String(), nil
}

func (r *RedisOperationalStore) GetReqStatus(ctx context.Context, reqId string) (map[string]string, error) {
	res, err := r.RedisClient.HGetAll(ctx, reqId).Result()
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *RedisOperationalStore) ChangeReqStatus(ctx context.Context, reqId string, Error string, status types.ReqStatus) error {
	_, err := r.RedisClient.HSet(ctx, reqId, []string{"status", string(status), "error", Error}).Result()
	if err != nil {
		slog.Warn("Got this error while trying to change the req status", "error", err)
		return err
	}
	slog.Info("Request status updation was successful!")
	return nil
}

func (r *RedisOperationalStore) CreateReq(reqId string, ctx context.Context) error {
	isNew, err := r.RedisClient.HSetNX(ctx, reqId, "status", string(types.Pending)).Result()
	if err != nil {
		slog.Error("Got this error while trying to HsetNX", "error", err)
		return err
	}

	if !isNew {
		slog.Info("Request that is being tried to create is NOT NEW!")
		// 2. The key exists. Fetch its actual current status.
		currentStatus, err := r.RedisClient.HGet(ctx, reqId, "status").Result()
		if err != nil {
			slog.Error("Failed to fetch existing status for duplicate check", "error", err, "reqId", reqId)
			return err
		}

		// 3. If it is anything OTHER than Failure, block it.
		// This covers Pending, Processing, and Success.
		if currentStatus != string(types.Failure) {
			slog.Info("Duplicate request rejected. Job already exists and has not failed.", "reqId", reqId, "currentStatus", currentStatus)
			return types.DuplicateError
		}

		slog.Info("Retrying a previously failed request. Overwriting status.", "reqId", reqId)
		_, err = r.RedisClient.HSet(ctx, reqId, "status", string(types.Pending)).Result()
		if err != nil {
			slog.Error("Failed to reset status to Pending", "error", err, "reqId", reqId)
			return err
		}
	}
	_, err = r.RedisClient.HSet(ctx, reqId, []string{"error", "", "createdAt", time.Now().String()}).Result()
	if err != nil {
		return err
	}

	// 4. Set expiration
	_, err = r.RedisClient.Expire(ctx, reqId, 24*time.Hour).Result()
	if err != nil {
		slog.Error("Got this error while trying to set the expiration for this reqId", "reqId", reqId, "error", err)
		return err
	}

	slog.Info("Request creation was successful!")
	return nil
}

// Get Core Memories from the Redis Cache. It return nil,nil if the user has currently no core memories.
func (r *RedisOperationalStore) GetCoreMemory(userId string, ctx context.Context) ([]types.Memory, error) {
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

func (r *RedisOperationalStore) SetCoreMemory(userId string, CoreMemories []types.Memory, ctx context.Context) error {
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

func (r *RedisOperationalStore) DeleteCoreMemory(CoreMemoryIds []string, userId string, ctx context.Context) error {
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
