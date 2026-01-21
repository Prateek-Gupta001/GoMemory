package vectordb

import (
	"context"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/qdrant/go-client/qdrant"
)

type VectorDB interface {
	GetSimilarMemories(types.DenseEmbedding, types.SparseEmbedding, string) ([]string, error)
	InsertNewMemories([]types.DenseEmbedding, []types.SparseEmbedding) error
}

type QdrantMemoryDB struct {
	Client *qdrant.Client
}

func NewQdrantMemoryDB() (*QdrantMemoryDB, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6336,
	})
	if err != nil {
		slog.Error("Got this error while trying to intialise the qdrant memory db!", "error", err)
		panic(err)
	}
	exists, err1 := client.CollectionExists(context.Background(), "Go_Memory_db")
	if err1 != nil {
		slog.Error("Got this error while checking if collection exists or not!", "error", err1)
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

func (qdb *QdrantMemoryDB) GetSimilarMemories(DenseEmbedding types.DenseEmbedding, SparseEmbedding types.SparseEmbedding, userId string) ([]string, error) {
	qdb.Client.Query(context.Background(), &qdrant.QueryPoints{
		CollectionName: "Go_Memory_db",
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("userId", userId),
			}},
		Prefetch: []*qdrant.PrefetchQuery{
			{
				Query: qdrant.NewQuerySparse(SparseEmbedding.Indices, SparseEmbedding.Values),
				Using: qdrant.PtrOf("sparse"),
			},
			{
				Query: qdrant.NewQueryDense(DenseEmbedding),
				Using: qdrant.PtrOf("dense"),
			},
		},
		Query: qdrant.NewQueryFusion(qdrant.Fusion_RRF),
	})

	return nil, nil
}

func (qdb *QdrantMemoryDB) InsertNewMemories(DenseEmbedding []types.DenseEmbedding, SparseEmbeddings []types.SparseEmbedding) error {

	return nil
}
