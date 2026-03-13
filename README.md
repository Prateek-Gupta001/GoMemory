<div align="center">

<img width="466" height="111" alt="Screenshot 2026-03-05 155143" src="https://github.com/user-attachments/assets/0d8d661e-aaf6-4042-a4ca-503cebb737fd" />

# GoMemory

**A high-performance, open-source memory layer for AI agents.**

*Sub-100ms retrieval. Dual memory architecture. Built to outrun the status quo.*

<br/>

[![Go Version](https://img.shields.io/badge/Go-1.25.3+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![PyPI](https://img.shields.io/pypi/v/gomemory?style=flat-square&label=pip%20install%20gomemory)](https://pypi.org/project/gomemory/)
[![Alpha](https://img.shields.io/badge/status-alpha-orange?style=flat-square)]()

<br/>

[**Docs**](https://prateek-gupta001.github.io/go-memory-docs/)
· [**Quickstart**](https://prateek-gupta001.github.io/go-memory-docs/docs/category/quickstart) · [**API Reference**](https://prateek-gupta001.github.io/go-memory-docs/docs/category/api-reference) · [**Report a Bug**](https://github.com/Prateek-Gupta001/GoMemory/issues)

</div>

---

## What is GoMemory?

Most memory layers for AI agents are slow, bloated, or trade accuracy for speed. GoMemory was built to challenge that.

It uses a **dual memory architecture** — a persistent *core memory profile* and a continuously updated *running memory profile* — backed by Hybrid RAG (dense + sparse embeddings) and an LLM-driven conflict resolution engine. The result: retrieval that is fast by default and accurate by design.

> **Philosophy:** *Slow, high-quality inserts. Fast, dumb reads.*
>
> Do all the hard thinking at write time — so that reads are just a lightweight vector lookup.

---

## Benchmarks

> Tested on a Ryzen 5 CPU (no GPU). Sub-50ms is achievable with GPU-based embedding inference.

| Metric | GoMemory | mem0 |
|---|---|---|
| **Retrieval Latency (p50)** | **< 100ms** | ~500ms+ |
| **Memory Quality Coverage** | ~90–95% | ~99% |
| **Architecture** | Hybrid RAG + LLM Orchestration | Graph DB + RAG |
| **Self-hostable** | ✅ | ✅ |

> If you need 99% memory recall and latency is no concern — use mem0. If you need blazing-fast retrieval at 90–95% quality — use GoMemory.

---

## Core Features

- **⚡ Sub-100ms Retrieval** — The read path is a lightweight Hybrid RAG search over pre-curated memories.
- **🧠 Dual Memory Architecture** — Every user has a *core memory* (stable facts) and a *running memory* (evolving context). Both are built and pruned on every insert.
- **🔄 Continual Memory Updation Protocol** — Contradictory memories are automatically detected and replaced. The LLM reasons over new + existing memories and emits a structured JSON action plan (`INSERT` / `DELETE`).
- **📬 NATS JetStream Backed Ingestion** — Memory jobs are durable. Server restarts don't lose pending jobs — they're replayed from the stream.
- **🗄️ Redis Core Memory Cache** — Core memory is cached in Redis for instant reads without hitting the vector DB.
- **🔌 MCP Server** — LLMs can connect directly to GoMemory via the Model Context Protocol and query both core and running memories as tools. *(In progress)*
- **📊 Full Observability** — OpenTelemetry instrumented, with metrics exported to Prometheus, traces to Jaeger, and dashboards in Grafana. *(In progress)*

---

## Architecture
```
Client Request (/add_memory)
        │
        ▼
  Go HTTP Server  ──── returns reqId immediately (non-blocking)
        │
        ▼
  NATS JetStream  ──── durable message queue
        │
        ▼
  Worker Pool (Go goroutines)
        │
        ├── Python gRPC Embedding Service  ──▶  Dense (BGE) + Sparse (SPLADE)
        │
        ├── Qdrant Hybrid Search  ──▶  Fetch similar existing memories
        │
        └── Gemini Flash 2.5 (Thinking)
                │
                ├── Conflict detection
                ├── JSON-constrained action plan
                └── INSERT new / DELETE stale
                        │
                        ▼
              Qdrant (vectors) + PostgreSQL (metadata) + Redis (core memory cache)
```

### Embedding Models

| Type | Model | Purpose |
|---|---|---|
| Dense | `BAAI/bge-small-en-v1.5` | Semantic similarity |
| Sparse | `prithivida/Splade_PP_en_v1` | Exact keyword matching |

---

## Tech Stack

| Component | Technology |
|---|---|
| Server | Go (`net/http`) |
| Message Queue | NATS JetStream |
| Vector DB | Qdrant |
| Metadata Store | PostgreSQL |
| Cache | Redis |
| Embedding Service | Python gRPC microservice |
| LLM (Curation) | Gemini Flash 2.5 Thinking |
| Observability | OpenTelemetry → Prometheus + Jaeger + Grafana |

---

## Quickstart

> **Full guide:** [Quickstart](https://prateek-gupta001.github.io/go-memory-docs/docs/category/quickstart))

### Prerequisites

- Go `1.25.3+`
- Docker & Docker Compose
- Python `3.8+`
- A [Gemini API Key](https://aistudio.google.com/)

### 1. Clone & Configure
```bash
git clone -b dev https://github.com/Prateek-Gupta001/GoMemory.git
cd GoMemory
```

Create a `.env` file in the project root:
```env
DB_PASSWORD=your_secure_password_here
GEMINI_API_KEY=your_gemini_api_key_here
```

### 2. Start Infrastructure
```bash
docker compose up -d
```

This spins up Postgres, Redis, Qdrant, and NATS JetStream.

### 3. Run the Server
```bash
go mod tidy
make run
```

The server starts at `http://localhost:9000`.

---

### Python SDK
```bash
pip install gomemory
```
```python
from gomemory import GoMemoryClient, Role, Message
from gomemory.exceptions import APIError

client = GoMemoryClient(base_url="http://localhost:9000")

try:
    # Create a user
    user = client.create_user()
    print(f"User ID: {user.user_id}")

    # Add a conversation to memory
    messages = [
        Message(role=Role.USER, content="My name is Mario and my brother is Wario."),
        Message(role=Role.MODEL, content="Got it, I'll remember that.")
    ]
    response = client.add_memory(user_id=user.user_id, messages=messages)
    print(f"Memory job queued. Request ID: {response.req_id}")

except APIError as e:
    print(f"Error {e.status}: {e.message}")
```

→ **[Explore the full API Reference](https://prateek-gupta001.github.io/go-memory-docs/docs/category/api-reference)**

---

## API Overview

### `POST /add_memory`

Non-blocking. Returns a `reqId` immediately while the memory job is processed in the background.
```json
// Request
{
  "userId": "user-123",
  "messages": [
    { "role": "user", "content": "..." },
    { "role": "assistant", "content": "..." }
  ]
}

// Response
{
  "reqId": "job-abc-xyz",
  "msg": "Memory job queued."
}
```

### `POST /retrieve_memory`

Hybrid RAG search over pre-curated memories. Consistently sub-100ms.
```json
// Request
{
  "userId": "user-123",
  "query": "What is the user's brother's name?",
  "reqId": "optional-correlation-id"
}

// Response
[
  {
    "memory_text": "User's brother is named Wario.",
    "memory_id": "mem-456",
    "userId": "user-123"
  }
]
```

> **Tip:** For best results, pass an intent-focused query generated by your LLM rather than raw user input.


## Roadmap

- [x] Hybrid RAG retrieval (dense + sparse)
- [x] Dual memory architecture (core + running)
- [x] Continual memory updation protocol
- [x] NATS JetStream durable ingestion
- [x] Redis core memory cache
- [x] Python SDK (`pip install gomemory`)
- [x] Docs site + landing page
- [x] CI/CD, Makefile, Docker Compose, graceful shutdown
- [ ] MCP Server *(in progress)*
- [ ] Grafana observability dashboard *(in progress)*
- [ ] Concurrent Chunking (Game Changer)
- [ ] Conversational `/retrieve` endpoint (raw chat history input)

---

## Contributing

Contributions, issues, and feature requests are welcome. This is a solo-maintained open source project — any help is genuinely appreciated.

1. Fork the repository
2. Create your branch: `git checkout -b feat/your-feature`
3. Commit your changes: `git commit -m 'feat: add your feature'`
4. Push and open a Pull Request

Please open an issue before starting work on major features.

---

## License

MIT © [Prateek Gupta](https://github.com/Prateek-Gupta001)

---

<div align="center">

Built with stubbornness and a genuine belief that LLM orchestration + Hybrid RAG beats graph DBs for memory.

**[⭐ Star this repo if GoMemory is useful to you](https://github.com/Prateek-Gupta001/GoMemory)**

</div>
