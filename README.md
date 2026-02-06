# GoMemory

<img width="638" height="322" alt="image" src="https://github.com/user-attachments/assets/771a68c7-6c0d-46b0-bbde-4179d3656510" />

**A High-Performance, Low-Latency Memory Layer for AI Agents.**
*Think of it as the RAM for your AI applications—optimized for factual accuracy and sub-100ms retrieval.*

---

## 🚀 The Core Idea

Solving memory for AI is a massive engineering challenge. **GoMemory** is my aim at cracking it.

The architecture is built on a specific philosophy: **Slow, high-quality inserts; fast, "dumb" reads.**

* **Ingestion (Write)**: Memory insertion is treated as an expensive, background operation. We prioritize consistency, conflict resolution, and factual accuracy over write-speed.
* **Retrieval (Read)**: Memory retrieval is optimized for raw speed. By pre-processing and curating memories during the write phase, the read path becomes extremely lightweight, consistently clocking **sub-150ms** (often <100ms) latencies.

## 🧠 The Dual-Layer Memory System

We don't treat all memories equally. GoMemory uses a **Tiered Architecture** to handle different types of data with different latency requirements.

### 1. Core Memories (Identity Layer)
* **Storage:** **Redis** (In-Memory Key-Value Store).
* **Latency:** Microseconds.
* **Purpose:** Stores the "Soul" of the user—unchanging or slowly changing facts like **Name, Age, Location, Profession, Core Preferences, and Native Language**.
* **Retrieval Logic:** Core memories are **always** retrieved and provided to the LLM on every single request. They act as the "System Prompt" extension, ensuring the AI never forgets who it is talking to.
    * *Example:* "User lives in Paris", "User is a Backend Engineer".

### 2. General Memories (Context Layer)
* **Storage:** **Qdrant** (Vector Database).
* **Latency:** <100ms.
* **Purpose:** Stores the "Life History" of the user—past conversations, specific project details, fast-changing context, and biographical nuances.
* **Retrieval Logic:** These are retrieved dynamically via **Hybrid RAG** based on the relevance to the current user query.

---

## 🛠️ Tech Stack & Architecture

This project is built to demonstrate robust **Backend Engineering** principles, utilizing industry-standard tools for reliability and scale.

* **Language**: **Golang** (using `net/http` for a lightweight, dependency-free server core).
* **Messaging Queue**: **NATS Jetstream**.
    * *Why?* Memory jobs are critical. I chose NATS Jetstream to ensure persistence and reliability. If the server goes down, the memory jobs are replayed upon recovery.
* **Core Storage**: **Redis** (for Core Memories).
* **Vector Database**: **Qdrant** (for General Memories).
* **Metadata Storage**: **PostgreSQL** (for user metadata).
* **Observability**: Designed for reliability with error handling and structured logging (`slog`).

## 🧠 Hybrid RAG & AI Pipeline

To provide the highest quality memory retrieval, GoMemory uses a **Hybrid RAG** approach, combining dense and sparse embeddings.

### Embeddings
* **Dense**: `BAAI/bge-small-en-v1.5` (captures semantic meaning).
* **Sparse**: `prithivida/Splade_PP_en_v1` (captures exact keyword matches).

### The "Thinking" Engine (Gemini Flash 3)
We utilize **Gemini Flash 3 (Medium Thinking)** for the heavy lifting of memory curation.
* **Query Expansion**: The LLM expands incoming user queries to find relevant past contexts.
* **"Predatory Pruning" Protocol**:
    * When a new memory is added, the system fetches similar existing memories.
    * The LLM acts as a **Ruthless Archivist**: it actively hunts for contradictions.
    * **Logic:** If the user moves from "Italy" to "Paris", the system issues a `DELETE` for the old location ID (in Qdrant/Redis) and an `INSERT` for the new fact.
    * **Promotion:** If a general fact becomes defining (e.g., "I am now a Doctor"), it can be promoted from General Memory to Core Memory.

## ⚡ Workflow

1.  **Request**: User sends a `/add_memory` request.
2.  **Ack**: Server returns a `reqId` immediately (Non-blocking).
3.  **Queue**: The job is published to **NATS Jetstream**.
4.  **Worker Pool**: Background Go workers subscribe to the stream and pick up the job.
5.  **Processing**:
    * Expands query.
    * Fetches existing memories (Core via Redis, General via Qdrant).
    * LLM reasons about conflicts, duplicates, and core vs. general classification.
6.  **Commit**: Updates Qdrant (Vectors) and Redis (Core) based on the LLM's decision.

---

## 🔌 API Reference

### 1. Memory Insertion (`/add_memory`)
*We take care of generating new memories, clearing out the old ones, and updating Core/General stores.*

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

*Returns a composite view of the user: All Core Memories + Relevant General Memories.*

**Recommended Approach (Tool Use):** The LLM calls this tool with a specific, intent-heavy query.

**Request:**

```go
type MemoryRetrievalRequest struct {
    UserId    string `json:"userId"`
    UserQuery string `json:"query,omitempty"` // Recommended: LLM generated intent query
    ReqId     string `json:"reqId"`
}

```

**Response:**

```go

[]Memory

type Memory struct {
    Memory_text string `json:"memory_text"`
    Memory_Id   string `json:"memory_id"`
    Type        string `json:"type"` // "core" or "general"
    UserId      string `json:"userId"`
}

```

## 🔮 Roadmap & Future

This project is currently in the Alpha stage, but the core infrastructure is stable and performing well against benchmarks.

* [ ] **SDKs**: Native Node.js and Python SDKs to make integration seamless.
* [ ] **Documentation**: Full landing page and API docs.
* [ ] **Optimization**: Refine the conversational raw chat endpoints.


