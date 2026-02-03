package llm

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Prateek-Gupta001/GoMemory/types"
)

func TestGenerateMemoryText(t *testing.T) {
	llm, err := NewGeminiLLM()
	if err != nil {
		t.Fatal("Got this error while trying to generate a llm", "error", err)
	}
	messagesCase3 := []types.Message{
		{
			Role:    types.RoleUser,
			Content: "Yo me and that bitch are back together AND I got the kids ... let's fucking go boiii",
		},
	}

	oldMemoriesCase3 := []types.Memory{
		{
			Memory_Id:   "mem_01", // <--- Targeted for DELETE (too vague)
			Memory_text: "User works on backend systems.",
			UserId:      "user_123",
		},
		{
			Memory_Id:   "mem_02",
			Memory_text: "User has children and is currently undergoing a divorce from his wife.",
			UserId:      "user_123",
		},
	}
	res, err := llm.GenerateMemoryText(messagesCase3, oldMemoriesCase3, context.Background())
	if err != nil {
		slog.Warn("Got this error", "error", err)
	}
	slog.Info("res ", "res", res)
}
