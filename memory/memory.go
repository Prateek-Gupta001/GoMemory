package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Prateek-Gupta001/GoMemory/embed"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/Prateek-Gupta001/GoMemory/vectordb"
	"github.com/nats-io/nats.go"
)

type Memory interface {
	GetMemories(user_query string, userId string, threshold float32) ([]string, error) //For normal messages
	DeleteMemory(payloadId string) error                                               //from the db
	SumbitMemoryInsertionRequest(memJob types.MemoryInsertionJob) error
	GetAllMemories(userId string) ([]string, error)
	// in the future: delete user's memories and delete memory by Id...
}

type MemoryAgent struct {
	Vectordb    vectordb.VectorDB
	LLM         llm.LLM
	EmbedClient embed.Embed
	NatClient   *nats.Conn
}

func NewMemoryAgent(vectordb vectordb.VectorDB, llm llm.LLM, embedClient embed.Embed, nc *nats.Conn, queueLen int, numWorker int) (*MemoryAgent, error) {
	m := &MemoryAgent{
		Vectordb:    vectordb,
		LLM:         llm,
		EmbedClient: embedClient,
		NatClient:   nc,
	}
	for i := 0; i < numWorker; i++ {
		go m.MemoryWorker(i)
	}
	return m, nil
}

func (m *MemoryAgent) MemoryWorker(id int) {
	slog.Info("Memory agent is up and running!", "id", id)
	m.NatClient.Subscribe("memory_work", func(msg *nats.Msg) {
		fmt.Printf("Worker got a memory Job: %s\n", string(msg.Data))
		memJob := &types.MemoryInsertionJob{}
		if err := json.Unmarshal(msg.Data, memJob); err != nil {
			slog.Error("error while unmarshalling NATS-jetstream data", "error", err)
		}
		if err := m.InsertMemory(memJob); err != nil {
			slog.Info("Memory worker encountered an error while working", "error", err, "reqId", memJob.ReqId, "userId", memJob.UserId)
		}
	})
}

func (m *MemoryAgent) SumbitMemoryInsertionRequest(memJob types.MemoryInsertionJob) error {

	slog.Info("Memory Job inserted successfully into NATS-Jetstream ", "reqId", memJob.ReqId)
	memJson, err := json.Marshal(memJob)
	if err != nil {
		slog.Info("Got this error while marshalling the MemoryInsertionJob ", "error", err)
		return err
	}
	err = m.NatClient.Publish("memory_work", memJson)
	return err
}

func (m *MemoryAgent) GetMemories(text string, userId string, threshold float32) ([]string, error) {

	return nil, nil
}

func (m *MemoryAgent) DeleteMemory(payloadId string) error {
	return nil
}

func (m *MemoryAgent) InsertMemory(memjob *types.MemoryInsertionJob) error {
	//take the messages and pass it to llm -> get query
	ctx, cancel_ctx := context.WithTimeout(context.Background(), time.Second*500)
	defer cancel_ctx()
	expandedQuery, err := m.LLM.ExpandQuery(memjob.Messages, ctx)
	if err != nil {
		slog.Info("Got this error message here while trying to generate expanded query", "error", err, "reqId", memjob.ReqId)
		return err
	}
	if strings.ToLower(expandedQuery) == "skip" {
		slog.Info("Memory Insertion is NOT REQUIRED!", "messages", memjob.Messages)
		return nil
	}
	DenseEmbedding, SparseEmbedding, err := m.EmbedClient.GenerateEmbeddings([]string{expandedQuery})
	if err != nil {
		slog.Info("Got this error message here while trying to generate expanded query Embeddings", "error", err, "reqId", memjob.ReqId)
		return err
	}
	//take query and pass it to qdrant
	//Here len of Embedding will be 0
	slog.Info("Len of the emebddings should be in harmony", "len(DenseEmbedding)", len(DenseEmbedding), "len(SparseEmbedding)", len(SparseEmbedding), "num", 1)
	similarityResults, err := m.Vectordb.GetSimilarMemories(DenseEmbedding[0], SparseEmbedding[0], memjob.UserId, ctx)
	if err != nil {
		slog.Info("Got this error message here while trying to get similarity results with the expanded query", "error", err, "reqId", memjob.ReqId)
		return err
	}
	//get the results and pass it to llm
	NewMemories, err := m.LLM.GenerateMemoryText(memjob.Messages, similarityResults)
	if err != nil {
		slog.Info("Got this error message here while trying to generate new memory text", "error", err, "reqId", memjob.ReqId)
		return err
	}
	DenseEmbedding, SparseEmbedding, err = m.EmbedClient.GenerateEmbeddings(NewMemories)
	//get llm response and pass it to qdrant
	slog.Info("New memories are", "memories", NewMemories)
	err = m.Vectordb.InsertNewMemories(DenseEmbedding, SparseEmbedding, NewMemories, ctx) //will take in userId as well
	if err != nil {
		slog.Info("Got this error while trying to generate insert the new memories into the vector db", "error", err, "reqId", memjob.ReqId)
	}
	//update the entry in the database.
	return nil
}

func LastUserContent(messages []types.Message) (string, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleUser {
			return messages[i].Content, true
		}
	}
	return "", false
}

func (m *MemoryAgent) GetAllMemories(userId string) ([]string, error) {
	return nil, nil
}
