package vectordb

import (
	"context"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/qdrant/go-client/qdrant"
)

type VectorDB interface {
	GetSimilarityResults(embedding types.Embedding) ([]string, error)
	InsertNewMemories() error
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

func (qdb *QdrantMemoryDB) GetSimilarityResults(embedding types.Embedding) ([]string, error) {
	return nil, nil
}

func (qdb *QdrantMemoryDB) InsertNewMemories() error {

	return nil
}
