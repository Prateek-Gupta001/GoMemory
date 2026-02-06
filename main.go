package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Prateek-Gupta001/GoMemory/api"
	"github.com/Prateek-Gupta001/GoMemory/embed"
	"github.com/Prateek-Gupta001/GoMemory/llm"
	logger "github.com/Prateek-Gupta001/GoMemory/log"
	"github.com/Prateek-Gupta001/GoMemory/memory"
	"github.com/Prateek-Gupta001/GoMemory/redis"
	"github.com/Prateek-Gupta001/GoMemory/storage"
	"github.com/Prateek-Gupta001/GoMemory/vectordb"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
)

func PrintBanner() {
	// ANSI Color Codes for that "Futuristic Terminal" look
	const (
		Reset   = "\033[0m"
		Cyan    = "\033[36m"
		Green   = "\033[32m"
		Magenta = "\033[35m"
		Bold    = "\033[1m"
	)

	banner := `
   ______      __  ___                          
  / ____/___  /  |/  /___  ____ ___  ____  _____ __  __
 / / __/ __ \/ /|_/ / __ \/ __ ` + "`" + `__ \/ __ \/ ___/ / / /
/ /_/ / /_/ / /  / /  __/ / / / / / /_/ / /   / /_/ / 
\____/\____/_/  /_/\___/_/ /_/ /_/\____/_/    \__, /  
                                             /____/   
`
	fmt.Println(Cyan + Bold + banner + Reset)
	fmt.Printf("%s:: GoMemory Cortex v1.0 ::%s %sThe future of memory for AI Agents is here%s\n\n", Magenta, Reset, Bold, Reset)

	// The "System Check" sequence
	fmt.Println(Green + " ✓ " + Reset + "Gatekeeper Protocol........ " + Green + "ONLINE" + Reset)
	fmt.Println(Green + " ✓ " + Reset + "Predatory Pruning.......... " + Green + "ACTIVE" + Reset)
	fmt.Println(Green + " ✓ " + Reset + "Vector Uplink.............. " + Green + "ESTABLISHED" + Reset)
	fmt.Println(Green + " ✓ " + Reset + "Recall Systems............. " + Green + "READY" + Reset)
	fmt.Println("") // Spacer
}

func main() {
	PrintBanner()
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
		panic(err)
	}

	js, err := nc.JetStream()
	if err != nil {
		panic(err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "MEMORY_SYSTEM",
		Subjects: []string{"memory_work"},
		// Storage:  nats.FileStorage,     //For production, uncomment this line! This will make our stuff persist in file and
	}) //and allow us to not loose our memory jobs!
	if err != nil {
		panic(err)
	}

	defer nc.Close()
	RC := redis.NewRedisCoreMemoryCache()
	memory, err := memory.NewMemoryAgent(vectordb, llm, embedClient, js, RC, 5000, 2)
	if err != nil {
		slog.Error("Got this error while trying to intialise the new Qdrant Memory DB", "error", err)
	}
	server := api.NewMemoryServer(":9000", store, memory)
	if err := server.Run(); err != nil {
		panic(err)
	}
}
