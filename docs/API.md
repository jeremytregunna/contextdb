# ContextDB REST API Documentation

## Overview

ContextDB provides a REST API for external integration, enabling AI tools and other applications to interact with the operation-based version control system. The API supports operations management, search functionality, intent analysis, and authentication.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

### API Key Authentication

When authentication is enabled, include the API key in the Authorization header:

```
Authorization: Bearer your-api-key-here
```

### Managing API Keys

#### Create API Key
```http
POST /api/v1/auth/keys
Content-Type: application/json

{
  "name": "my-integration",
  "author_id": "developer-123",
  "permissions": ["read:operations", "write:operations", "analyze"],
  "expires_in": "24h"
}
```

#### List API Keys
```http
GET /api/v1/auth/keys
```

#### Revoke API Key
```http
DELETE /api/v1/auth/keys/{key_id}
```

## Operations API

### Create Operation
```http
POST /api/v1/operations
Content-Type: application/json

{
  "type": "insert",
  "position": {
    "segments": [{"value": 1, "author": "user-123"}],
    "hash": "position-hash"
  },
  "content": "{\"type\": \"insert\", \"added\": \"Hello, world!\"}",
  "content_type": "json",
  "author": "user-123",
  "document_id": "main.go",
  "metadata": {
    "session_id": "session-123",
    "context": {
      "language": "go",
      "file_size": "1024",
      "line_count": "45",
      "workspace": "my-project"
    }
  }
}
```

#### Content Types

The `content_type` field indicates how to interpret the `content` field:

- **`text`** - Plain text content (default for backwards compatibility)
- **`json`** - Structured JSON data containing operation details
- **`binary`** - Base64-encoded binary content

#### Structured Content Examples

**Text Changes:**
```json
{
  "type": "delete",
  "content": "{\"type\": \"delete\", \"deleted\": \"old code here\"}",
  "content_type": "json"
}
```

```json
{
  "type": "insert", 
  "content": "{\"type\": \"replace\", \"old\": \"old code\", \"new\": \"new code\"}",
  "content_type": "json"
}
```

**Session Events:**
```json
{
  "type": "insert",
  "content": "{\"type\": \"session\", \"event\": \"file_save\", \"message\": \"Saved main.go\"}",
  "content_type": "json"
}
```

### Get Operation
```http
GET /api/v1/operations/{operation_id}
```

### List Operations
```http
GET /api/v1/operations?document_id=main.go&author=user-123&limit=50&offset=0
```

### Get Operation Intent
```http
GET /api/v1/operations/{operation_id}/intent
```

## Search API

### Search Operations
```http
GET /api/v1/search?q=hello&type=operations&limit=20&offset=0
```

### Search Documents
```http
GET /api/v1/search?q=main&type=documents&limit=20&offset=0
```

### Search All Content
```http
GET /api/v1/search?q=function&limit=20&offset=0
```

## Analysis API

### Analyze Operation Intent
```http
POST /api/v1/analyze/intent
Content-Type: application/json

{
  "operations": ["operation-id-1", "operation-id-2"]
}
```

## Health Check

```http
GET /api/v1/health
```

## Response Format

All API responses follow this format:

### Success Response
```json
{
  "success": true,
  "data": { ... },
  "message": "Operation completed successfully"
}
```

### Error Response
```json
{
  "success": false,
  "error": "Error description",
  "code": "ERROR_CODE"
}
```

## Status Codes

- `200` - Success
- `201` - Created
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `500` - Internal Server Error

## Rate Limiting

The API implements basic rate limiting to prevent abuse. Current limits:
- 1000 requests per minute per API key
- 10,000 operations per day per API key

## Examples

See the `examples/` directory for complete integration examples in various programming languages.