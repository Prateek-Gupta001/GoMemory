package redis

import (
	"fmt"
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
	assert := assert.New(t) // Initialize assert for this test
	ctx := t.Context()

	testUserId := "user_test_2"
	expectedMemories := []types.Memory{}

	err := r.SetCoreMemory(testUserId, expectedMemories, ctx)
	assert.NoError(err, "Setting core memories should not fail")

	missMemories, err := r.GetCoreMemory("user_Test_does_not_exist", ctx)
	fmt.Println("error for user that doesn't have an acc", err)
	assert.Error(err, fmt.Errorf("User doesn't exist!"))
	assert.Empty(missMemories, "Cache miss should return an empty slice")
	fmt.Println("MISS MEMORIES", missMemories)

	actualMemories, err := r.GetCoreMemory(testUserId, ctx)
	assert.NoError(err, "Getting core memories should not fail")
	fmt.Println("ACTUAL MEMORIES", actualMemories)
	if missMemories == nil {
		fmt.Println("len of miss memories", len(missMemories))
		fmt.Println("miss memories is nil")
	}
	fmt.Println("len of actual memories", len(actualMemories))
	if actualMemories == nil {
		fmt.Println("len of actual memories", len(actualMemories))
		fmt.Println("actual memories is nil")
	}

	assert.Len(actualMemories, 0)
}

//mmm .. so what should I do here .... having a userId as an account being created and core memories as an empty list is okay
//but here .... if that's the case .. and if core memories don't exist .. then you would be returning nil .. which breaks the response from
//get memories if userId doesn't exist .. so the thing to do is .. to handle that case ... and return a nil response
//also ... to create user account ... you intialise an entry for it in .. redis ... and if it doesn't trigger redis.nil error
//then let memory insertion go via ... the account exists ...
