package types

type MemoryRetrievalRequest struct {
	UserId    string    `json:"userId"`
	Messages  []Message `json:"messages,omitempty"`
	UserQuery string    `json:"query,omitempty"`
	ReqId     string
}

type InsertMemoryRequest struct {
	UserId   string    `json:"userId"`
	Messages []Message `json:"messages"`
}

const UserIdKey ctxKey = iota

type ctxKey int

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "model"
	RoleSystem    Role = "system"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type MemoryInsertionJob struct {
	ReqId    string
	UserId   string
	Messages []Message
}

type DenseEmbedding struct {
	Values []float32 `json:"values"`
}

type SparseEmbedding struct {
	Indices []uint32
	Values  []float32
}

type Memory struct {
	Memory_text string
	Memory_Id   string
	UserId      string
}

type MemoryOutput struct {
	Reasoning string         `json:"step_1 reasoning_scratchpad"`
	Actions   []MemoryAction `json:"step_2 memory_actions"`
}

type MemoryAction struct {
	ActionType     string  `json:"action_type"`
	Payload        *string `json:"payload"`
	TargetMemoryID *string `json:"target_memory_id"`
}
