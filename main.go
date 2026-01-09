package main

import (
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/api"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	logger "github.com/Prateek-Gupta001/GoMemory/log"
	"github.com/Prateek-Gupta001/GoMemory/memory"
	"github.com/Prateek-Gupta001/GoMemory/storage"
	"github.com/joho/godotenv"
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
	llm := llm.NewGeminiLLM()
	memory, err := memory.NewQdrantMemoryDB()
	if err != nil {
		slog.Error("Got this error while trying to intialise the new Qdrant Memory DB", "error", err)
	}
	server := api.NewMemoryServer(":9000", store, llm, memory)
	slog.Info("Server is running on port 9000!")
	if err := server.Run(); err != nil {
		panic(err)
	}
}
