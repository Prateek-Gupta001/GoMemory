# GoMemory

**A High-Performance, Low-Latency Memory Layer for AI Agents.**
*Think of it as the RAM for your AI applications—optimized for factual accuracy and sub-100ms retrieval.*

---

## 🚀 The Core Idea

Solving memory for AI is a massive engineering challenge. **GoMemory** is my aim at cracking it.

The architecture is built on a specific philosophy: **Slow, high-quality inserts; fast, "dumb" reads.**

* **Ingestion (Write)**: Memory insertion is treated as an expensive, background operation. We prioritize consistency, conflict resolution, and factual accuracy over write-speed.
* **Retrieval (Read)**: Memory retrieval is optimized for raw speed. By pre-processing and curating memories during the write phase, the read path becomes extremely lightweight, consistently clocking **sub-150ms** (often <100ms) latencies.

## 🛠️ Tech Stack & Architecture

This project is built to demonstrate robust **Backend Engineering** principles, utilizing industry-standard tools for reliability and scale.

* **Language**: **Golang** (using `net/http` for a lightweight, dependency-free server core).
* **Messaging Queue**: **NATS Jetstream**.
    * *Why?* Memory jobs are critical. I chose NATS Jetstream to ensure persistence and reliability. If the server goes down or power fails, the memory jobs aren't lost—they are replayed once the system recovers.
* **Vector Database**: **Qdrant**.
* **Metadata Storage**: **PostgreSQL** (for user metadata).
* **Observability**: Designed for reliability with error handling and structured logging.

## 🧠 Hybrid RAG & AI Pipeline

To provide the highest quality memory retrieval, GoMemory uses a **Hybrid RAG** approach, combining dense and sparse embeddings.

### Embeddings
* **Dense**: `BAAI/bge-small-en-v1.5` (captures semantic meaning).
* **Sparse**: `prithivida/Splade_PP_en_v1` (captures exact keyword matches).

### The "Thinking" Engine
We utilize **Gemini Flash 3 (Medium Thinking)** for the heavy lifting of memory curation.
* **Query Expansion**: The LLM expands incoming user queries to find relevant past contexts.
* **Memory Pruning & Conflict Resolution**:
    * When a new memory is added, the system fetches similar existing memories.
    * The LLM analyzes the new info against the old data.
    * It generates a **JSON-constrained output** aimed at truthfulness. If a conflict exists (e.g., user changed their preference), the system issues a `DELETE` for the old memory ID and an `INSERT` for the new fact.

## ⚡ Workflow

1.  **Request**: User sends a `/add_memory` request.
2.  **Ack**: Server returns a `reqId` immediately (Non-blocking).
3.  **Queue**: The job is published to **NATS Jetstream**.
4.  **Worker Pool**: Background Go workers subscribe to the stream and pick up the job.
5.  **Processing**:
    * Expands query.
    * Fetches existing memories (Hybrid Search).
    * LLM reasons about conflicts and generates actions.
6.  **Commit**: Updates Qdrant and Postgres based on the LLM's decision.

---

## 🔌 API Reference

### 1. Memory Insertion (`/add_memory`)
*We take care of generating new memories and clearing out the old ones so you can focus on the chat experience.*

**Request:**
```go
type InsertMemoryRequest struct {
    UserId   string    `json:"userId"`
    Messages []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"` // e.g., "user", "assistant"
    Content string `json:"content"`
}
```

**Response:**
```go
type MemoryInsertionResponse struct {
    ReqId string `json:"reqId"`
    Msg   string `json:"msg"`
}
```

### 2. Memory Retrieval
Currently achieves sub-100ms speeds.

**Recommended Approach (Tool Use):** The LLM calls this tool with a specific, intent-heavy query. We run this against Qdrant using Hybrid RAG.

**Request:**
```go
type MemoryRetrievalRequest struct {
    UserId    string `json:"userId"`
    UserQuery string `json:"query,omitempty"` // Recommended: LLM generated intent query
    ReqId     string `json:"reqId"`
}
```

**Alternative Approach (Conversational):** Pass the raw chat history. The system will transform the last user message into a query. (Note: This endpoint is currently in active development).

**Response:**
```go
type Memory struct {
    Memory_text string `json:"memory_text"`
    Memory_Id   string `json:"memory_id"`
    UserId      string `json:"userId"`
}
```

## 🔮 Roadmap & Future
This project is currently in the Alpha stage, but the core infrastructure is stable and performing well against benchmarks.

- [ ] **SDKs**: Native Node.js and Python SDKs to make integration seamless.
- [ ] **Documentation**: Full landing page and API docs.
- [ ] **Optimization**: The raw chats endpoints
