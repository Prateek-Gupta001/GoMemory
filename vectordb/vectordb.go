package vectordb

import (
	"context"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

type VectorDB interface {
	GetSimilarMemories(types.DenseEmbedding, types.SparseEmbedding, string, context.Context) ([]string, error)
	InsertNewMemories([]types.DenseEmbedding, []types.SparseEmbedding, []string, string, context.Context) error
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
			// 1. Define the Dense Vector with a specific name (e.g., "dense")
			VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
				"dense": {
					Size:     384,
					Distance: qdrant.Distance_Cosine,
				},
			}),
			// 2. Define the Sparse Vector with a specific name (e.g., "sparse")
			SparseVectorsConfig: qdrant.NewSparseVectorsConfig(
				map[string]*qdrant.SparseVectorParams{
					"sparse": {},
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

func (qdb *QdrantMemoryDB) GetSimilarMemories(DenseEmbedding types.DenseEmbedding, SparseEmbedding types.SparseEmbedding, userId string, ctx context.Context) ([]string, error) {
	res, err := qdb.Client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: "Go_Memory_db",
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("userId", userId),
			}},
		WithPayload: qdrant.NewWithPayload(true),
		Prefetch: []*qdrant.PrefetchQuery{
			{
				Query: qdrant.NewQuerySparse(SparseEmbedding.Indices, SparseEmbedding.Values),
				Using: qdrant.PtrOf("sparse"),
			},
			{
				Query: qdrant.NewQueryDense(DenseEmbedding.Values),
				Using: qdrant.PtrOf("dense"),
			},
		},
		Query: qdrant.NewQueryFusion(qdrant.Fusion_RRF),
	})
	if err != nil {
		slog.Error("Got this error while trying to get similar memories", "error", err)
		return nil, err
	}
	var x []string
	for _, r := range res {
		y := r.Payload
		x = append(x, string(y["Memory"].GetStringValue()))
	}
	return x, nil
}

func (qdb *QdrantMemoryDB) InsertNewMemories(DenseEmbedding []types.DenseEmbedding, SparseEmbeddings []types.SparseEmbedding, memories []string, userId string, ctx context.Context) error {
	var Points []*qdrant.PointStruct
	for idx, sp := range SparseEmbeddings {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(memories[idx])).String()
		Points = append(Points,
			&qdrant.PointStruct{
				Id: qdrant.NewIDUUID(id),
				Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
					"sparse": qdrant.NewVectorSparse(
						sp.Indices,
						sp.Values),
					"dense": qdrant.NewVectorDense(DenseEmbedding[idx].Values),
				}),
				Payload: qdrant.NewValueMap(map[string]any{
					"userId": userId,
					"memory": memories[idx],
				}),
			})
	}
	_, err := qdb.Client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: "Go_Memory_db",
		Points:         Points,
	})
	if err != nil {
		slog.Info("Got this error while upserting qdrant points", "error", err)
		return err
	}
	return nil
}
