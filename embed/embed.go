package embed

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/Prateek-Gupta001/GoMemory/proto/embedding"
	"github.com/Prateek-Gupta001/GoMemory/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Embed interface defines methods for generating embeddings
type Embed interface {
	GenerateEmbeddings(user_query []string, ctx context.Context) ([]types.DenseEmbedding, []types.SparseEmbedding, error)
	GenerateDenseEmbedding(query string) (types.DenseEmbedding, error)
}

// EmbeddingClient handles gRPC communication with the embedding service
type EmbeddingClient struct {
	EmbedServiceUrl string
	conn            *grpc.ClientConn
	client          pb.EmbeddingServiceClient
}

// NewEmbeddingClient creates a new embedding client with connection pooling
func NewEmbeddingClient(EmbedServiceUrl string) (*EmbeddingClient, error) {
	// Create gRPC connection with proper options
	conn, err := grpc.NewClient(
		EmbedServiceUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	// Create the embedding service client
	client := pb.NewEmbeddingServiceClient(conn)

	return &EmbeddingClient{
		EmbedServiceUrl: EmbedServiceUrl,
		conn:            conn,
		client:          client,
	}, nil
}

// Close closes the gRPC connection
func (e *EmbeddingClient) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

var Tracer = otel.Tracer("Go-Memory")

// GenerateEmbeddings sends a gRPC request to generate both dense and sparse embeddings
// for a list of queries
func (e *EmbeddingClient) GenerateEmbeddings(user_query []string, ctx context.Context) ([]types.DenseEmbedding, []types.SparseEmbedding, error) {
	// Validate input
	ctx, span := Tracer.Start(ctx, "Generate Embeddings")
	defer span.End()
	span.SetAttributes(attribute.Float64("Num Queries", float64(len(user_query))))
	slog.Info("Got Embedding Generation Request!")
	if len(user_query) == 0 {
		return nil, nil, fmt.Errorf("user_query cannot be empty")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the request
	req := &pb.Queries{
		Queries: user_query,
	}

	// Make the gRPC call
	resp, err := e.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create embeddings: %w", err)
	}
	slog.Info("Embedding Generation was succesful!", "len(resp.DenseEmbeddings)", len(resp.DenseEmbeddings), "len(resp.SparseEmbeddings)", len(resp.SparseEmbeddings))

	// Convert protobuf response to types
	denseEmbeddings := make([]types.DenseEmbedding, len(resp.DenseEmbeddings))
	for i, denseEmb := range resp.DenseEmbeddings {
		denseEmbeddings[i] = types.DenseEmbedding{
			Values: denseEmb.Values,
		}
	}

	sparseEmbeddings := make([]types.SparseEmbedding, len(resp.SparseEmbeddings))
	for i, sparseEmb := range resp.SparseEmbeddings {
		sparseEmbeddings[i] = types.SparseEmbedding{
			Indices: sparseEmb.Indices,
			Values:  sparseEmb.Values,
		}
	}
	slog.Info("Generate Embeddings securely secured from the protobuf response format!")
	return denseEmbeddings, sparseEmbeddings, nil
}

// GenerateDenseEmbedding sends a gRPC request to generate only a dense embedding
// for a single query
func (e *EmbeddingClient) GenerateDenseEmbedding(query string) (types.DenseEmbedding, error) {
	// Validate input
	if query == "" {
		return types.DenseEmbedding{}, fmt.Errorf("query cannot be empty")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create the request
	req := &pb.Query{
		Query: query,
	}

	// Make the gRPC call
	resp, err := e.client.CreateDenseEmbedding(ctx, req)
	if err != nil {
		return types.DenseEmbedding{}, fmt.Errorf("failed to create dense embedding: %w", err)
	}

	// Convert protobuf response to types
	denseEmbedding := types.DenseEmbedding{
		Values: resp.Values,
	}

	return denseEmbedding, nil
}
