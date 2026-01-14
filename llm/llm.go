package llm

import "github.com/Prateek-Gupta001/GoMemory/types"

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
		ApiKey:    "dummyapikey",
	}
}
func (llm *GeminiLLM) GenerateMemoryText(messages []types.Message, oldMemories []string) ([]string, error) {
	return nil, nil
}

func (llm *GeminiLLM) ExpandQuery(messages []types.Message) (string, error) {

	return "", nil
}
