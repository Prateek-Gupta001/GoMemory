package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
	llm, err := llm.NewGeminiLLM()
	if err != nil {
		slog.Info("Got this error while creating new Gemini LLM", "error", err)
		panic(err)
	}
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
	agent := &MemoryAgent{
		vectordb,
		llm,
		embed,
		&redis.RedisCoreMemoryCache{},
		js}
	return agent
}

func TestSumbitMemoryInsertionRequest(t *testing.T) {
	slog.Info("Beginning test")
	fmt.Println("Beginning Test")
	memJob := types.MemoryInsertionJob{
		ReqId:    "1234",
		UserId:   "user_123",
		Messages: []types.Message{},
	}
	agent := NewtestMemoryAgent()
	go func() {
		agent.JSClient.Subscribe("memory_work", func(msg *nats.Msg) {
			fmt.Printf("Worker got a memory Job: %s\n", string(msg.Data))
			memJob := &types.MemoryInsertionJob{}
			if err := json.Unmarshal(msg.Data, memJob); err != nil {
				slog.Error("error while unmarshalling NATS-jetstream data", "error", err)
				t.Errorf("Memory Insertion Job wasn't in NATS!")
			}
			// InsertMemory(memJob)
		})
	}()
	time.Sleep(time.Second)
	agent.SumbitMemoryInsertionRequest(memJob)
	time.Sleep(time.Second * 4)
}

func TestInsertMemory(t *testing.T) {
	agent := NewtestMemoryAgent()
	memJob := types.MemoryInsertionJob{
		ReqId:  "1234",
		UserId: "user_123",
		Messages: []types.Message{
			{
				Role:    types.RoleUser,
				Content: "I wanna go to Rome and Paris ... I mean I live in Italy but I long for Sicily",
			},
		},
	}
	fmt.Println("agent", agent)
	err := agent.InsertMemory(&memJob)
	if err != nil {
		t.Error("Got this error right here", err)
	}
}

func TestGetMemories(t *testing.T) {
	agent := NewtestMemoryAgent()
	start := time.Now()
	m, err := agent.GetMemories("My car was towed yesterday what should I do?", "user_123", "1234", 0.65, t.Context())
	if err != nil {
		t.Error("ERROR ", err)
		t.Fail()
	}
	end := time.Since(start)
	fmt.Println("Time taken for Getting Memories is", "time", end)
	fmt.Println("Memories", m)
}
