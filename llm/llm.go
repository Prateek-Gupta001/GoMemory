package llm

import (
	"context"
	"log/slog"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"google.golang.org/genai"
)

type LLM interface {
	GenerateMemoryText([]types.Message, []string) ([]string, error)
	ExpandQuery([]types.Message, context.Context) (string, error)
}

type GeminiLLM struct {
	ModelName    string
	GeminiClient *genai.Client
}

func NewGeminiLLM() (*GeminiLLM, error) {
	client, err := genai.NewClient(context.Background(), nil)
	if err != nil {
		slog.Error("Got this error while trying to generate Gemini Client", "error", err)
		return &GeminiLLM{}, err
	}
	return &GeminiLLM{
		ModelName:    "Gemini",
		GeminiClient: client,
	}, nil
}

func (llm *GeminiLLM) GenerateMemoryText(messages []types.Message, oldMemories []string) ([]string, error) {

	return nil, nil
}

func (llm *GeminiLLM) ExpandQuery(messages []types.Message, ctx context.Context) (string, error) {
	history := GetGeminiHistory(messages)
	chat, _ := llm.GeminiClient.Chats.Create(ctx, "gemini-3-flash-preview", nil, history)
	res, err := chat.SendMessage(ctx, genai.Part{Text: ` 
	--- SYSTEM INSTRUCTION ---
	### Role
	You are a Memory Query Generator. Your goal is to generate a search query to check a vector database for EXISTING memories that might conflict with or relate to the conversation history provided to you.

	### Instructions
	1. Analyze the user and the LLM conversation in the context of the history.
	2. Identify if the user is stating a **new fact, preference, or plan**.
	3. If yes, generate a keyword-heavy search query to find *previous* memories about that specific **TOPIC**.
	4. If the user is just chatting (e.g., "Hi", "Help me code"), output "SKIP".

	### Examples
	Assistant: "Do you still live in London?"
	User: "No, I moved to Tokyo last week."
	Query: current residence location home address city

	User: "My dog's name is actually Rover, not Rex."
	Query: pet dog name pets animal companions

	User: "How do I reverse a linked list?"
	Query: SKIP

	### Output
	Return ONLY the query string or "SKIP". Do not add markdown or explanations.
	`})

	if err != nil {
		slog.Error("Got this error while trying to Expand Query", "error", err)
		return "", err
	}

	return res.Text(), nil
}

func GetGeminiHistory(messages []types.Message) []*genai.Content {
	history := make([]*genai.Content, len(messages))
	for _, msg := range messages {
		history = append(history, genai.NewContentFromText(msg.Content, genai.Role(msg.Role)))
	}
	return history
}
