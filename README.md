# ContextDB

A pure operation-based version control system with stable addressing for character-level permalinks and real-time collaboration.

## Features

- **Operation-based Version Control**: Pure CRDT implementation using Logoot positioning
- **Stable Addressing**: Character-level permalinks that remain valid across document changes
- **REST API**: Complete API for external integration with AI tools
- **Real-time Search**: Full-text search across operations and documents
- **Intent Analysis**: Automatic operation classification and intent detection
- **Authentication**: API key-based authentication with permissions

## Quick Start

```bash
# Clone and build
git clone https://github.com/jeremytregunna/contextdb.git
cd contextdb
go build -o contextdb ./cmd/contextdb

# Start server
./contextdb

# Create your first operation
curl -X POST http://localhost:8080/api/v1/operations \
  -H "Content-Type: application/json" \
  -d '{
    "type": "insert",
    "position": {"segments": [{"value": 1, "author": "user-123"}], "hash": "initial"},
    "content": "Hello, world!",
    "author": "user-123",
    "document_id": "hello.txt"
  }'
```

## Documentation

- **[API Documentation](API.md)** - Complete REST API reference
- **[Editor Integration Guide](docs/EDITOR_INTEGRATION.md)** - How to integrate ContextDB into editors and IDEs

## Examples

Integration examples available in `examples/`:
- Python client library
- Node.js integration with AI patterns
- Go application integration
- Shell scripting utilities
- Complete VSCode extension

## License

Copyright (C) 2025 Jeremy Tregunna

Licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for details.

## Support

For issues and questions, please use the GitHub issue tracker.

Also, please read the [CONTRIBUTING.md](./CONTRIBUTING.md) file for the rules on contributing.
