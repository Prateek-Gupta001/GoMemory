package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"os"

	"math/rand"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"google.golang.org/api/googleapi"
	"google.golang.org/genai"
)

type LLM interface {
	GenerateMemoryText([]types.Message, []types.Memory, context.Context) (*types.MemoryOutput, error)
	ExpandQuery([]types.Message, context.Context) (string, error)
}

type GeminiLLM struct {
	ModelName    string
	GeminiClient *genai.Client
}

func NewGeminiLLM() (*GeminiLLM, error) {

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: os.Getenv("gemini_api_key"),
	})
	if err != nil {
		slog.Error("Got this error while trying to generate Gemini Client", "error", err)
		return &GeminiLLM{}, err
	}
	return &GeminiLLM{
		ModelName:    "Gemini",
		GeminiClient: client,
	}, nil
}

type Existing_Memory struct {
	Memory_text string
	MemoryId    string
}

func (llm *GeminiLLM) GenerateMemoryText(messages []types.Message, oldMemories []types.Memory, ctx context.Context) (*types.MemoryOutput, error) {

	var allUserText string
	var prompt string
	var Existing_Memories []Existing_Memory
	for _, msg := range messages {
		if msg.Role == types.RoleUser {
			allUserText += msg.Content
		}
	}
	for _, m := range oldMemories {
		Existing_Memories = append(Existing_Memories, Existing_Memory{
			Memory_text: m.Memory_text,
			MemoryId:    m.Memory_Id,
		})
	}
	bytes, err := json.MarshalIndent(Existing_Memories, "", " ")
	if err != nil {
		slog.Error("Got this error while doing json.MarshalIndent.. and while generating memory text", "err", err)
		return nil, err
	}
	slog.Info("Here are the thing being passed into the prompt", "Exisisting Memories", string(bytes), "UserInput", allUserText)
	//TODO: Simiplify the Exisitng Memories to include simple integer IDs and map them back to their normal unique uuids

	prompt = "<EXISTING_MEMORIES> \n" + string(bytes) + "\n </EXISTING_MEMORIES> \n" + "<USER_INPUT> \n" + allUserText + "\n </USER_INPUT>"
	ptr := true
	responseSchema := &genai.Schema{
		Type:  genai.TypeObject,
		Title: "MemoryArchivistOutput",
		Properties: map[string]*genai.Schema{
			"step_1 reasoning_scratchpad": {
				Type:  genai.TypeString,
				Title: "Reasoning Step-by-Step",
				Description: `CRITICAL: rigorous step-by-step analysis.
			1. Identify new high-fidelity facts.
			2. Search for conflicts in existing memory IDs.
			3. Decide on DELETE/INSERT.
			Do not output JSON until this analysis is complete.
			First output reasoning and thinking and then generate memory_actions`,
			},
			"step_2 memory_actions": {
				Type:  genai.TypeArray,
				Title: "Final Memory Operations",
				Items: &genai.Schema{
					Type:  genai.TypeObject,
					Title: "SingleMemoryAction",
					Properties: map[string]*genai.Schema{
						"action_type": {
							Type:        genai.TypeString,
							Title:       "Operation Type",
							Enum:        []string{"INSERT", "DELETE"},
							Description: "The operation to perform.",
						},
						"payload": {
							Type:        genai.TypeString,
							Title:       "Memory Content",
							Description: "Required for INSERT. The high-fidelity, context-rich memory text. Leave empty/null if action_type is DELETE.",
							Nullable:    &ptr, // Explicitly allow null
						},
						"target_memory_id": {
							Type:        genai.TypeString,
							Title:       "Target ID",
							Description: "Required for DELETE. The exact ID of the existing memory to remove. Leave empty/null if action_type is INSERT.",
							Nullable:    &ptr, // Explicitly allow null
						},
					},
					Required: []string{"action_type"},
				},
			},
		},
		// CRITICAL: The order in this slice dictates the generation order
		Required: []string{"step_1 reasoning_scratchpad", "step_2 memory_actions"},
	}
	//TODO: Think of whether we should just pass allUserText type thing in ExpandQuery function as well ..
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(`### ROLE
You are the **Memory Archivist**, an advanced neural interface responsible for maintaining a high-fidelity, long-term database of user facts. 

### OBJECTIVE
Your goal is to parse new User Input, compare it against a provided list of Existing Memories, and output a list of JSON actions (*INSERT* or *DELETE*) to update the database. 

### CRITICAL STANDARDS FOR MEMORY CREATION
1.  **High Fidelity Only:** Never store vague statements. 
    * *BAD:* "User likes coding."
    * *GOOD:* "User is a Golang backend engineer utilizing Qdrant for vector search."
2.  **Context Preservation:** Memories must be standalone facts. If the user says "I hate it," and "it" refers to "Java," store: "User has a strong distaste for the Java programming language," not "User hates it."
3.  **Selectivity:** Do not archive trivial chitchat (e.g., "Hello," "How are you?"). Only archive permanent facts, preferences, goals, or biographical data.

### LOGIC FLOW (The Decision Tree)
For every distinct fact found in the User Input, perform this check against *Existing_Memories*:

**Case 1: No Conflict, New Information**
* *Condition:* The fact is new and does not overlap with any existing memory ID.
* *Action:* *INSERT* the new rich text.

**Case 2: Direct Contradiction (Correction)**
* *Condition:* New input contradicts an existing memory (e.g., Old: "User lives in Ohio", New: "I moved to Paris").
* *Action:* *DELETE* the old *memory_id* AND *INSERT* the new fact.

**Case 3: Consolidation (Refinement)**
* *Condition:* New input adds specific detail to a vague existing memory (e.g., Old: "User owns a dog", New: "My dog is a Golden Retriever named Max").
* *Action:* *DELETE* the old *memory_id* AND *INSERT* the merged, high-fidelity memory ("User owns a Golden Retriever named Max").

### INPUT FORMAT
You will receive:
1.  *Existing_Memories*: A list of objects containing { "id": "...", "text": "..." }.
2.  *User_Input*: The raw text string from the current conversation turn.

### OUTPUT INSTRUCTIONS
1.  **Reasoning Scratchpad:** You MUST begin by writing a "reasoning_scratchpad". List the facts extracted, identify specific memory_id's that conflict, and justify your decision to Delete or Insert.
2.  **Strict JSON:** Output the final result as a JSON object adhering to the schema. 
3.  **ID Integrity:** When using *DELETE*, you MUST use the exact *memory_id* provided in *Existing_Memories*. Never invent an ID.
`, genai.RoleUser),
		ResponseMIMEType: "application/json",
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel: "medium",
		},
		ResponseJsonSchema: responseSchema,
	}

	var result *genai.GenerateContentResponse
	for i := 0; i < 5; i++ {
		result, err = llm.GeminiClient.Models.GenerateContent(
			ctx,
			"gemini-3-flash-preview",
			genai.Text(prompt),
			config)
		if err == nil {
			break
		}
		if !RetryAbleError(err) {
			return nil, err
		}
		backoff := time.Duration(1<<i) * time.Second
		jitter := time.Duration(rand.Int63n(int64(backoff)/5*2) - int64(backoff)/5)
		retryDuration := backoff + jitter
		slog.Error("Got this error while generating memory text.. in the llm call. Retrying after some time", "error", err, "time", retryDuration)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDuration):
		}
		slog.Info("Retrying!")
	}

	if err != nil {
		slog.Error("Got this error while generating memory text.. in the llm call.", "error", err)
		return nil, err
	}
	fmt.Println("result text: ", result.Text())
	memoryOutput := &types.MemoryOutput{}
	if err := json.NewDecoder(strings.NewReader(result.Text())).Decode(memoryOutput); err != nil {
		slog.Error("Got malformed JSON output from the LLM", "error", err)
		return nil, err
	}

	return memoryOutput, nil
}

func (llm *GeminiLLM) ExpandQuery(messages []types.Message, ctx context.Context) (string, error) {
	history := GetGeminiHistory(messages)
	chat, _ := llm.GeminiClient.Chats.Create(ctx, "gemini-3-flash-preview", nil, history)
	expandQueryPrompt := ` 
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
	`

	//TODO: Test this properly .... can cause error maybe ....
	var res *genai.GenerateContentResponse
	var err error
	for i := 0; i < 5; i++ {
		res, err = chat.SendMessage(ctx, genai.Part{Text: expandQueryPrompt})

		if err == nil {
			break
		}
		backoff := time.Duration(1<<i) * time.Second
		jitter := time.Duration(rand.Int63n(int64(backoff)/5*2) - int64(backoff)/5)
		retryDuration := backoff + jitter
		slog.Error("Got this error while generating memory text.. in the llm call. Retrying after some time", "error", err, "time", retryDuration)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(retryDuration):
		}
		slog.Info("Retrying!")
	}

	if err != nil {
		slog.Error("Got this error while trying to Expand Query", "error", err)
		return "", err
	}

	return res.Text(), nil
}

func RetryAbleError(err error) bool {
	if gErr, ok := err.(*googleapi.Error); ok {
		slog.Info("Retryable Error", "errcode", gErr.Code)
		if gErr.Code >= 500 {
			return true
		}
		if gErr.Code == 429 {
			return true
		}
		return false
	}
	return false
}

func GetGeminiHistory(messages []types.Message) []*genai.Content {
	history := make([]*genai.Content, 0, len(messages))
	for _, msg := range messages {
		content := genai.NewContentFromText(msg.Content, genai.Role(msg.Role))
		history = append(history, content)
	}
	return history
}
