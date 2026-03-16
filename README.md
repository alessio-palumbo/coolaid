# ai-cli

`ai-cli` is a local AI-powered CLI tool for exploring and querying codebases. It supports semantic search, code explanations, summarization, and general LLM queries — all running locally with your own embeddings and vector store.

---

## Features

- **Index your code** with semantic chunking
  - Go functions and methods are chunked via AST
  - Generic text chunking for other files
  - Tracks file paths and line numbers
- **Vector store** backed by SQLite for fast semantic search
- **LLM integration** (via Ollama client) for:
  - Code explanation
  - Summarization
  - General questions
- **Semantic search** over your indexed code
- **Template-driven prompts** for consistent and structured LLM outputs

---

## Installation

```bash
git clone https://github.com/yourusername/ai-cli.git
cd ai-cli
go build -o ai
```

---

## Commands

| Command     | Description                                                                                          |
| ----------- | ---------------------------------------------------------------------------------------------------- |
| `ask`       | Ask the AI any general question (no indexing required)                                               |
| `summarize` | Summarize code or text input                                                                         |
| `explain`   | Explain a piece of code or file                                                                      |
| `index`     | Build the vector index of your codebase                                                              |
| `search`    | Perform a semantic search and return top K chunks                                                    |
| `query`     | Ask a question over your indexed code (RAG: retrieves top-K relevant chunks and generates an answer) |

---

## Examples

### Ask the AI a general question

```bash
./ai ask "What is a mutex in Go?"
```

### Summarize code

```bash
./ai summarize path/to/file.go
```

### Explain code

```bash
./ai explain path/to/file.go
```

### Build a semantic index

```bash
./ai index ./my-repo
```

### Search for code snippets

```bash
./ai search "vector store embedding normalization"
```

### Query the codebase using the LLM

```bash
./ai query "How is the vector store implemented?" --k 5
```

- --k specifies the number of top chunks to retrieve (default: 5)

---

## How it Works

1. Chunking – Code and text files are split into semantic chunks
2. Embedding – Each chunk is converted to a vector via the LLM
3. Vector Store – Chunks and embeddings are stored in SQLite
4. Search – Queries are embedded and compared using cosine similarity
5. LLM Prompting – Retrieved chunks are injected into prompt templates for answers

---

## Configuration

- LLM client settings are configured in internal/embedding/ollama.go
- Prompt templates are stored in internal/prompts (uses Go text/template)
- Vector store persistence is handled automatically in SQLite

---

## License

MIT License
