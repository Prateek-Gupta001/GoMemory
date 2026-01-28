package llm

import (
	"context"
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
			Content: "Regarding my backend work, I'm specifically using Go with Gin and GORM for the API layer. Do you know any cool stuff about cars? I own a Honda civic .. which is pretty dang fast .. but a little rusty imo .. lmao",
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
			Memory_text: "User has a golden retriever.",
			UserId:      "user_123",
		},
	}
	llm.GenerateMemoryText(messagesCase3, oldMemoriesCase3, context.Background())
}
