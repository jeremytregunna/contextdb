# ContextDB VSCode Extension

A comprehensive VSCode extension that integrates with ContextDB to capture development context, classify activities, and provide AI-powered insights.

## Features

- **Real-time Context Capture**: Automatically tracks text changes, file operations, git events, and debug sessions
- **Intelligent Classification**: Uses pattern matching and context analysis to classify development activities
- **Session Analytics**: Provides detailed session statistics and productivity insights
- **Performance Optimized**: Smart batching and filtering to minimize overhead
- **Privacy First**: Local operation with configurable data capture controls

## Installation

### Prerequisites

1. **ContextDB Server**: Ensure ContextDB server is running locally or remotely
2. **Node.js**: Version 16 or higher
3. **TypeScript**: For development

### Building the Extension

```bash
# Clone the repository
git clone <repository-url>
cd contextdb/examples/vscode-extension

# Install dependencies
npm install

# Compile TypeScript
npm run compile

# Package extension (optional)
npm install -g vsce
vsce package
```

### Installing in VSCode

1. **From Source**: 
   - Open the extension folder in VSCode
   - Press `F5` to run in Extension Development Host

2. **From Package**:
   - Install the generated `.vsix` file via `Extensions: Install from VSIX...`

## Configuration

Configure the extension through VSCode settings (`Ctrl+,`):

### Basic Settings

```json
{
  "contextdb.enabled": true,
  "contextdb.serverUrl": "http://localhost:8080/api/v1",
  "contextdb.apiKey": "your-api-key-here"
}
```

### Capture Settings

```json
{
  "contextdb.captureTextChanges": true,
  "contextdb.captureFileOperations": true,
  "contextdb.captureGitEvents": true,
  "contextdb.captureDebugSessions": true,
  "contextdb.minimumChangeSize": 3
}
```

### Performance Settings

```json
{
  "contextdb.batchSize": 10,
  "contextdb.flushInterval": 5000,
  "contextdb.excludeDirectories": [
    "node_modules",
    ".git",
    "dist",
    "build",
    ".vscode"
  ],
  "contextdb.excludeFileTypes": [
    ".log",
    ".tmp",
    ".cache"
  ]
}
```

## Usage

### Commands

Access through Command Palette (`Ctrl+Shift+P`):

- **ContextDB: Enable Context Capturing** - Start capturing development context
- **ContextDB: Disable Context Capturing** - Stop capturing context
- **ContextDB: Show Session Statistics** - View current session stats
- **ContextDB: Analyze Current Session** - Get detailed session analysis
- **ContextDB: Flush Pending Operations** - Manually send queued operations

### Keyboard Shortcuts

- `Ctrl+Alt+C` (Windows/Linux) / `Cmd+Alt+C` (Mac): Analyze Current Session

### Status Indicators

The extension provides visual feedback in the status bar:
- **ContextDB: Active** - Context capturing is enabled and working
- **ContextDB: Error** - Connection or API issues detected
- **ContextDB: Idle** - Extension loaded but capturing disabled

## Activity Classification

The extension automatically classifies development activities:

### Code Activities
- **Code Editing**: General text changes and modifications
- **Code Refactoring**: Function extraction, variable renaming, import reorganization
- **Debugging**: Adding console.log, breakpoints, debug statements

### File Operations
- **File Creation**: New files added to workspace
- **File Deletion**: Files removed from workspace
- **File Renaming**: File path changes

### Development Workflow
- **Testing**: Working with test files, writing test cases
- **Documentation**: Editing markdown, comments, README files
- **Configuration**: Modifying config files, package.json, etc.
- **API Development**: Working with REST APIs, async code
- **UI Development**: Frontend components, styling, templates
- **Database Work**: SQL queries, ORM operations

### Git Integration
- **Git Operations**: Commits, branch changes, status updates
- **Code Review**: Pull request activities, review comments

## Session Analytics

### Real-time Statistics
- Operation count and frequency
- Files modified and most active file
- Language breakdown
- Activity type distribution
- Lines added/deleted

### Productivity Metrics
- **Productivity Score**: Operations per minute
- **Focus Score**: Based on file switching frequency
- **Session Duration**: Total active development time
- **Average Operation Interval**: Time between operations

### Intent Analysis
Leverages ContextDB's intent analysis to understand:
- Primary development intent for the session
- Evidence supporting the classification
- Collective patterns across operations

## Privacy and Security

### Data Captured
- **File paths** (configurable exclusions)
- **Code changes** (content of modifications)
- **Git operations** (status, branch info)
- **Debug session info** (session names, types)
- **Timing data** (operation timestamps)

### Data NOT Captured
- **File contents** (only changes are captured)
- **Passwords or secrets** (filtered out)
- **Personal information** (outside development context)
- **External communications** (emails, chats, etc.)

### Privacy Controls
- **Opt-out directories**: Exclude sensitive folders
- **File type filtering**: Skip temporary/log files
- **Local storage**: Data stays on your machine by default
- **Disable anytime**: Easy on/off toggle

## Architecture

### Core Components

1. **ContextDBClient**: API communication layer
2. **OperationBatcher**: Performance optimization for API calls
3. **ActivityClassifier**: Pattern-based activity recognition
4. **SessionTracker**: Session analytics and management

### Event Handling

```typescript
// Text changes
workspace.onDidChangeTextDocument → classify → batch → send

// File operations  
workspace.onDidCreateFiles → classify → batch → send

// Git events
git.onDidChangeState → classify → batch → send

// Debug sessions
debug.onDidStartDebugSession → classify → batch → send
```

### Performance Optimizations

- **Smart Batching**: Groups related operations
- **Deduplication**: Prevents duplicate operations
- **Filtering**: Skips trivial or unwanted changes
- **Async Processing**: Non-blocking operation handling

## Troubleshooting

### Common Issues

**Extension not capturing data**
- Check if ContextDB server is running
- Verify server URL in settings
- Check API key if authentication is enabled
- Look at VSCode Developer Console for errors

**High CPU usage**
- Increase `batchSize` to reduce API calls
- Add more exclusions in `excludeDirectories`
- Increase `minimumChangeSize` to skip small changes

**Missing operations**
- Check `flushInterval` - operations may be batched
- Verify network connectivity to ContextDB server
- Check server logs for API errors

### Debug Mode

Enable debug logging:

```json
{
  "contextdb.debug": true
}
```

View logs in VSCode Output panel (select "ContextDB" channel).

### Reset Extension

To reset all extension data:
1. Disable the extension
2. Clear workspace storage: `Developer: Reload Window`
3. Re-enable extension

## Development

### Building from Source

```bash
# Install dependencies
npm install

# Compile TypeScript
npm run compile

# Watch for changes
npm run watch

# Run tests (if available)
npm test
```

### Extension Structure

```
src/
├── extension.ts          # Main extension entry point
├── contextdb-client.ts   # ContextDB API client
├── operation-batcher.ts  # Performance batching logic
├── activity-classifier.ts # Activity pattern recognition
└── session-tracker.ts   # Session analytics
```

### Adding New Activity Types

1. Add activity type to `ActivityType` enum in `contextdb-client.ts`
2. Add classification patterns in `activity-classifier.ts`
3. Update activity descriptions in `getActivityDescription()`

### Custom Event Handlers

```typescript
// Register custom event handler
const handler = vscode.workspace.onDidChangeConfiguration(event => {
    if (event.affectsConfiguration('myextension')) {
        // Handle configuration change
    }
});

context.subscriptions.push(handler);
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

- **Documentation**: See [EDITOR_INTEGRATION.md](../../EDITOR_INTEGRATION.md)
- **Issues**: GitHub Issues
- **Discussions**: GitHub Discussions