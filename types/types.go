package types

import "errors"

type MemoryRetrievalRequest struct {
	UserId    string    `json:"userId"`
	Messages  []Message `json:"messages,omitempty"`
	UserQuery string    `json:"query,omitempty"`
	Threshold float32   `json:"threshold,omitempty"`
	ReqId     string
}

type InsertMemoryRequest struct {
	UserId   string    `json:"userId"`
	Messages []Message `json:"messages"`
}

type DeleteMemoryRequest struct {
	UserId    string   `json:"userId"`
	MemoryIds []string `json:"memoryId"`
}

const UserIdKey ctxKey = iota

var ErrUserNotFound = errors.New("user doesn't exist")

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
	ReqId     string
	UserId    string
	Messages  []Message
	Threshold float32
}

type DenseEmbedding struct {
	Values []float32 `json:"values"`
}

type SparseEmbedding struct {
	Indices []uint32
	Values  []float32
}
type MemoryType string

const (
	MemoryTypeCore    MemoryType = "core"
	MemoryTypeGeneral MemoryType = "general"
)

type Memory struct {
	Memory_text string
	Type        MemoryType
	Memory_Id   string
	UserId      string
}

type MemoryOutput struct {
	Reasoning            string         `json:"step_1 critical_reasoning"`
	CoreMemoryActions    []MemoryAction `json:"step_2 core_memory_actions"`
	GeneralMemoryActions []MemoryAction `json:"step_3 general_memory_actions"`
}

type MemoryAction struct {
	ActionType     string  `json:"action_type"`
	Payload        *string `json:"payload"`
	TargetMemoryID *string `json:"target_memory_id"`
}
