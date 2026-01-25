package main

import (
	"log/slog"
	"os"

	"github.com/Prateek-Gupta001/GoMemory/api"
	"github.com/Prateek-Gupta001/GoMemory/embed"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	logger "github.com/Prateek-Gupta001/GoMemory/log"
	"github.com/Prateek-Gupta001/GoMemory/memory"
	"github.com/Prateek-Gupta001/GoMemory/storage"
	"github.com/Prateek-Gupta001/GoMemory/vectordb"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
)

func main() {
	logger.SetLogger()
	err := godotenv.Load()
	if err != nil {
		slog.Error("got this error while trying to load a dotenv file", "error", err)
	}
	store, err := storage.NewPostgresStore()
	if err != nil {
		panic(err)
	}
	vectordb, err := vectordb.NewQdrantMemoryDB()
	if err != nil {
		slog.Error("Got this error while trying to intialise the vector db", "err", err)

	}
	llm, err := llm.NewGeminiLLM()
	if err != nil {
		slog.Error("Got this error while trying to generate a new geminiLLM client", "error", err)
		os.Exit(1)
	}
	embedClient, err := embed.NewEmbeddingClient("localhost:50051")
	if err != nil {
		slog.Info("Got this error while creating a new embedding client!")
	}
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		slog.Error("can't connect to NATS", "error", err)
	}
	slog.Info("NATS JetStream is up!")
	defer nc.Close()
	memory, err := memory.NewMemoryAgent(vectordb, llm, embedClient, nc, 5000, 2)
	if err != nil {
		slog.Error("Got this error while trying to intialise the new Qdrant Memory DB", "error", err)
	}
	server := api.NewMemoryServer(":9000", store, memory)
	slog.Info("Server is running on port 9000!")
	if err := server.Run(); err != nil {
		panic(err)
	}
}
