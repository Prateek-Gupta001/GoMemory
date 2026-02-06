package vectordb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

type VectorDB interface {
	GetSimilarMemories(types.DenseEmbedding, types.SparseEmbedding, string, float32, context.Context) ([]types.Memory, error)
	InsertNewMemories([]types.DenseEmbedding, []types.SparseEmbedding, []string, string, context.Context) error
	DeleteMemories([]string, context.Context) error
	GetAllUserMemories(userId string, ctx context.Context) ([]types.Memory, error)
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
	// client.DeleteCollection(context.Background(),"Go_Memory_db")
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
		fieldType := qdrant.FieldType_FieldTypeKeyword

		_, err := client.CreateFieldIndex(context.Background(), &qdrant.CreateFieldIndexCollection{
			CollectionName: "Go_Memory_db",
			FieldName:      "userId",
			FieldType:      &fieldType,
		})
		if err != nil {
			slog.Error("Got this error while trying to make the userId a field Index.", "error", err)
		}
	}
	return &QdrantMemoryDB{
		Client: client,
	}, nil
}

func (qdb *QdrantMemoryDB) GetSimilarMemories(DenseEmbedding types.DenseEmbedding, SparseEmbedding types.SparseEmbedding, userId string, threshold float32, ctx context.Context) ([]types.Memory, error) {
	res, err := qdb.Client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: "Go_Memory_db",
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("userId", userId),
			}},
		ScoreThreshold: &threshold,
		WithPayload:    qdrant.NewWithPayload(true),
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
	var Memories []types.Memory
	if len(res) == 0 {
		slog.Info("No Memories of the user found", "userId", userId)
		return nil, nil
	}
	for _, r := range res {
		y := r.Payload
		slog.Info("memory is", "memory", y["Memory"].GetStringValue())
		_, ok := y["Memory"]
		if !ok {
			slog.Error("Payload is missing 'Memory' key", "id", r.Id)
			continue
		}
		Memories = append(Memories, types.Memory{
			Memory_text: string(y["Memory"].GetStringValue()),
			Memory_Id:   r.Id.GetUuid(),
			Type:        types.MemoryTypeGeneral,
			UserId:      userId,
		})

	}
	slog.Info("Similar Memories are being returned from qdrant!", "memories", Memories)
	return Memories, nil
}

func (qdb *QdrantMemoryDB) InsertNewMemories(DenseEmbedding []types.DenseEmbedding, SparseEmbeddings []types.SparseEmbedding, memories []string, userId string, ctx context.Context) error {
	var Points []*qdrant.PointStruct
	for idx, sp := range SparseEmbeddings {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(memories[idx]+userId)).String()
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
					"Memory": memories[idx],
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
	slog.Info("Memory insertion was successful!")
	return nil
}

func (qdb *QdrantMemoryDB) GetAllUserMemories(userId string, ctx context.Context) ([]types.Memory, error) {
	res, err := qdb.Client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: "Go_Memory_db",
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("userId", userId),
			},
		},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		slog.Error("Got this error while trying to get all memories of the user", "error", err, "userId", userId)
		return nil, err
	}
	var Memories []types.Memory
	for _, r := range res {
		y := r.Payload
		slog.Info("memory is", "memory", y["Memory"].GetStringValue())
		_, ok := y["Memory"]
		if !ok {
			slog.Error("Payload is missing 'Memory' key", "id", r.Id)
			continue
		}
		Memories = append(Memories, types.Memory{
			Memory_text: string(y["Memory"].GetStringValue()),
			Memory_Id:   r.Id.GetUuid(),
			Type:        types.MemoryTypeGeneral,
			UserId:      userId,
		})
	}
	slog.Info("All the user Memories are being returned from qdrant!", "memories", Memories)
	return Memories, nil
}

func (qdb *QdrantMemoryDB) DeleteMemories(memoryIds []string, ctx context.Context) error {
	var qdrantPointIds []*qdrant.PointId
	for _, memId := range memoryIds {
		qdrantPointIds = append(qdrantPointIds, qdrant.NewIDUUID(memId))
	}
	retrieveResp, err := qdb.Client.Get(ctx, &qdrant.GetPoints{
		CollectionName: "Go_Memory_db",
		Ids:            qdrantPointIds,
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: false}}, // We don't need the data, just existence
		WithVectors:    &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false}}, // Optimization: Don't fetch vectors
	})
	if err != nil {
		slog.Error("failed to check points existence:", "error", err)
	}

	// 3. VERIFY: Did we find anything?
	slog.Info("Did we find as many memories as there are Ids", "num of memoryIds", len(memoryIds), "num of retrieved results", len(retrieveResp))
	if len(retrieveResp) != len(memoryIds) {
		slog.Info("LLM Probably hallucinated and gave an invalid memory id ... it doesn't exist in the db")
	}
	if len(retrieveResp) == 0 {
		slog.Info("No points found!", "points", memoryIds)
		return fmt.Errorf("No points found!")
	}
	_, err = qdb.Client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: "Go_Memory_db",
		Points:         qdrant.NewPointsSelectorIDs(qdrantPointIds),
	})
	if err != nil {
		slog.Error("Got this error while Deleting Memories", "error", err)
		return err
	}
	slog.Info("Deletion of the Memories was succesful!")

	return nil
}
