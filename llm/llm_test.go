package llm

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestGenerateMemoryText_GodMode(t *testing.T) {
	llm, err := NewGeminiLLM()
	if err != nil {
		t.Fatal("Failed to initialize LLM", "error", err)
	}

	// 1. THE MULTI-TURN CONVERSATION (The "Firehose" of data)
	messagesGodMode := []types.Message{
		{
			Role:    types.RoleUser,
			Content: "Man, I am absolutely gutted. The Google L5 interview was a total disaster. I froze on the graph traversal problem.",
		},
		{
			Role:    types.RoleAssistant,
			Content: "I'm so sorry to hear that. Interview nerves can get the best of anyone. Do you think there's any chance of recovery?",
		},
		{
			Role:    types.RoleUser,
			Content: "Nah, I bombed it. But honestly? It's a blessing in disguise. I actually just signed the contract with that Neural Interface startup in Tokyo we talked about!",
		},
		{
			Role:    types.RoleAssistant,
			Content: "Tokyo?! That is an incredible pivot! Congratulations! Are you ready for such a big move?",
		},
		{
			Role:    types.RoleUser,
			Content: "Born ready. I'm flying out Sunday. I'm actually selling my car and my desktop PC. \nI'm going fully minimal: just me and a MacBook Air. \nAlso, since Sarah and I split up last month, there's nothing really holding me back in SF anymore. It's a clean slate.",
		},
		{
			Role:    types.RoleUser,
			Content: "Oh, and I'm switching to Rust for this role. No more Python spaghetti code for me. \nI've also started waking up at 5 AM for running. No more late-night Valorant sessions.",
		},
	}

	// 2. EXISTING CORE MEMORIES (The "Identity" Targets)
	oldMemoriesCore := []types.Memory{
		{
			Memory_Id:   "core_01",
			Type:        types.MemoryTypeCore,
			Memory_text: "User lives in San Francisco, CA.", // TARGET: DELETE (Moving to Tokyo)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "core_02",
			Type:        types.MemoryTypeCore,
			Memory_text: "User is in a long-term relationship with Sarah.", // TARGET: DELETE (Split up)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "core_03",
			Type:        types.MemoryTypeCore,
			Memory_text: "User is a Senior Python Engineer.", // TARGET: UPDATE/DELETE (Switching to Rust)
			UserId:      "user_god_mode",
		},
	}

	// 3. EXISTING GENERAL MEMORIES (The "Context" Targets)
	oldMemoriesGeneral := []types.Memory{
		{
			Memory_Id:   "gen_101",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User is preparing for Google L5 interviews.", // TARGET: DELETE (Event passed/failed)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "gen_102",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User owns a custom-built Gaming PC and a Toyota Camry.", // TARGET: DELETE (Selling both)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "gen_103",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User is a 'Night Owl' and typically stays up late gaming.", // TARGET: DELETE (5 AM running, no more Valorant)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "gen_104",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User primarily codes in Python and Django.", // TARGET: DELETE (No more Python spaghetti)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "gen_105",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User is allergic to shellfish.", // NO CONFLICT: Must keep (Control Group)
			UserId:      "user_god_mode",
		},
		{
			Memory_Id:   "gen_106",
			Type:        types.MemoryTypeGeneral,
			Memory_text: "User plays Valorant competitively.", // TARGET: DELETE (No more sessions)
			UserId:      "user_god_mode",
		},
	}

	// 4. EXECUTE
	slog.Info("Starting GOD MODE Test...")
	res, err := llm.GenerateMemoryText(messagesGodMode, oldMemoriesCore, oldMemoriesGeneral, context.Background())
	if err != nil {
		t.Fatalf("LLM Failed: %v", err)
	}

	// 5. LOG RESULTS
	slog.Info("GOD MODE RESULT", "step_1_reasoning", res.Reasoning)

	// Validation Logic (for your eyes):
	// Check if core_01, core_02 are DELETED.
	// Check if gen_101, gen_102, gen_103, gen_104, gen_106 are DELETED.
	// Check if gen_105 is PRESERVED.
	// Check for INSERTs: "Lives in Tokyo", "Single", "Rust Developer", "5 AM Runner", "Uses MacBook Air".

	printJSON(res) // Helper to pretty print the output
}

func printJSON(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
