package embed

import (
	"log/slog"
	"testing"
	"time"
)

// func TestGenerateEmbeddings(t *testing.T) {
// 	user_query := []string{"Hey there who are you"}
// 	embed, err := NewEmbeddingClient("localhost:50051")
// 	if err != nil {
// 		t.Error("Got this error while trying to create new embedding client", err)
// 	}
// 	s := time.Now()
// 	denseEmbed, sparseEmbed, err := embed.GenerateEmbeddings(user_query)
// 	end := time.Since(s)
// 	slog.Info("Time taken is", "time:", end)
// 	if err != nil {
// 		t.Fatal("Got this error while trying to create new embeddings", err)
// 	}
// 	slog.Info("attributes: ", "len(denseEmbed)", len(denseEmbed), "len(sparseEmbed)", len(sparseEmbed), "user_query", len(user_query))
// 	slog.Info("shape stuff: ", "len(denseEmbed)", len(denseEmbed[0].Values), "len(sparseEmbed[0].Indices)", len(sparseEmbed[0].Indices), "len(sparseEmbed[0].Values)", len(sparseEmbed[0].Values))
// 	if len(denseEmbed) != len(sparseEmbed) {
// 		t.Error("Len mismatch of dense embed and sparse embed ... error!")
// 	}
// 	if len(user_query) != len(denseEmbed) {
// 		t.Error("Len mismatch of dense embed/sparse embed with user_query ... error!")
// 	}

// }

func TestGenerateEmbeddings(t *testing.T) {
	// 1. Setup Connection ONCE (simulating a long-running app)
	client, err := NewEmbeddingClient("localhost:50051")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 2. WARMUP (Do not measure this)
	// This forces gRPC handshake + ONNX Runtime allocation
	_, _, _ = client.GenerateEmbeddings([]string{"Warmup request"})
	slog.Info("Warmup complete. Starting benchmark...")

	// 3. The Real Test
	user_query := []string{"Hey there who are you", "I am a high performance agent"}

	start := time.Now()
	denseEmbed, sparseEmbed, err := client.GenerateEmbeddings(user_query)
	slog.Info("attributes: ", "len(denseEmbed)", len(denseEmbed), "len(sparseEmbed)", len(sparseEmbed), "user_query", len(user_query))
	slog.Info("shape stuff: ", "len(denseEmbed)", len(denseEmbed[0].Values), "len(sparseEmbed[0].Indices)", len(sparseEmbed[0].Indices), "len(sparseEmbed[0].Values)", len(sparseEmbed[0].Values))
	duration := time.Since(start) // Store duration immediately

	// 4. Analysis
	if err != nil {
		t.Fatal("Inference failed", err)
	}

	slog.Info("Hot Inference Time", "duration", duration)
	slog.Info("Per Query Latency", "latency", duration/time.Duration(len(user_query)))
}
