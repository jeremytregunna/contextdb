#!/bin/bash

# ContextDB Shell Integration Example
# Demonstrates how to integrate ContextDB with shell scripts and command-line workflows

set -euo pipefail

# Configuration
CONTEXTDB_URL="${CONTEXTDB_URL:-http://localhost:8080/api/v1}"
API_KEY="${CONTEXTDB_API_KEY:-}"
AUTHOR="${USER:-shell-user}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Utility functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# API helper function
api_call() {
    local method="$1"
    local endpoint="$2"
    local data="${3:-}"
    
    local curl_args=(-s -X "$method" "$CONTEXTDB_URL$endpoint")
    
    if [ -n "$API_KEY" ]; then
        curl_args+=(-H "Authorization: Bearer $API_KEY")
    fi
    
    if [ -n "$data" ]; then
        curl_args+=(-H "Content-Type: application/json" -d "$data")
    fi
    
    curl "${curl_args[@]}"
}

# Check server health
check_server() {
    log_info "Checking server health..."
    if response=$(api_call GET "/health" 2>/dev/null); then
        if echo "$response" | grep -q '"success":true'; then
            log_success "Server is healthy"
            return 0
        fi
    fi
    log_error "Server is not responding or unhealthy"
    return 1
}

# Create an operation
create_operation() {
    local content="$1"
    local document_id="$2"
    local position_value="${3:-$(date +%s)}"
    
    local json_data=$(cat <<EOF
{
    "type": "insert",
    "position": {
        "segments": [{"value": $position_value, "author": "$AUTHOR"}],
        "hash": "$AUTHOR-$position_value"
    },
    "content": "$content",
    "author": "$AUTHOR",
    "document_id": "$document_id"
}
EOF
    )
    
    local response=$(api_call POST "/operations" "$json_data")
    
    if echo "$response" | grep -q '"success":true'; then
        local op_id=$(echo "$response" | jq -r '.data.id' 2>/dev/null || echo "unknown")
        log_success "Created operation: ${op_id:0:16}..."
        echo "$op_id"
    else
        local error=$(echo "$response" | jq -r '.error // "Unknown error"' 2>/dev/null)
        log_error "Failed to create operation: $error"
        return 1
    fi
}

# Search operations
search_operations() {
    local query="$1"
    local limit="${2:-10}"
    
    log_info "Searching for: '$query'"
    local response=$(api_call GET "/search?q=$(printf '%s' "$query" | sed 's/ /%20/g')&limit=$limit")
    
    if echo "$response" | grep -q '"success":true'; then
        local count=$(echo "$response" | jq -r '.data.results | length' 2>/dev/null || echo "0")
        log_success "Found $count results"
        
        echo "$response" | jq -r '.data.results[] | "  - [\(.type)] \(.content[:50])..."' 2>/dev/null || {
            log_warning "Could not parse search results (jq not available)"
        }
    else
        log_error "Search failed"
        return 1
    fi
}

# List operations for a document
list_operations() {
    local document_id="$1"
    local limit="${2:-20}"
    
    log_info "Listing operations for: $document_id"
    local response=$(api_call GET "/operations?document_id=$document_id&limit=$limit")
    
    if echo "$response" | grep -q '"success":true'; then
        local count=$(echo "$response" | jq -r '. | length' 2>/dev/null || echo "0")
        log_success "Found $count operations"
        
        echo "$response" | jq -r '.data[] | "  \(.timestamp[:19]) - \(.author) - \(.content[:50])..."' 2>/dev/null || {
            log_warning "Could not parse operations (jq not available)"
        }
    else
        log_error "Failed to list operations"
        return 1
    fi
}

# Monitor file changes and create operations
monitor_file() {
    local file_path="$1"
    local document_id="${2:-$(basename "$file_path")}"
    
    if ! command -v inotifywait >/dev/null 2>&1; then
        log_error "inotifywait not found. Please install inotify-tools."
        return 1
    fi
    
    if [ ! -f "$file_path" ]; then
        log_error "File not found: $file_path"
        return 1
    fi
    
    log_info "Monitoring file: $file_path"
    log_info "Press Ctrl+C to stop monitoring"
    
    # Initial content capture
    local initial_content=$(head -c 500 "$file_path" | tr '\n' ' ')
    create_operation "Initial content: $initial_content" "$document_id" > /dev/null
    
    while inotifywait -e modify "$file_path" >/dev/null 2>&1; do
        sleep 1  # Debounce rapid changes
        local content=$(head -c 500 "$file_path" | tr '\n' ' ')
        local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        create_operation "File modified at $timestamp: $content" "$document_id" > /dev/null
        log_success "Captured change to $file_path"
    done
}

# Backup operations to JSON file
backup_operations() {
    local output_file="${1:-operations_backup_$(date +%Y%m%d_%H%M%S).json}"
    
    log_info "Backing up operations to: $output_file"
    local response=$(api_call GET "/operations?limit=1000")
    
    if echo "$response" | grep -q '"success":true'; then
        echo "$response" | jq '.data' > "$output_file" 2>/dev/null || {
            echo "$response" > "$output_file"
            log_warning "Backup created but jq formatting failed"
        }
        log_success "Backup created: $output_file"
    else
        log_error "Failed to backup operations"
        return 1
    fi
}

# Interactive shell mode
interactive_mode() {
    log_info "Entering interactive mode (type 'help' for commands)"
    
    while true; do
        printf "contextdb> "
        read -r input
        
        case "$input" in
            "help")
                echo "Available commands:"
                echo "  create <content> <document_id>  - Create an operation"
                echo "  search <query>                  - Search operations"
                echo "  list <document_id>              - List operations for document"
                echo "  health                          - Check server health"
                echo "  backup [filename]               - Backup operations"
                echo "  exit                            - Exit interactive mode"
                ;;
            "health")
                check_server
                ;;
            "exit"|"quit")
                log_info "Exiting interactive mode"
                break
                ;;
            create\ *)
                # Parse create command: create <content> <document_id>
                local args=($input)
                if [ ${#args[@]} -lt 3 ]; then
                    log_error "Usage: create <content> <document_id>"
                else
                    create_operation "${args[1]}" "${args[2]}" > /dev/null
                fi
                ;;
            search\ *)
                local query="${input#search }"
                search_operations "$query"
                ;;
            list\ *)
                local doc_id="${input#list }"
                list_operations "$doc_id"
                ;;
            backup*)
                local args=($input)
                local filename="${args[1]:-}"
                backup_operations "$filename"
                ;;
            "")
                # Empty input, continue
                ;;
            *)
                log_error "Unknown command: $input (type 'help' for available commands)"
                ;;
        esac
    done
}

# Main function
main() {
    echo "🚀 ContextDB Shell Integration Example"
    echo "======================================"
    
    # Check dependencies
    if ! command -v curl >/dev/null 2>&1; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    if ! check_server; then
        log_error "Cannot connect to ContextDB server at $CONTEXTDB_URL"
        exit 1
    fi
    
    case "${1:-help}" in
        "create")
            shift
            if [ $# -lt 2 ]; then
                log_error "Usage: $0 create <content> <document_id>"
                exit 1
            fi
            create_operation "$1" "$2" > /dev/null
            ;;
        "search")
            shift
            if [ $# -lt 1 ]; then
                log_error "Usage: $0 search <query>"
                exit 1
            fi
            search_operations "$1"
            ;;
        "list")
            shift
            if [ $# -lt 1 ]; then
                log_error "Usage: $0 list <document_id>"
                exit 1
            fi
            list_operations "$1"
            ;;
        "monitor")
            shift
            if [ $# -lt 1 ]; then
                log_error "Usage: $0 monitor <file_path> [document_id]"
                exit 1
            fi
            monitor_file "$1" "${2:-}"
            ;;
        "backup")
            shift
            backup_operations "${1:-}"
            ;;
        "interactive"|"shell")
            interactive_mode
            ;;
        "demo")
            # Run a quick demo
            log_info "Running demo..."
            
            # Create sample operations
            local doc_id="demo-$(date +%s)"
            create_operation "function hello() { console.log('Hello World'); }" "$doc_id.js" > /dev/null
            create_operation "function goodbye() { console.log('Goodbye'); }" "$doc_id.js" > /dev/null
            
            # Search and list
            search_operations "function"
            echo
            list_operations "$doc_id.js"
            ;;
        "help"|*)
            cat << 'EOF'
ContextDB Shell Integration

Usage: ./shell_integration.sh <command> [arguments]

Commands:
  create <content> <document_id>     Create a new operation
  search <query>                     Search operations
  list <document_id>                 List operations for a document
  monitor <file_path> [document_id]  Monitor file changes (requires inotify-tools)
  backup [filename]                  Backup all operations to JSON
  interactive                        Enter interactive shell mode
  demo                              Run a quick demonstration
  help                              Show this help message

Environment Variables:
  CONTEXTDB_URL                     Server URL (default: http://localhost:8080/api/v1)
  CONTEXTDB_API_KEY                 API key for authentication
  
Examples:
  ./shell_integration.sh create "console.log('test')" "test.js"
  ./shell_integration.sh search "function"
  ./shell_integration.sh monitor ./src/main.js main.js
  ./shell_integration.sh interactive

EOF
            ;;
    esac
}

# Run main function with all arguments
main "$@"