package memory

import (
	"context"
	"log/slog"

	"github.com/qdrant/go-client/qdrant"
)

type Memory interface {
	GetMemories(text string, userId string, threshold float32) ([]string, error)
	DeleteMemory(payloadId string) error //from the db
	AddMemory(memories []string) error
	GetAllMemories(userId string) ([]string, error)
	// in the future: delete user's memories and delete memory by Id...
}

type QdrantMemoryDB struct {
	Client *qdrant.Client
}

func NewQdrantMemoryDB() (*QdrantMemoryDB, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6335,
	})
	if err != nil {
		slog.Error("Got this error while trying to intialise the qdrant memory db!", "error", err)
		panic(err)
	}
	exists, err1 := client.CollectionExists(context.Background(), "Go_Memory_db")
	if err1 != nil {
		slog.Error("Got this error while checking if collection exists or not!", "error", err)
		return &QdrantMemoryDB{}, err
	}
	if !exists {
		slog.Info("new collection being created!")
		err = client.CreateCollection(context.Background(), &qdrant.CreateCollection{
			CollectionName: "Go_Memory_db",
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     384,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			slog.Error("Got this error while trying to create the collection", "error", err)
		}
	}
	return &QdrantMemoryDB{
		Client: client,
	}, nil
}

func (qm *QdrantMemoryDB) GetMemories(text string, userId string, threshold float32) ([]string, error) {

	return nil, nil
}

func (qm *QdrantMemoryDB) DeleteMemory(payloadId string) error {
	return nil
}

func (qm *QdrantMemoryDB) AddMemory(memories []string) error {
	return nil
}

func (qm *QdrantMemoryDB) GetAllMemories(userId string) ([]string, error) {
	return nil, nil
}
