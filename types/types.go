package types

type RequestStruct struct {
	UserId    string    `json:"userId"`
	Messages  []Message `json:"messages,omitempty"`
	UserQuery string    `json:"query,omitempty"`
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
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type Embedding []float32
