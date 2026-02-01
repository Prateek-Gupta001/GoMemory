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
	GetMemories(user_query string, userId string, reqId string, threshold float32, ctx context.Context) ([]types.Memory, error) //For normal messages
	DeleteMemory(memoryIds []string, ctx context.Context) error                                                                 //from the db
	SumbitMemoryInsertionRequest(memJob types.MemoryInsertionJob) error
	GetAllUserMemories(userId string, ctx context.Context) ([]types.Memory, error)
	// in the future: delete user's memories and delete memory by Id...
}

type MemoryAgent struct {
	Vectordb    vectordb.VectorDB
	LLM         llm.LLM
	EmbedClient embed.Embed
	JSClient    nats.JetStreamContext
}

func NewMemoryAgent(vectordb vectordb.VectorDB, llm llm.LLM, embedClient embed.Embed, nc nats.JetStreamContext, queueLen int, numWorker int) (*MemoryAgent, error) {
	m := &MemoryAgent{
		Vectordb:    vectordb,
		LLM:         llm,
		EmbedClient: embedClient,
		JSClient:    nc,
	}
	for i := 0; i < numWorker; i++ {
		go m.MemoryWorker(i)
	}
	return m, nil
}

func (m *MemoryAgent) MemoryWorker(id int) {
	slog.Info("Memory agent is up and running!", "id", id)
	m.JSClient.QueueSubscribe("memory_work", "workers", func(msg *nats.Msg) {
		fmt.Printf("----------------------------------- Worker got a memory Job: %s\n ----------------------------------- \n", string(msg.Data))
		memJob := &types.MemoryInsertionJob{}
		if err := json.Unmarshal(msg.Data, memJob); err != nil {
			slog.Error("error while unmarshalling NATS-jetstream data", "error", err)
			msg.Term()
			return
		}
		if err := m.InsertMemory(memJob); err != nil {
			slog.Info("Memory worker encountered an error while working", "error", err, "reqId", memJob.ReqId, "userId", memJob.UserId)
			//TODO: Check from InsertMemory if its a deterministic error or not .. if its an API server issue or an OpenAI issue or an LLM issue
			//TODO: .. You would wanna retry the job .. in that case .. otherwise not!
			msg.Term()
			return
		}
		msg.Ack()
	})
}

func (m *MemoryAgent) SumbitMemoryInsertionRequest(memJob types.MemoryInsertionJob) error {

	slog.Info("Memory Job inserted successfully into NATS-Jetstream ", "reqId", memJob.ReqId)
	memJson, err := json.Marshal(memJob)
	if err != nil {
		slog.Info("Got this error while marshalling the MemoryInsertionJob ", "error", err)
		return err
	}
	_, err = m.JSClient.Publish("memory_work", memJson)
	return err
}

func (m *MemoryAgent) GetMemories(text string, userId string, reqId string, threshold float32, ctx context.Context) ([]types.Memory, error) {
	dense, sparse, err := m.EmbedClient.GenerateEmbeddings([]string{text})
	if err != nil {
		slog.Error("Got this error while generating emebddings", "error", err, "reqId", reqId)
		return nil, err
	}
	Memories, err := m.Vectordb.GetSimilarMemories(dense[0], sparse[0], userId, threshold, ctx)
	if err != nil {
		slog.Error("Got this error while getting similar memories!", "error", err, "reqId", reqId)
		return nil, err
	}
	return Memories, nil
}

func (m *MemoryAgent) DeleteMemory(memoryIds []string, ctx context.Context) error {
	err := m.Vectordb.DeleteMemories(memoryIds, ctx)
	if err != nil {
		return err
	}
	return nil
}

func (m *MemoryAgent) InsertMemory(memjob *types.MemoryInsertionJob) error {
	//take the messages and pass it to llm -> get query
	ctx, cancel_ctx := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel_ctx()
	slog.Info("Insert Memory Request recieved!", "jobId", memjob.ReqId)
	expandedQuery, err := m.LLM.ExpandQuery(memjob.Messages, ctx)
	if err != nil {
		slog.Info("Got this error message here while trying to generate expanded query", "error", err, "reqId", memjob.ReqId)
		return err
	}
	slog.Info("Expanded query has been prepared by the LLM!", "query", expandedQuery)

	if strings.ToLower(expandedQuery) == "skip" {
		slog.Info("Memory Insertion is NOT REQUIRED!", "messages", memjob.Messages)
		return nil
	}
	slog.Info("Preparing Embedding Generation!")
	DenseEmbedding, SparseEmbedding, err := m.EmbedClient.GenerateEmbeddings([]string{"_Query_" + expandedQuery})
	if err != nil {
		slog.Info("Got this error message here while trying to generate expanded query Embeddings", "error", err, "reqId", memjob.ReqId)
		return err
	}
	//take query and pass it to qdrant
	//Here len of Embedding will be 0
	slog.Info("Len of the emebddings should be in harmony", "len(DenseEmbedding)", len(DenseEmbedding), "len(SparseEmbedding)", len(SparseEmbedding), "num", 1)
	similarityResults, err := m.Vectordb.GetSimilarMemories(DenseEmbedding[0], SparseEmbedding[0], memjob.UserId, memjob.Threshold, ctx)
	if err != nil {
		slog.Info("Got this error message here while trying to get similarity results with the expanded query", "error", err, "reqId", memjob.ReqId)
		return err
	}
	//get the results and pass it to llm
	MemoryOutput, err := m.LLM.GenerateMemoryText(memjob.Messages, similarityResults, ctx)
	if err != nil {
		slog.Info("Got this error message here while trying to generate new memory text", "error", err, "reqId", memjob.ReqId)
		return err
	}
	var memories []string
	var memoryIds []string //These are the memory ids to be deleted from the database!!
	for _, memory := range MemoryOutput.Actions {
		if memory.ActionType == "INSERT" {
			slog.Info("got an insert!")
			if memory.TargetMemoryID != nil {
				slog.Info("Damn .. llm made a mistake and gave a target memory Id in an INSERT request", "targetMemoryId", memory.TargetMemoryID)
			}
			memories = append(memories, *memory.Payload)
		}
		if memory.ActionType == "DELETE" {
			slog.Info("got a DELETE!")
			if memory.Payload != nil {
				slog.Info("Damn .. llm made a mistake and gave a payload in an DELETE request", "payload", memory.Payload)
			}
			memoryIds = append(memoryIds, *memory.TargetMemoryID)
		}
	}

	DenseEmbedding, SparseEmbedding, err = m.EmbedClient.GenerateEmbeddings(memories)
	if len(memoryIds) != 0 {
		slog.Info("Memories to delete are: ", "memoryIds", memoryIds)
		if err := m.Vectordb.DeleteMemories(memoryIds, ctx); err != nil {
			slog.Error("Got this error while deleting old memories of the user", "error", err)
		}
	}
	//get llm response and pass it to qdrant
	if len(memories) != 0 {
		slog.Info("Len of the emebddings should be in harmony", "len(DenseEmbedding)", len(DenseEmbedding), "len(SparseEmbedding)", len(SparseEmbedding), "memories", len(memories))
		slog.Info("Memories to insert are: ", "memories", memories)
		err = m.Vectordb.InsertNewMemories(DenseEmbedding, SparseEmbedding, memories, memjob.UserId, ctx) //will take in userId as well
		if err != nil {
			slog.Info("Got this error while trying to insert the new memories into the vector db", "error", err, "reqId", memjob.ReqId)
		}
	}
	//update the entry in the database.
	return nil
}

func (m *MemoryAgent) GetAllUserMemories(userId string, ctx context.Context) ([]types.Memory, error) {
	mem, err := m.Vectordb.GetAllUserMemories(userId, ctx)
	if err != nil {
		slog.Error("Got this error while trying to get all memories of the user (in the memory agent)", "error", err, "userId", userId)
		return nil, err
	}
	return mem, nil
}

func LastUserContent(messages []types.Message) (string, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleUser {
			return messages[i].Content, true
		}
	}
	return "", false
}
