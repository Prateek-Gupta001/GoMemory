package memory

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/Prateek-Gupta001/GoMemory/embed"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/Prateek-Gupta001/GoMemory/vectordb"
	"github.com/nats-io/nats.go"
)

func NewtestMemroyAgent() *MemoryAgent {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		slog.Error("can't connect to NATS", "error", err)
	}
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
		nc}
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
// 	agent := NewtestMemroyAgent()
// 	go func() {
// 		agent.NatClient.Subscribe("memory_work", func(msg *nats.Msg) {
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

// func newTestMemoryWorker(id int) {
// 	slog.Info("Memory agent is up and running!", "id", id)

// }

func TestInsertMemory(t *testing.T) {
	agent := NewtestMemroyAgent()
	memJob := types.MemoryInsertionJob{
		ReqId:  "1234",
		UserId: "user_123",
		Messages: []types.Message{
			{
				Role:    types.RoleUser,
				Content: "Regarding my backend work, I'm specifically using Go with Gin and GORM for the API layer. Do you know any cool stuff about cars? I own a Honda civic .. which is pretty dang fast .. but a little rusty imo .. lmao",
			},
			{
				Role:    types.RoleAssistant,
				Content: "okay how can I help you with your backend work?",
			},
			{
				Role:    types.RoleUser,
				Content: "Yeah Actually I am building an AI Gateway .. which uses semantic caching to provide fast answers to repeating queries .. What do you know about all that?",
			},
		},
	}
	fmt.Println("agent", agent)
	err := agent.InsertMemory(&memJob)
	if err != nil {
		t.Error("Got this error right here", err)
	}

}
