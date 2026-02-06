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

	// THE TRICKY INPUT
	// Contains: Emotional venting, A failure (Google), A success (Startup),
	// A location move, A hardware switch, A tech stack switch, and a Task request.
	messagesCaseComplex := []types.Message{
		{
			Role: types.RoleUser,
			Content: `The google interview sucked ass tbh... need to drink a monster and coffee combo lmao.
			what do you think .. is that a safe combo? I am starting to like drinking monster these days tho ... might make it a staple`,
		},
	}

	// OLD GENERAL MEMORIES (To be Pruned)
	oldMemoriesGeneral := []types.Memory{
		{
			Memory_Id:   "mem_101",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User is building a Chatbot Builder project in Python.", // CONFLICT: User said "scrap that"
			UserId:      "user_123",
		},
		{
			Memory_Id:   "mem_102",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User prefers Windows OS for development.", // CONFLICT: Switching to Mac
			UserId:      "user_123",
		},
		{
			Memory_Id:   "mem_103",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User is preparing for Google interviews.", // OBSOLETE: Event passed/failed
			UserId:      "user_123",
		},
		{
			Memory_Id:   "mem_104",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User likes coffee.", // NO CONFLICT: Should remain untouched
			UserId:      "user_123",
		},
	}

	// OLD CORE MEMORIES (To be Pruned)
	oldMemoriesCore := []types.Memory{
		{
			Memory_Id:   "mem_01",
			Type:        types.MemoryTypeCore,
			Memory_text: "User lives in Bangalore, India.", // CONFLICT: Moving to Berlin
			UserId:      "user_123",
		},
		{
			Memory_Id:   "mem_02",
			Type:        types.MemoryTypeCore,
			Memory_text: "User is a final year CS Student.", // CONFLICT: Implied ("Signed an offer" -> Professional)
			UserId:      "user_123",
		},
	}

	res, err := llm.GenerateMemoryText(messagesCaseComplex, oldMemoriesCore, oldMemoriesGeneral, context.Background())
	if err != nil {
		slog.Warn("Got this error", "error", err)
	}

	// Log the reasoning to see if it caught the nuances
	slog.Info("LLM Execution Result", "response", res)
}
