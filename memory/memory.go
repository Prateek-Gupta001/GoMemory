package memory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/embed"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/Prateek-Gupta001/GoMemory/vectordb"
)

type Memory interface {
	GetMemories(user_query string, userId string, threshold float32) ([]string, error) //For normal messages
	DeleteMemory(payloadId string) error                                               //from the db
	SumbitMemoryInsertionRequest(messages []types.Message, reqId string, ctx context.Context, userId string) error
	GetAllMemories(userId string) ([]string, error)
	// in the future: delete user's memories and delete memory by Id...
}

type MemoryAgent struct {
	vectordb                vectordb.VectorDB
	llm                     llm.LLM
	embedClient             embed.Embed
	MemoryInsertionJobQueue chan MemoryInsertionJob
}

func NewMemoryAgent(vectordb vectordb.VectorDB, llm llm.LLM, embedClient embed.Embed, queueLen int, numWorker int) (*MemoryAgent, error) {
	MemoryInsertionJobQueue := make(chan MemoryInsertionJob, queueLen)
	m := &MemoryAgent{
		vectordb:                vectordb,
		llm:                     llm,
		embedClient:             embedClient,
		MemoryInsertionJobQueue: MemoryInsertionJobQueue,
	}
	for i := 0; i < numWorker; i++ {
		go m.MemoryWorker(i)
	}
	return m, nil
}

type MemoryInsertionJob struct {
	ReqId    string
	UserId   string
	Messages []types.Message
	ctx      context.Context
}

func (m *MemoryAgent) MemoryWorker(id int) {
	slog.Info("Memory agent is up and running!", "id", id)
	for job := range m.MemoryInsertionJobQueue {
		err := m.InsertMemory(job.Messages, job.UserId, job.ctx)
		if err != nil {
			slog.Error("Got this error while inserting memory", "id", job.ReqId, "userId", job.UserId)
		}
		slog.Info("Memory Insertion Succesful", "id", job.ReqId, "userId", job.UserId)
	}
}

func (m *MemoryAgent) SumbitMemoryInsertionRequest(messages []types.Message, reqId string, ctx context.Context, userId string) error {
	select {
	case m.MemoryInsertionJobQueue <- MemoryInsertionJob{
		Messages: messages,
		ReqId:    reqId,
		ctx:      ctx,
		UserId:   userId,
	}:
		slog.Info("Memory Job inserted successfully!", "reqId", reqId)
		return nil
	default:
		slog.Info("Channel was full! Dropping this request ....")
		return fmt.Errorf("Too many requests on the server!")
	}
}

func (m *MemoryAgent) GetMemories(text string, userId string, threshold float32) ([]string, error) {

	return nil, nil
}

func (m *MemoryAgent) DeleteMemory(payloadId string) error {
	return nil
}

func (m *MemoryAgent) InsertMemory(messages []types.Message, reqId string, ctx context.Context) error {
	//take the messages and pass it to llm -> get query
	expandedQuery, err := m.llm.ExpandQuery(messages)
	if err != nil {
		slog.Info("Got this error message here while trying to generate expanded query", "error", err, "reqId", reqId)
		return err
	}
	Embedding, err := m.embedClient.GenerateEmbeddings(expandedQuery)
	if err != nil {
		slog.Info("Got this error message here while trying to generate expanded query Embeddings", "error", err, "reqId", reqId)
		return err
	}
	//take query and pass it to qdrant
	similarityResults, err := m.vectordb.GetSimilarityResults(Embedding)
	if err != nil {
		slog.Info("Got this error message here while trying to get similarity results with the expanded query", "error", err, "reqId", reqId)
		return err
	}
	//get the results and pass it to llm
	NewMemories, err := m.llm.GenerateMemoryText(messages, similarityResults)
	if err != nil {
		slog.Info("Got this error message here while trying to generate new memory text", "error", err, "reqId", reqId)
		return err
	}
	//get llm response and pass it to qdrant
	slog.Info("New memories are", "memories", NewMemories)
	err = m.vectordb.InsertNewMemories()
	if err != nil {
		slog.Info("Got this error while trying to generate insert the new memories into the vector db", "error", err, "reqId", reqId)
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
