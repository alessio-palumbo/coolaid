# coolaid

`coolaid` is a local AI-powered CLI tool for exploring and querying codebases. It supports semantic search, code explanations, summarization, and general LLM queries — all running locally with your own embeddings and vector store.

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

### Prerequisites

`coolaid` runs fully locally but requires Ollama to be installed and running.

1. Install Ollama: https://ollama.com
2. Pull the required models:

```bash
ollama pull llama3.1
ollama pull nomic-embed-text
```

- LLM model (default): llama3.1
- Embedding model (default): nomic-embed-text

You can change these later via configuration if needed.

### Download Prebuild Binary (Recommended)

Download the latest release for your platform from GitHub:
👉 https://github.com/alessio-palumbo/coolaid/releases

Then make it executable (Linux/macOS):

```bash
chmod +x ai
./ai --help
```

### Build from source

```bash
git clone https://github.com/alessio-palumbo/coolaid.git
cd coolaid
go build -o ai ./cmd/main
```

### Verify installation

```bash
./ai ask "What is a mutex in Go?"
```

---

## Getting Started

Before using most commands, you need to index your codebase.

```bash
./ai index
```

By default, `coolaid` will:

- Detect the root of the current Git repository and index it
- Fallback to the current directory if no Git repo is found

This process:

- Chunks your code into meaningful pieces
- Generates embeddings for each chunk
- Stores them in a local vector database

> Note: All commands except ask require an index to exist.

---

## Storage & Configuration

### Configuration

On first run, coolaid will automatically create a configuration file:

```bash
~/.ai/config.toml
```

This file contains:

- LLM model (default: llama3.1)
- Embedding model (default: nomic-embed-text)
- Indexing settings (included extensions and ignore patterns)

You can edit this file to customize how coolaid behaves.

### Vector Store (Indexes)

Indexed data is stored locally in:

```bash
~/.ai/indexes/
```

Each project gets its own SQLite database, automatically named based on:

- The project root directory
- A short hash (to avoid collisions)

This allows multiple projects to be indexed independently without conflicts.

### When to Rebuild the Index

You should rerun:

```bash
./ai index
```

whenever:

- Files in your codebase change significantly
- You update include_extensions
- You update ignore_patterns
- You change the embedding model

> Note: The index is not automatically kept in sync with your codebase.
> If a breaking change or configuration mismatch is detected, coolaid will return an error prompting you to rebuild the index.

---

## Commands

| Command     | Description                                                                                          |
| ----------- | ---------------------------------------------------------------------------------------------------- |
| `ask`       | Ask the AI any general question (no indexing required)                                               |
| `summarize` | Summarize code or text input                                                                         |
| `index`     | Build the vector index of your codebase                                                              |
| `search`    | Perform a semantic search and return top K chunks                                                    |
| `query`     | Ask a question over your indexed code (RAG: retrieves top-K relevant chunks and generates an answer) |
| `explain`   | Explain a piece of code or file                                                                      |
| `test`      | Generate tests for a piece of code or file                                                           |

---

## Examples

### Ask the AI a general question (LLM only)

```bash
./ai ask "What is a mutex in Go?"
```

- -web optional, performs a web search on the prompt (use DuckDuckGo html search)
- -search_limit optional, set limit for search responses (defaults to 5)

### Summarize code (LLM only)

```bash
./ai summarize path/to/file.go
```

### Build a semantic index (required to perform any of the action below)

```bash
./ai index ./my-repo
```

### Start a chat session (w RAG)

```bash
./ai chat
```

### Search for code snippets (w RAG)

```bash
./ai search "vector store embedding normalization"
```

- -k specifies the number of top chunks to retrieve (default: 5)

### Query the codebase using the LLM (w RAG)

```bash
./ai query "How is the vector store implemented?" -mode balanced
```

- -mode specifies the default mode use for RAG: fast, balanced or deep (use MMR)

### Explain file or function code (w RAG)

```bash
./ai explain path/to/file.go [-fn functionName]
```

- -fn optional, targets functionName only

### Generate test for file or function (w RAG)

```bash
./ai test path/to/file.go [-fn functionName]
```

- -fn optional, targets functionName only

### Edit file or function (optional RAG)

```bash
./ai edit path/to/file.go [-fn functionName]
```

- -fn optional, targets functionName only
- -rag optional, set to true to use RAG for extra context

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
