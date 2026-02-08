package redis

import (
	"testing"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func NewMockRedisCache() *RedisCoreMemoryCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		Protocol: 2,
	})
	return &RedisCoreMemoryCache{
		RedisClient: rdb,
	}
}
func TestGetCoreMemories(t *testing.T) {

	r := NewMockRedisCache()
	r.RedisClient.FlushDB(t.Context())
	defer r.RedisClient.FlushDB(t.Context())
	assert := assert.New(t) // Initialize assert for this test
	ctx := t.Context()

	testUserId := "user_test"
	expectedMemories := []types.Memory{
		{Memory_text: "Lives in Paris", Memory_Id: "1", UserId: "user_test"},
		{Memory_text: "Name is Prateek", Memory_Id: "2", UserId: "user_test"},
	}

	err := r.SetCoreMemory(testUserId, expectedMemories, ctx)
	assert.NoError(err, "Setting core memories should not fail")

	missMemories, err := r.GetCoreMemory("user_Test_does_not_exist", ctx)
	assert.NoError(err, "Cache miss should not return an error")
	assert.Empty(missMemories, "Cache miss should return an empty slice")

	actualMemories, err := r.GetCoreMemory(testUserId, ctx)
	assert.NoError(err, "Getting core memories should not fail")

	assert.Equal(expectedMemories, actualMemories, "Retrieved memories must match the stored ones")

	assert.Len(actualMemories, 2)
}
