package memory

import (
	// "encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Prateek-Gupta001/GoMemory/embed"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	"github.com/Prateek-Gupta001/GoMemory/redis"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/Prateek-Gupta001/GoMemory/vectordb"
	"github.com/nats-io/nats.go"
)

func NewtestMemoryAgent() *MemoryAgent {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		slog.Error("can't connect to NATS", "error", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		panic(err)
	}

	// 3. Define the Stream (The "Hard Drive" storage)
	// This is where you get .AddStream()
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "MEMORY_SYSTEM",
		Subjects: []string{"memory_work"}, // This stream listens to this subject
		// Storage:  nats.FileStorage,        // <--- SURVIVES POWER OUTAGE //For production, uncomment this line!
	})
	if err != nil {
		panic(err)
	}
	defer nc.Close()
	slog.Info("NATS JetStream is up!")
	// llm, err := llm.NewGeminiLLM()
	// if err != nil {
	// 	slog.Info("Got this error while creating new Gemini LLM", "error", err)
	// 	panic(err)
	// }
	slog.Info("Setting up Qdrant")
	vectordb, err := vectordb.NewQdrantMemoryDB()
	if err != nil {
		slog.Info("Got this error while creating new Qdrant Memory db", "error", err)
		panic(err)
	}
	slog.Info("Setting up new embed client")
	embed, err := embed.NewEmbeddingClient("localhost:50051")
	if err != nil {
		slog.Info("Got this error while creating new Qdrant Memory db", "error", err)
		panic(err)
	}
	slog.Info("Finished")
	RC := redis.NewRedisCoreMemoryCache()
	agent := &MemoryAgent{
		vectordb,
		&llm.GeminiLLM{},
		embed,
		RC,
		js}
	return agent
}

// func TestSumbitMemoryInsertionRequest(t *testing.T) {
// 	slog.Info("Beginning test")
// 	fmt.Println("Beginning Test")
// 	memJob := types.MemoryInsertionJob{
// 		ReqId:    "1234",
// 		UserId:   "user_123",
// 		Messages: []types.Message{},
// 	}
// 	agent := NewtestMemoryAgent()
// 	go func() {
// 		agent.JSClient.Subscribe("memory_work", func(msg *nats.Msg) {
// 			fmt.Printf("Worker got a memory Job: %s\n", string(msg.Data))
// 			memJob := &types.MemoryInsertionJob{}
// 			if err := json.Unmarshal(msg.Data, memJob); err != nil {
// 				slog.Error("error while unmarshalling NATS-jetstream data", "error", err)
// 				t.Errorf("Memory Insertion Job wasn't in NATS!")
// 			}
// 			// InsertMemory(memJob)
// 		})
// 	}()
// 	time.Sleep(time.Second)
// 	agent.SumbitMemoryInsertionRequest(memJob)
// 	time.Sleep(time.Second * 4)
// }

// func TestInsertMemory(t *testing.T) {
// 	agent := NewtestMemoryAgent()
// 	memJob := types.MemoryInsertionJob{
// 		ReqId:  "1234",
// 		UserId: "user_123",
// 		Messages: []types.Message{
// 			{
// 				Role: types.RoleUser,
// 				Content: `I finally did it! I moved to Paris! It's amazing here.
// 			I actually sold that rusty Honda Civic before leaving Italy and just bought a bicycle to get around the city.

// 			Oh, and for the AI Gateway? I'm rewriting the whole thing in Rust now.
// 			Go and GORM were just giving me too many headaches, so I'm done with them for this project.`,
// 			},
// 		},
// 	}
// 	fmt.Println("agent", agent)
// 	err := agent.InsertMemory(&memJob)
// 	if err != nil {
// 		t.Error("Got this error right here", err)
// 	}
// }

func TestGetMemories(t *testing.T) {
	agent := NewtestMemoryAgent()
	start := time.Now()
	memories := []types.Message{
		{
			Role:    types.RoleAssistant,
			Content: "You mentioned the move was recent. How are you adjusting to the daily rhythm of the city?",
		},
		{
			Role: types.RoleUser,
			Content: `It's a workout, literally. My legs are burning every time I get to the office. 
			
			I honestly don't miss the insurance payments on my old ride, but I do miss the air conditioning. 
			Still, being able to just weave through the gridlock and park anywhere is a huge plus.`,
		},
	}
	query := ConstructContextualQuery(memories, 500)
	m, err := agent.GetMemories(query, "user_123", "1234", 0.65, t.Context())
	if err != nil {
		t.Error("ERROR ", err)
		t.Fail()
	}
	end := time.Since(start)
	fmt.Println("Time taken for Getting Memories is", "time", end)
	fmt.Println("Memories", m)
}

func ConstructContextualQuery(messages []types.Message, charLimit int) string {
	if len(messages) == 0 {
		return ""
	}

	var accumulatedParts []string
	currentLen := 0

	re := regexp.MustCompile(`[^.!?]+[.!?]+(\s|$)`)

	// 1. Iterate BACKWARDS through messages (Latest -> Oldest)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		sentences := re.FindAllString(content, -1)
		if len(sentences) == 0 {
			sentences = []string{content}
		}

		var msgParts []string

		for j := len(sentences) - 1; j >= 0; j-- {
			sent := strings.TrimSpace(sentences[j])
			msgParts = append([]string{sent}, msgParts...) // Prepend to keep order within message

			currentLen += len(sent)

			// Check limit inside the sentence loop
			if currentLen >= charLimit {
				break
			}
		}

		finalMsgContent := strings.Join(msgParts, " ")

		// Prepend this block to our master list of parts
		accumulatedParts = append([]string{finalMsgContent}, accumulatedParts...)

		if currentLen >= charLimit {
			break
		}
	}

	// Join all blocks with newlines to separate turns clearly
	return strings.Join(accumulatedParts, "\n")
}
