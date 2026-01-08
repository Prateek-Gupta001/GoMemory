package main

import (
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/api"
	logger "github.com/Prateek-Gupta001/GoMemory/log"
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
		slog.Error("Got this error while trying to create a new postgres store", "error", err)
	}
	server := api.NewMemoryServer(":9000", store)
	if err := server.Run(); err != nil {
		panic(err)
	}
}
