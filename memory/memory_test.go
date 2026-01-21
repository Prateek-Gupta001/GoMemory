package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

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

	agent := &MemoryAgent{
		&vectordb.QdrantMemoryDB{},
		&llm.GeminiLLM{},
		&embed.EmbeddingClient{},
		nc}
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
	agent := NewtestMemroyAgent()
	go func() {
		agent.NatClient.Subscribe("memory_work", func(msg *nats.Msg) {
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

func newTestMemoryWorker(id int) {
	slog.Info("Memory agent is up and running!", "id", id)

}
