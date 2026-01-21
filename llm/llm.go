package llm

import (
	"os"

	"github.com/Prateek-Gupta001/GoMemory/types"
)

type LLM interface {
	GenerateMemoryText([]types.Message, []string) ([]string, error)
	ExpandQuery([]types.Message) (string, error)
}

type GeminiLLM struct {
	ModelName string
	ApiKey    string
}

func NewGeminiLLM() *GeminiLLM {
	return &GeminiLLM{
		ModelName: "Gemini",
		ApiKey:    os.Getenv("GOOGLE_GEMINI_API_KEY"),
	}
}

func (llm *GeminiLLM) GenerateMemoryText(messages []types.Message, oldMemories []string) ([]string, error) {

	return nil, nil
}

func (llm *GeminiLLM) ExpandQuery(messages []types.Message) (string, error) {
	return "", nil
}
