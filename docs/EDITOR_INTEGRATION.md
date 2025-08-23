# Editor Integration Guide

## ContextDB Integration for Editor Developers

This guide helps editor and IDE developers integrate ContextDB into their tools to capture development context, classify activities, and provide AI-powered insights.

## Overview

ContextDB captures **operations** - discrete, traceable actions that occur during development. Each operation represents a meaningful change or event that contributes to the development context.

### Core Integration Pattern

```
Editor Event → Operation Classification → ContextDB Storage → Analysis & Insights
```

## VSCode Extension Integration

### Basic Extension Setup

```typescript
// extension.ts
import * as vscode from 'vscode';
import { ContextDBClient } from './contextdb-client';

export function activate(context: vscode.ExtensionContext) {
    const contextDB = new ContextDBClient(
        'http://localhost:8080/api/v1',
        getAPIKey()
    );

    // Register event handlers
    registerTextDocumentHandlers(context, contextDB);
    registerGitHandlers(context, contextDB);
    registerTerminalHandlers(context, contextDB);
    registerDebugHandlers(context, contextDB);
}
```

### Text Document Events

```typescript
function registerTextDocumentHandlers(
    context: vscode.ExtensionContext, 
    contextDB: ContextDBClient
) {
    // Document changes
    vscode.workspace.onDidChangeTextDocument(async (event) => {
        const document = event.document;
        
        for (const change of event.contentChanges) {
            await contextDB.createOperation({
                type: 'insert', // or 'delete' based on change type
                position: createPositionFromRange(change.range, document),
                content: change.text,
                author: getActiveAuthor(),
                document_id: document.fileName,
                metadata: {
                    session_id: getSessionId(),
                    context: {
                        language: document.languageId,
                        change_type: change.text ? 'insertion' : 'deletion',
                        line_count: document.lineCount,
                        workspace: vscode.workspace.name
                    }
                }
            });
        }
    });

    // File operations
    vscode.workspace.onDidCreateFiles(async (event) => {
        for (const file of event.files) {
            await contextDB.createOperation({
                type: 'insert',
                position: createFilePosition(file.path),
                content: `File created: ${file.path}`,
                author: getActiveAuthor(),
                document_id: file.path,
                metadata: {
                    operation_type: 'file_creation',
                    file_type: getFileType(file.path)
                }
            });
        }
    });

    // File saves
    vscode.workspace.onDidSaveTextDocument(async (document) => {
        await contextDB.createOperation({
            type: 'insert',
            position: createTimestampPosition(),
            content: `Saved: ${document.fileName}`,
            author: getActiveAuthor(),
            document_id: document.fileName,
            metadata: {
                operation_type: 'file_save',
                language: document.languageId,
                size: document.getText().length
            }
        });
    });
}
```

### Git Integration

```typescript
function registerGitHandlers(
    context: vscode.ExtensionContext, 
    contextDB: ContextDBClient
) {
    // Git extension integration
    const gitExtension = vscode.extensions.getExtension('vscode.git');
    if (gitExtension) {
        const git = gitExtension.exports.getAPI(1);
        
        // Monitor git operations
        git.repositories.forEach(repo => {
            repo.state.onDidChange(async () => {
                const status = repo.state;
                
                // Capture git status changes
                await contextDB.createOperation({
                    type: 'insert',
                    position: createGitPosition(repo.rootUri.path),
                    content: `Git status: ${status.workingTreeChanges.length} changes, ${status.indexChanges.length} staged`,
                    author: getActiveAuthor(),
                    document_id: `${repo.rootUri.path}/.git`,
                    metadata: {
                        operation_type: 'git_status',
                        branch: status.HEAD?.name,
                        working_changes: status.workingTreeChanges.length,
                        staged_changes: status.indexChanges.length
                    }
                });
            });
        });
    }
}
```

### GitHub Integration

```typescript
interface GitHubIntegration {
    // Pull Request Events
    async onPullRequestComment(pr: PullRequest, comment: Comment) {
        await contextDB.createOperation({
            type: 'insert',
            position: createPRPosition(pr.number, comment.id),
            content: `PR Comment: ${comment.body}`,
            author: comment.author,
            document_id: `pr-${pr.number}`,
            metadata: {
                operation_type: 'pr_comment',
                pr_number: pr.number,
                pr_title: pr.title,
                comment_type: comment.type, // 'review', 'issue', 'line'
                file_path: comment.filePath,
                line_number: comment.lineNumber
            }
        });
    }

    // Code Review Events
    async onCodeReview(pr: PullRequest, review: Review) {
        await contextDB.createOperation({
            type: 'insert',
            position: createReviewPosition(pr.number, review.id),
            content: `Code Review: ${review.state} - ${review.body}`,
            author: review.author,
            document_id: `pr-${pr.number}`,
            metadata: {
                operation_type: 'code_review',
                pr_number: pr.number,
                review_state: review.state, // 'approved', 'changes_requested', 'commented'
                files_changed: review.filesChanged?.length
            }
        });
    }

    // Issue Events
    async onIssueActivity(issue: Issue, activity: IssueActivity) {
        await contextDB.createOperation({
            type: 'insert',
            position: createIssuePosition(issue.number, activity.id),
            content: `Issue ${activity.type}: ${activity.content}`,
            author: activity.author,
            document_id: `issue-${issue.number}`,
            metadata: {
                operation_type: 'issue_activity',
                issue_number: issue.number,
                activity_type: activity.type, // 'comment', 'label', 'assignment', 'close'
                labels: issue.labels
            }
        });
    }
}
```

### Terminal and Command Integration

```typescript
function registerTerminalHandlers(
    context: vscode.ExtensionContext, 
    contextDB: ContextDBClient
) {
    // Terminal commands
    vscode.window.onDidOpenTerminal(async (terminal) => {
        // Monitor terminal output (limited by VSCode API)
        await contextDB.createOperation({
            type: 'insert',
            position: createTerminalPosition(terminal.name),
            content: `Terminal opened: ${terminal.name}`,
            author: getActiveAuthor(),
            document_id: 'terminal',
            metadata: {
                operation_type: 'terminal_open',
                terminal_name: terminal.name
            }
        });
    });

    // Task execution
    vscode.tasks.onDidStartTask(async (event) => {
        const task = event.execution.task;
        await contextDB.createOperation({
            type: 'insert',
            position: createTaskPosition(task.name),
            content: `Task started: ${task.name} - ${task.detail}`,
            author: getActiveAuthor(),
            document_id: 'tasks',
            metadata: {
                operation_type: 'task_start',
                task_name: task.name,
                task_type: task.source,
                command: task.execution?.toString()
            }
        });
    });

    vscode.tasks.onDidEndTask(async (event) => {
        const task = event.execution.task;
        await contextDB.createOperation({
            type: 'insert',
            position: createTaskPosition(task.name),
            content: `Task completed: ${task.name} - Exit code: ${event.exitCode}`,
            author: getActiveAuthor(),
            document_id: 'tasks',
            metadata: {
                operation_type: 'task_end',
                task_name: task.name,
                exit_code: event.exitCode,
                duration: Date.now() - (taskStartTimes.get(task.name) || 0)
            }
        });
    });
}
```

### Debug Session Integration

```typescript
function registerDebugHandlers(
    context: vscode.ExtensionContext, 
    contextDB: ContextDBClient
) {
    vscode.debug.onDidStartDebugSession(async (session) => {
        await contextDB.createOperation({
            type: 'insert',
            position: createDebugPosition(session.id),
            content: `Debug session started: ${session.name} (${session.type})`,
            author: getActiveAuthor(),
            document_id: 'debug',
            metadata: {
                operation_type: 'debug_start',
                session_name: session.name,
                session_type: session.type,
                configuration: session.configuration
            }
        });
    });

    vscode.debug.onDidReceiveDebugSessionCustomEvent(async (event) => {
        if (event.event === 'breakpoint') {
            await contextDB.createOperation({
                type: 'insert',
                position: createBreakpointPosition(event.body.source, event.body.line),
                content: `Breakpoint hit: ${event.body.source.path}:${event.body.line}`,
                author: getActiveAuthor(),
                document_id: event.body.source.path,
                metadata: {
                    operation_type: 'breakpoint_hit',
                    file_path: event.body.source.path,
                    line_number: event.body.line,
                    session_id: event.session.id
                }
            });
        }
    });
}
```

## Operation Classification System

### Activity Categories

```typescript
enum ActivityType {
    // Code Operations
    CODE_EDIT = 'code_edit',
    CODE_REFACTOR = 'code_refactor',
    CODE_DEBUG = 'code_debug',
    
    // File Operations
    FILE_CREATE = 'file_create',
    FILE_DELETE = 'file_delete',
    FILE_RENAME = 'file_rename',
    
    // Git Operations
    GIT_COMMIT = 'git_commit',
    GIT_BRANCH = 'git_branch',
    GIT_MERGE = 'git_merge',
    
    // Collaboration
    PR_REVIEW = 'pr_review',
    ISSUE_DISCUSSION = 'issue_discussion',
    CODE_COMMENT = 'code_comment',
    
    // Build/Deploy
    BUILD_START = 'build_start',
    BUILD_FAIL = 'build_fail',
    DEPLOY = 'deploy',
    
    // Testing
    TEST_RUN = 'test_run',
    TEST_FAIL = 'test_fail',
    TEST_CREATE = 'test_create'
}

class ActivityClassifier {
    static classifyTextChange(change: vscode.TextDocumentContentChangeEvent, document: vscode.TextDocument): ActivityType {
        const text = change.text;
        const range = change.range;
        
        // Detect refactoring patterns
        if (this.isRefactoring(text, document)) {
            return ActivityType.CODE_REFACTOR;
        }
        
        // Detect debugging additions
        if (this.isDebugging(text)) {
            return ActivityType.CODE_DEBUG;
        }
        
        return ActivityType.CODE_EDIT;
    }
    
    static isRefactoring(text: string, document: vscode.TextDocument): boolean {
        // Look for refactoring patterns
        const refactorPatterns = [
            /function\s+\w+\s*\(/,  // Function extraction
            /class\s+\w+/,          // Class extraction
            /import.*from/,         // Import reorganization
            /const\s+\w+\s*=/       // Variable extraction
        ];
        
        return refactorPatterns.some(pattern => pattern.test(text));
    }
    
    static isDebugging(text: string): boolean {
        const debugPatterns = [
            /console\.log/,
            /print\(/,
            /debugger;/,
            /breakpoint/i
        ];
        
        return debugPatterns.some(pattern => pattern.test(text));
    }
}
```

### Context Enrichment

```typescript
class ContextEnricher {
    static async enrichOperation(operation: Operation, document: vscode.TextDocument): Promise<Operation> {
        const context = {
            ...operation.metadata?.context,
            
            // File context
            language: document.languageId,
            file_size: document.getText().length,
            line_count: document.lineCount,
            
            // Workspace context
            workspace_name: vscode.workspace.name,
            workspace_folders: vscode.workspace.workspaceFolders?.length,
            
            // Editor context
            active_editor: vscode.window.activeTextEditor?.document.fileName,
            visible_editors: vscode.window.visibleTextEditors.length,
            
            // Git context
            git_branch: await this.getCurrentBranch(),
            git_status: await this.getGitStatus(),
            
            // Time context
            timestamp: new Date().toISOString(),
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            
            // Session context
            session_duration: Date.now() - sessionStartTime,
            operations_count: await this.getSessionOperationCount()
        };
        
        return {
            ...operation,
            metadata: {
                ...operation.metadata,
                context
            }
        };
    }
}
```

## Performance Optimizations

### Batching Operations

```typescript
class OperationBatcher {
    private batch: Operation[] = [];
    private batchTimeout: NodeJS.Timeout | null = null;
    
    async addOperation(operation: Operation) {
        this.batch.push(operation);
        
        // Batch size limit
        if (this.batch.length >= 10) {
            await this.flushBatch();
        }
        
        // Time-based batching
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
        }
        
        this.batchTimeout = setTimeout(async () => {
            await this.flushBatch();
        }, 1000); // 1 second delay
    }
    
    private async flushBatch() {
        if (this.batch.length === 0) return;
        
        try {
            await this.contextDB.createBatchOperations(this.batch);
            this.batch = [];
        } catch (error) {
            console.error('Failed to flush operation batch:', error);
            // Implement retry logic here
        }
        
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
            this.batchTimeout = null;
        }
    }
}
```

### Smart Filtering

```typescript
class OperationFilter {
    static shouldCapture(operation: Operation, document: vscode.TextDocument): boolean {
        // Skip trivial changes
        if (operation.content.trim().length < 3) {
            return false;
        }
        
        // Skip temporary files
        if (document.fileName.includes('/tmp/') || document.fileName.includes('\\temp\\')) {
            return false;
        }
        
        // Skip generated files
        const generatedPatterns = [
            /\.generated\./,
            /node_modules/,
            /\.git/,
            /\.vscode/
        ];
        
        if (generatedPatterns.some(pattern => pattern.test(document.fileName))) {
            return false;
        }
        
        return true;
    }
}
```

## Integration Examples

### Language Server Protocol Integration

```typescript
class LSPIntegration {
    async onDiagnostics(diagnostics: vscode.Diagnostic[], document: vscode.TextDocument) {
        for (const diagnostic of diagnostics) {
            await contextDB.createOperation({
                type: 'insert',
                position: createDiagnosticPosition(diagnostic.range, document),
                content: `Diagnostic: ${diagnostic.message}`,
                author: 'language-server',
                document_id: document.fileName,
                metadata: {
                    operation_type: 'diagnostic',
                    severity: diagnostic.severity,
                    source: diagnostic.source,
                    code: diagnostic.code
                }
            });
        }
    }
    
    async onCodeAction(action: vscode.CodeAction, document: vscode.TextDocument) {
        await contextDB.createOperation({
            type: 'insert',
            position: createActionPosition(action, document),
            content: `Code action: ${action.title}`,
            author: getActiveAuthor(),
            document_id: document.fileName,
            metadata: {
                operation_type: 'code_action',
                action_kind: action.kind?.value,
                is_preferred: action.isPreferred
            }
        });
    }
}
```

### CI/CD Integration

```typescript
class CIIntegration {
    async onBuildStart(build: BuildInfo) {
        await contextDB.createOperation({
            type: 'insert',
            position: createBuildPosition(build.id),
            content: `Build started: ${build.name} (#${build.number})`,
            author: build.triggeredBy,
            document_id: 'ci-builds',
            metadata: {
                operation_type: 'build_start',
                build_id: build.id,
                commit_sha: build.commitSha,
                branch: build.branch
            }
        });
    }
    
    async onTestResults(results: TestResults) {
        await contextDB.createOperation({
            type: 'insert',
            position: createTestPosition(results.suite),
            content: `Tests: ${results.passed}/${results.total} passed`,
            author: 'ci-system',
            document_id: 'test-results',
            metadata: {
                operation_type: 'test_results',
                passed: results.passed,
                failed: results.failed,
                duration: results.duration,
                coverage: results.coverage
            }
        });
    }
}
```

## Usage Patterns

### Development Session Tracking

```typescript
class SessionTracker {
    private sessionStart = Date.now();
    private operationCount = 0;
    
    async startSession() {
        await contextDB.createOperation({
            type: 'insert',
            position: createSessionPosition(),
            content: `Development session started`,
            author: getActiveAuthor(),
            document_id: 'session',
            metadata: {
                operation_type: 'session_start',
                workspace: vscode.workspace.name,
                extensions: vscode.extensions.all.length
            }
        });
    }
    
    async trackActivity(operation: Operation) {
        this.operationCount++;
        
        // Periodic session summaries
        if (this.operationCount % 50 === 0) {
            await this.createSessionSummary();
        }
    }
    
    private async createSessionSummary() {
        const duration = Date.now() - this.sessionStart;
        
        await contextDB.createOperation({
            type: 'insert',
            position: createSummaryPosition(),
            content: `Session summary: ${this.operationCount} operations in ${duration}ms`,
            author: getActiveAuthor(),
            document_id: 'session',
            metadata: {
                operation_type: 'session_summary',
                duration_ms: duration,
                operations_count: this.operationCount,
                avg_ops_per_minute: (this.operationCount / (duration / 60000))
            }
        });
    }
}
```

### Intent Recognition

```typescript
class IntentRecognizer {
    async analyzeWorkingSession(operations: Operation[]): Promise<SessionIntent> {
        // Group operations by time windows
        const timeWindows = this.groupByTimeWindows(operations, 5 * 60 * 1000); // 5 minute windows
        
        const intents = [];
        for (const window of timeWindows) {
            const windowIntent = await this.analyzeTimeWindow(window);
            intents.push(windowIntent);
        }
        
        return this.aggregateIntents(intents);
    }
    
    private async analyzeTimeWindow(operations: Operation[]): Promise<WindowIntent> {
        // Send to ContextDB for analysis
        const analysis = await contextDB.analyzeIntent(
            operations.map(op => op.id).filter(id => id)
        );
        
        return {
            primary_activity: analysis.collective_intent,
            confidence: this.calculateConfidence(analysis.evidence),
            operation_count: operations.length,
            files_touched: new Set(operations.map(op => op.document_id)).size
        };
    }
}
```

## Best Practices

### 1. Privacy and Security
- **Never capture sensitive data** (passwords, tokens, personal information)
- **Hash or obfuscate** file paths that might contain sensitive information
- **Implement opt-out mechanisms** for users who don't want tracking
- **Use local storage** by default, with explicit opt-in for remote storage

### 2. Performance
- **Batch operations** to reduce API calls
- **Filter trivial changes** to reduce noise
- **Use async processing** to avoid blocking the editor
- **Implement circuit breakers** for API failures

### 3. User Experience
- **Provide clear feedback** about what's being captured
- **Allow granular control** over what gets tracked
- **Show value immediately** through insights and analysis
- **Respect editor performance** - never block user actions

### 4. Data Quality
- **Enrich with context** to make operations meaningful
- **Classify operations** accurately for better analysis
- **Maintain consistency** in operation formats
- **Handle edge cases** gracefully

## Configuration Example

```json
{
  "contextdb.enabled": true,
  "contextdb.server": "http://localhost:8080/api/v1",
  "contextdb.apiKey": "${env:CONTEXTDB_API_KEY}",
  "contextdb.capture": {
    "textChanges": true,
    "fileOperations": true,
    "gitEvents": true,
    "debugSessions": true,
    "terminalCommands": false,
    "diagnostics": true
  },
  "contextdb.filters": {
    "minimumChangeSize": 3,
    "excludeDirectories": ["node_modules", ".git", "dist"],
    "excludeFileTypes": [".log", ".tmp"]
  },
  "contextdb.batching": {
    "enabled": true,
    "maxBatchSize": 10,
    "flushInterval": 1000
  }
}
```

This guide provides the foundation for integrating ContextDB into any editor or IDE, with VSCode as the primary example. The patterns can be adapted to other editors like JetBrains IDEs, Vim, Emacs, or custom development tools.