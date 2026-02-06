package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"math/rand"

	"github.com/Prateek-Gupta001/GoMemory/types"
	"github.com/joho/godotenv"

	// "github.com/google/uuid"
	"google.golang.org/api/googleapi"
	"google.golang.org/genai"
)

type LLM interface {
	GenerateMemoryText(messages []types.Message, coreMemories []types.Memory, oldMemories []types.Memory, ctx context.Context) (*types.MemoryOutput, error)
	ExpandQuery([]types.Message, context.Context) string
}

type GeminiLLM struct {
	ModelName    string
	GeminiClient *genai.Client
}

func NewGeminiLLM() (*GeminiLLM, error) {
	err := godotenv.Load()
	if err != nil {
		slog.Error("got this error while trying to load a dotenv file", "error", err)
	}

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
	Memory_text string           `json:"text"`
	Type        types.MemoryType `json:"type"`
	MemoryId    string           `json:"id"`
}

func (llm *GeminiLLM) GenerateMemoryText(messages []types.Message, coreMemories []types.Memory, oldMemories []types.Memory, ctx context.Context) (*types.MemoryOutput, error) {

	var allUserText string
	var prompt string
	var Existing_Memories_old []Existing_Memory
	var Existing_Memories_core []Existing_Memory
	var sb strings.Builder
	for _, msg := range messages {
		//TODO: Think about whether you actually want the allUserText or just the latest turn .. since the memories for the previous turns would
		//TODO: already have been stored by won't show up in existing memories .. or the dev should only do this .. after a fix no. of turns
		//TODO: something like that .. think about that in the docs.
		if msg.Role == types.RoleUser {
			sb.WriteString(msg.Content)
			sb.WriteString("\n")
		}
	}
	allUserText = sb.String()
	type DoubleMap struct {
		UUIDtoInt map[string]string
		IntTOUUID map[string]string
	}
	UUIDtoInt := make(map[string]string)
	UIntTOUUID := make(map[string]string)
	dMap := DoubleMap{
		UUIDtoInt: UUIDtoInt,
		IntTOUUID: UIntTOUUID,
	}

	for idx, m := range coreMemories {
		dMap.UUIDtoInt[m.Memory_Id] = strconv.Itoa(idx)
		dMap.IntTOUUID[strconv.Itoa(idx)] = m.Memory_Id
		Existing_Memories_core = append(Existing_Memories_core, Existing_Memory{
			Memory_text: m.Memory_text,
			Type:        m.Type,
			MemoryId:    dMap.UUIDtoInt[m.Memory_Id],
		})
	}
	for idx, m := range oldMemories {
		displacedId := idx + len(coreMemories)
		dMap.UUIDtoInt[m.Memory_Id] = strconv.Itoa(displacedId)
		dMap.IntTOUUID[strconv.Itoa(len(coreMemories))] = m.Memory_Id
		Existing_Memories_old = append(Existing_Memories_old, Existing_Memory{
			Memory_text: m.Memory_text,
			Type:        m.Type,
			MemoryId:    dMap.UUIDtoInt[m.Memory_Id],
		})
	}

	slog.Info("Here is the mapping here", "map", dMap)
	OldMemorybytes, err := json.MarshalIndent(Existing_Memories_old, "", " ")
	if err != nil {
		slog.Error("Got this error while doing json.MarshalIndent.. and while generating memory text", "err", err)
		return nil, err
	}
	coreMemoryBytes, err := json.MarshalIndent(Existing_Memories_core, "", " ")
	if err != nil {
		slog.Error("Got this error while doing json.MarshalIndent.. and while generating memory text", "err", err)
		return nil, err
	}

	slog.Info("Here are the thing being passed into the prompt", "Existing Old Memories", string(OldMemorybytes), "Existing Core Memories", string(coreMemoryBytes), "UserInput", allUserText)
	//TODO: Simiplify the Exisitng Memories to include simple integer IDs and map them back to their normal unique uuids

	prompt = "<EXISTING_CORE_MEMORIES> \n" + string(coreMemoryBytes) + "\n </EXISTING_CORE_MEMORIES> \n" + "<EXISTING_OLD_MEMORIES> \n" + string(OldMemorybytes) + "\n </EXISTING_OLD_MEMORIES> \n" + "<USER_INPUT> \n" + allUserText + "\n </USER_INPUT>"
	// Action Schema (Same as before)
	ptr := true

	memoryActionItemSchema := &genai.Schema{
		Type:  genai.TypeObject,
		Title: "SingleMemoryAction",
		Properties: map[string]*genai.Schema{
			"action_type": {
				Type: genai.TypeString,
				Enum: []string{"INSERT", "DELETE"},
			},
			"payload": {
				Type:     genai.TypeString,
				Nullable: &ptr,
			},
			"target_memory_id": {
				Type:        genai.TypeString,
				Nullable:    &ptr,
				Description: "The Integer ID of the memory to delete.",
			},
		},
		Required: []string{"action_type"},
	}

	responseSchema := &genai.Schema{
		Type:  genai.TypeObject,
		Title: "MemoryArchivistOutput",
		Properties: map[string]*genai.Schema{
			"step_1 critical_reasoning": {
				Type:  genai.TypeString,
				Title: "The Analyst Workbench",
				Description: `CRITICAL: You must output your thought process here BEFORE generating actions.
            Follow this EXACT structure in your text:
            1. [INPUT ANALYSIS] List distinct facts found in User Input. Think about what new facts have been mentioned by the user.
            2. [CORE SCAN] Check 'Existing_Core_Memories' for conflicts.
               - IF Conflict Found: Write "CONFLICT: Core ID [X] says '...' vs New Input '...' -> Must DELETE [X] and INSERT new."
            3. [GENERAL SCAN] Check 'Existing_General_Memories' for conflicts.
               - IF Conflict Found: Write "CONFLICT: Gen ID [Y] says '...' vs New Input '...' -> Must DELETE [Y] and INSERT new."
            4. [DUPLICATION CHECK] verify that the new fact doesn't already exist perfectly.
            `,
			},
			"step_2 core_memory_actions": {
				Type:  genai.TypeArray,
				Title: "Core Actions",
				Items: memoryActionItemSchema,
			},
			"step_3 general_memory_actions": {
				Type:  genai.TypeArray,
				Title: "General Actions",
				Items: memoryActionItemSchema,
			},
		},
		Required: []string{"step_1 critical_reasoning", "step_2 core_memory_actions", "step_3 general_memory_actions"},
	}
	//TODO: Think of whether we should just pass allUserText type thing in ExpandQuery function as well ..
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(`### ROLE
You are the **Memory Archivist**. You are a Ruthless Database Administrator. Your goal is to maintain a pristine, high-signal database.

### THE "GATEKEEPER" PROTOCOL (Spam Filter)
**90% of user conversation is NOT memory-worthy. Do not archive temporary context.**
You must aggressively filter out noise. Only store permanent truths.

* **TRASH (Do Not Save):**
    * *Chitchat:* "Hello", "How are you?", "Thanks".
    * *Temporary States:* "I'm hungry", "I'm going to sleep", "I'm driving right now".
    * *Task Context:* "Here is my code, fix it", "Write a poem about dogs". (The user asking for code is not a memory; the user *liking* Python is).
    * *Vague Statements:* "That's cool", "I like that".

* **TREASURE (Save Immediately):**
    * *Biographical:* "I am 25", "I live in Berlin".
    * *Preferences:* "I prefer short answers", "I hate Java".
    * *Projects:* "I am building a RAG app with Qdrant".

### THE "PREDATORY PRUNING" PROTOCOL
**Passive recording is insufficient. You must ACTIVELY HUNT for obsolete data.**
Your default mode is "Search and Destroy." Treat every new piece of information as a weapon to eliminate outdated facts.

**EXECUTION RULES:**
1.  **Assume Conflict:** For every single fact you extract, *assume* a contradiction already exists in the database. Your job is to find it.
2.  **Stalk the Target:** If a new fact updates a user's status (e.g., "Student" -> "Employed"), you MUST issue a DELETE command for the old ID.
3.  **Zero Tolerance:** Allowing two conflicting versions of the truth to coexist is a CRITICAL SYSTEM FAILURE.

### MEMORY CLASSIFICATION
**TIER 1: CORE MEMORIES (Identity & Existence)**
* **Scope:** Name, Age, Gender, Location, Profession, Global AI Instructions.
* **Action:** If these change, the old memory MUST be deleted immediately.

**TIER 2: GENERAL MEMORIES (Biographical & Tastes)**
* **Scope:** Projects, specific tech stack skills, pets, likes/dislikes.
* **Action:** Refine vague memories into specific ones.

### INPUT DATA
1.  *Existing_Core_Memories*: List of { "id": "1", "text": "..." }
2.  *Existing_General_Memories*: List of { "id": "101", "text": "..." }
3.  *User_Input*: The new text to process.

### PROCESS (The Analyst Workbench)
In your "step_1_critical_reasoning" field:
1.  **Filter:** explicitly state what you are IGNORING (e.g., "Ignored 'Hi' as chitchat").
2.  **Extract:** List the actual memory-worthy facts.
3.  **Target:** Identify specific IDs to DELETE.
4.  **Decide:** List the Kill List and the New Entries.

### ID HANDLING
* **CRITICAL:** When generating a DELETE action, you MUST output the exact Integer ID as a string (e.g., "42").

### ONE-SHOT DEMONSTRATION
**Input:**
* Existing_Core_Memories: [{"id": "1", "text": "User lives in Berlin"}, {"id": "2", "text": "User is a Student"}]
* Existing_General_Memories: [{"id": "55", "text": "User is learning Python basics"}]
* User_Input: "Hey Gemini! I actually just finished my degree and moved to London! Can you help me write a Go script for my new job? I've stopped using Python btw."

**Correct Output:**
{
  "step_1_critical_reasoning": "1. FILTERING: Ignored 'Hey Gemini' (Chitchat). Ignored 'Can you help me write a Go script' (Task Context). \n2. FACTS: User graduated (Student -> Worker), Moved (Berlin -> London), Tech Switch (Drop Python, Add Go). \n3. PREDATORY SCAN: Target ID '1' (Berlin) -> DELETE. Target ID '2' (Student) -> DELETE. Target ID '55' (Python) -> DELETE.",
  "step_2_core_memory_actions": [
    { "action_type": "DELETE", "target_memory_id": "1", "payload": null },
    { "action_type": "INSERT", "payload": "User lives in London, UK.", "target_memory_id": null },
    { "action_type": "DELETE", "target_memory_id": "2", "payload": null },
    { "action_type": "INSERT", "payload": "User is a working professional (Graduated).", "target_memory_id": null }
  ],
  "step_3_general_memory_actions": [
    { "action_type": "DELETE", "target_memory_id": "55", "payload": null },
    { "action_type": "INSERT", "payload": "User codes primarily in Golang and has stopped using Python.", "target_memory_id": null }
  ]
}
`, genai.RoleUser),
		ResponseMIMEType: "application/json",
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel:   "high",
			IncludeThoughts: false,
		},
		ResponseJsonSchema: responseSchema,
	}

	var result *genai.GenerateContentResponse
	for i := 0; i < 5; i++ {
		slog.Info("In the for loop for response generation!")
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
	for idx, _ := range memoryOutput.CoreMemoryActions {
		if memoryOutput.CoreMemoryActions[idx].ActionType == "DELETE" {
			slog.Info("Have Delete")
			if memoryOutput.CoreMemoryActions[idx].TargetMemoryID != nil {
				slog.Info("Have id", "id", *memoryOutput.CoreMemoryActions[idx].TargetMemoryID)
				id := *memoryOutput.CoreMemoryActions[idx].TargetMemoryID
				uuid := dMap.IntTOUUID[id]
				memoryOutput.CoreMemoryActions[idx].TargetMemoryID = &uuid
				slog.Info("Swapping it for this id", "new_id", *memoryOutput.CoreMemoryActions[idx].TargetMemoryID)
			}
		}
	}
	for idx, _ := range memoryOutput.GeneralMemoryActions {
		if memoryOutput.GeneralMemoryActions[idx].ActionType == "DELETE" {
			slog.Info("Have Delete")
			if memoryOutput.GeneralMemoryActions[idx].TargetMemoryID != nil {
				slog.Info("Have id", "id", *memoryOutput.GeneralMemoryActions[idx].TargetMemoryID)
				id := *memoryOutput.GeneralMemoryActions[idx].TargetMemoryID
				uuid := dMap.IntTOUUID[id]
				memoryOutput.GeneralMemoryActions[idx].TargetMemoryID = &uuid
				slog.Info("Swapping it for this id", "new_id", *memoryOutput.GeneralMemoryActions[idx].TargetMemoryID)
			}
		}
	}
	return memoryOutput, nil
}

func (llm *GeminiLLM) ExpandQuery(messages []types.Message, ctx context.Context) string {
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
			return ""
		case <-time.After(retryDuration):
		}
		slog.Info("Retrying!")
	}

	if err != nil {
		slog.Error("Got this error while trying to Expand Query Falling back to messages based query", "error", err)
		var sb strings.Builder
		for i := len(messages) - 1; i >= 0; i-- {
			msgContent := messages[i].Content
			sb.WriteString(msgContent)
			sb.WriteString("\n")
			if sb.Len()+len(msgContent) > 2000 {
				break
			}
		}
		expandedQuery := sb.String()
		return expandedQuery
	}

	return res.Text()
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
