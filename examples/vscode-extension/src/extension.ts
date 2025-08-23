import * as vscode from 'vscode';
import { ContextDBClient, Operation, ActivityType } from './contextdb-client';
import { OperationBatcher } from './operation-batcher';
import { ActivityClassifier } from './activity-classifier';
import { SessionTracker } from './session-tracker';

let contextDB: ContextDBClient;
let batcher: OperationBatcher;
let sessionTracker: SessionTracker;
let isEnabled = true;

export function activate(context: vscode.ExtensionContext) {
    console.log('ContextDB extension activated');

    // Initialize components
    initializeComponents(context);
    
    // Register event handlers
    registerCommands(context);
    registerEventHandlers(context);
    
    // Start session tracking
    sessionTracker.startSession();
    
    // Show activation message
    vscode.window.showInformationMessage('ContextDB: Context capturing enabled');
}

export function deactivate() {
    console.log('ContextDB extension deactivated');
    
    // Flush any remaining operations
    if (batcher) {
        batcher.flushBatch();
    }
    
    // End session tracking
    if (sessionTracker) {
        sessionTracker.endSession();
    }
}

function initializeComponents(context: vscode.ExtensionContext) {
    const config = vscode.workspace.getConfiguration('contextdb');
    
    // Initialize ContextDB client
    contextDB = new ContextDBClient(
        config.get('serverUrl') || 'http://localhost:8080/api/v1',
        config.get('apiKey') || ''
    );
    
    // Initialize operation batcher
    batcher = new OperationBatcher(
        contextDB,
        config.get('batchSize') || 10,
        config.get('flushInterval') || 5000
    );
    
    // Initialize session tracker
    sessionTracker = new SessionTracker(contextDB, context.globalState);
    
    // Update enabled state
    isEnabled = config.get('enabled', true);
    vscode.commands.executeCommand('setContext', 'contextdb.enabled', isEnabled);
}

function registerCommands(context: vscode.ExtensionContext) {
    // Enable/Disable commands
    const enableCmd = vscode.commands.registerCommand('contextdb.enable', () => {
        isEnabled = true;
        vscode.workspace.getConfiguration('contextdb').update('enabled', true, true);
        vscode.commands.executeCommand('setContext', 'contextdb.enabled', true);
        vscode.window.showInformationMessage('ContextDB: Context capturing enabled');
    });
    
    const disableCmd = vscode.commands.registerCommand('contextdb.disable', () => {
        isEnabled = false;
        vscode.workspace.getConfiguration('contextdb').update('enabled', false, true);
        vscode.commands.executeCommand('setContext', 'contextdb.enabled', false);
        vscode.window.showInformationMessage('ContextDB: Context capturing disabled');
    });
    
    // Show statistics
    const showStatsCmd = vscode.commands.registerCommand('contextdb.showStats', async () => {
        const stats = await sessionTracker.getSessionStats();
        const message = `Session Stats:\n` +
            `Duration: ${Math.round(stats.duration / 60000)} minutes\n` +
            `Operations: ${stats.operationCount}\n` +
            `Files modified: ${stats.filesModified}\n` +
            `Most active file: ${stats.mostActiveFile}`;
        
        vscode.window.showInformationMessage(message);
    });
    
    // Analyze current session
    const analyzeCmd = vscode.commands.registerCommand('contextdb.analyzeSession', async () => {
        try {
            const analysis = await sessionTracker.analyzeCurrentSession();
            
            const panel = vscode.window.createWebviewPanel(
                'contextdb.analysis',
                'Session Analysis',
                vscode.ViewColumn.One,
                {}
            );
            
            panel.webview.html = generateAnalysisHTML(analysis);
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to analyze session: ${error}`);
        }
    });
    
    // Flush operations
    const flushCmd = vscode.commands.registerCommand('contextdb.flushOperations', async () => {
        try {
            await batcher.flushBatch();
            vscode.window.showInformationMessage('ContextDB: Operations flushed successfully');
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to flush operations: ${error}`);
        }
    });
    
    context.subscriptions.push(enableCmd, disableCmd, showStatsCmd, analyzeCmd, flushCmd);
}

function registerEventHandlers(context: vscode.ExtensionContext) {
    const config = vscode.workspace.getConfiguration('contextdb');
    
    // Text document changes
    if (config.get('captureTextChanges', true)) {
        const textChangeHandler = vscode.workspace.onDidChangeTextDocument(async (event) => {
            if (!isEnabled || !shouldCaptureDocument(event.document)) return;
            
            for (const change of event.contentChanges) {
                if (change.text.trim().length < config.get('minimumChangeSize', 3)) continue;
                
                const activityType = ActivityClassifier.classifyTextChange(change, event.document);
                const operation = createTextChangeOperation(change, event.document, activityType);
                
                await batcher.addOperation(operation);
                sessionTracker.trackOperation(operation);
            }
        });
        context.subscriptions.push(textChangeHandler);
    }
    
    // File operations
    if (config.get('captureFileOperations', true)) {
        const fileCreateHandler = vscode.workspace.onDidCreateFiles(async (event) => {
            if (!isEnabled) return;
            
            for (const file of event.files) {
                if (!shouldCaptureFile(file.path)) continue;
                
                const operation = createFileOperation('create', file.path);
                await batcher.addOperation(operation);
                sessionTracker.trackOperation(operation);
            }
        });
        
        const fileDeleteHandler = vscode.workspace.onDidDeleteFiles(async (event) => {
            if (!isEnabled) return;
            
            for (const file of event.files) {
                const operation = createFileOperation('delete', file.path);
                await batcher.addOperation(operation);
                sessionTracker.trackOperation(operation);
            }
        });
        
        const fileRenameHandler = vscode.workspace.onDidRenameFiles(async (event) => {
            if (!isEnabled) return;
            
            for (const file of event.files) {
                const operation = createFileOperation('rename', file.newUri.path, {
                    old_path: file.oldUri.path
                });
                await batcher.addOperation(operation);
                sessionTracker.trackOperation(operation);
            }
        });
        
        context.subscriptions.push(fileCreateHandler, fileDeleteHandler, fileRenameHandler);
    }
    
    // File saves
    const fileSaveHandler = vscode.workspace.onDidSaveTextDocument(async (document) => {
        if (!isEnabled || !shouldCaptureDocument(document)) return;
        
        const operation = createFileSaveOperation(document);
        await batcher.addOperation(operation);
        sessionTracker.trackOperation(operation);
    });
    context.subscriptions.push(fileSaveHandler);
    
    // Git events
    if (config.get('captureGitEvents', true)) {
        registerGitHandlers(context);
    }
    
    // Debug sessions
    if (config.get('captureDebugSessions', true)) {
        registerDebugHandlers(context);
    }
    
    // Configuration changes
    const configChangeHandler = vscode.workspace.onDidChangeConfiguration(event => {
        if (event.affectsConfiguration('contextdb')) {
            initializeComponents(context);
        }
    });
    context.subscriptions.push(configChangeHandler);
}

function registerGitHandlers(context: vscode.ExtensionContext) {
    const gitExtension = vscode.extensions.getExtension('vscode.git');
    if (!gitExtension) return;
    
    gitExtension.activate().then(gitApi => {
        const git = gitApi.getAPI(1);
        
        git.repositories.forEach(repo => {
            const stateChangeHandler = repo.state.onDidChange(async () => {
                if (!isEnabled) return;
                
                const operation = createGitStatusOperation(repo);
                await batcher.addOperation(operation);
                sessionTracker.trackOperation(operation);
            });
            
            context.subscriptions.push(stateChangeHandler);
        });
    });
}

function registerDebugHandlers(context: vscode.ExtensionContext) {
    const debugStartHandler = vscode.debug.onDidStartDebugSession(async (session) => {
        if (!isEnabled) return;
        
        const operation = createDebugOperation('start', session);
        await batcher.addOperation(operation);
        sessionTracker.trackOperation(operation);
    });
    
    const debugEndHandler = vscode.debug.onDidTerminateDebugSession(async (session) => {
        if (!isEnabled) return;
        
        const operation = createDebugOperation('end', session);
        await batcher.addOperation(operation);
        sessionTracker.trackOperation(operation);
    });
    
    context.subscriptions.push(debugStartHandler, debugEndHandler);
}

// Helper functions for creating operations
function createTextChangeOperation(
    change: vscode.TextDocumentContentChangeEvent,
    document: vscode.TextDocument,
    activityType: ActivityType
): Operation {
    const position = {
        segments: [{
            value: change.rangeOffset + Date.now(),
            author: getAuthor()
        }],
        hash: `${document.fileName}-${change.rangeOffset}-${Date.now()}`
    };
    
    return {
        type: change.text ? 'insert' : 'delete',
        position,
        content: change.text || `Deleted ${change.rangeLength} characters`,
        author: getAuthor(),
        document_id: document.fileName,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                activity_type: activityType,
                language: document.languageId,
                line_number: change.range.start.line,
                character: change.range.start.character,
                change_length: change.text.length,
                workspace: vscode.workspace.name || 'unknown'
            }
        }
    };
}

function createFileOperation(
    operation: 'create' | 'delete' | 'rename',
    filePath: string,
    additional?: { old_path?: string }
): Operation {
    const position = {
        segments: [{
            value: Date.now() + Math.random() * 1000,
            author: getAuthor()
        }],
        hash: `file-${operation}-${Date.now()}`
    };
    
    let content = `File ${operation}: ${filePath}`;
    if (additional?.old_path) {
        content += ` (from ${additional.old_path})`;
    }
    
    return {
        type: 'insert',
        position,
        content,
        author: getAuthor(),
        document_id: filePath,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                activity_type: `file_${operation}` as ActivityType,
                file_type: getFileExtension(filePath),
                old_path: additional?.old_path
            }
        }
    };
}

function createFileSaveOperation(document: vscode.TextDocument): Operation {
    const position = {
        segments: [{
            value: Date.now(),
            author: getAuthor()
        }],
        hash: `save-${document.fileName}-${Date.now()}`
    };
    
    return {
        type: 'insert',
        position,
        content: `Saved: ${document.fileName}`,
        author: getAuthor(),
        document_id: document.fileName,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                activity_type: ActivityType.FILE_SAVE,
                language: document.languageId,
                file_size: document.getText().length,
                line_count: document.lineCount
            }
        }
    };
}

function createGitStatusOperation(repo: any): Operation {
    const position = {
        segments: [{
            value: Date.now(),
            author: getAuthor()
        }],
        hash: `git-${repo.rootUri.path}-${Date.now()}`
    };
    
    const status = repo.state;
    return {
        type: 'insert',
        position,
        content: `Git status: ${status.workingTreeChanges.length} changes, ${status.indexChanges.length} staged`,
        author: getAuthor(),
        document_id: `${repo.rootUri.path}/.git`,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                activity_type: ActivityType.GIT_STATUS,
                branch: status.HEAD?.name || 'unknown',
                working_changes: status.workingTreeChanges.length,
                staged_changes: status.indexChanges.length
            }
        }
    };
}

function createDebugOperation(type: 'start' | 'end', session: vscode.DebugSession): Operation {
    const position = {
        segments: [{
            value: Date.now(),
            author: getAuthor()
        }],
        hash: `debug-${type}-${session.id}-${Date.now()}`
    };
    
    return {
        type: 'insert',
        position,
        content: `Debug session ${type}: ${session.name} (${session.type})`,
        author: getAuthor(),
        document_id: 'debug',
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                activity_type: type === 'start' ? ActivityType.DEBUG_START : ActivityType.DEBUG_END,
                debug_session_id: session.id,
                debug_session_name: session.name,
                debug_type: session.type
            }
        }
    };
}

// Utility functions
function shouldCaptureDocument(document: vscode.TextDocument): boolean {
    const config = vscode.workspace.getConfiguration('contextdb');
    const excludeDirs = config.get('excludeDirectories', []);
    const excludeTypes = config.get('excludeFileTypes', []);
    
    // Check excluded directories
    for (const dir of excludeDirs) {
        if (document.fileName.includes(`/${dir}/`) || document.fileName.includes(`\\${dir}\\`)) {
            return false;
        }
    }
    
    // Check excluded file types
    for (const ext of excludeTypes) {
        if (document.fileName.endsWith(ext)) {
            return false;
        }
    }
    
    // Skip untitled documents
    if (document.isUntitled) {
        return false;
    }
    
    return true;
}

function shouldCaptureFile(filePath: string): boolean {
    const config = vscode.workspace.getConfiguration('contextdb');
    const excludeDirs = config.get('excludeDirectories', []);
    
    for (const dir of excludeDirs) {
        if (filePath.includes(`/${dir}/`) || filePath.includes(`\\${dir}\\`)) {
            return false;
        }
    }
    
    return true;
}

function getAuthor(): string {
    // Try to get git user name, fallback to OS user
    const gitConfig = vscode.workspace.getConfiguration('git');
    return gitConfig.get('defaultCloneDirectory') || process.env.USER || process.env.USERNAME || 'vscode-user';
}

function getFileExtension(filePath: string): string {
    const parts = filePath.split('.');
    return parts.length > 1 ? `.${parts[parts.length - 1]}` : '';
}

function generateAnalysisHTML(analysis: any): string {
    return `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Session Analysis</title>
            <style>
                body { font-family: Arial, sans-serif; padding: 20px; }
                .metric { margin: 10px 0; padding: 10px; background: #f5f5f5; border-radius: 5px; }
                .activity { margin: 5px 0; padding: 8px; background: #e8f4fd; border-radius: 3px; }
            </style>
        </head>
        <body>
            <h1>Session Analysis</h1>
            
            <div class="metric">
                <h3>Duration</h3>
                <p>${Math.round(analysis.duration / 60000)} minutes</p>
            </div>
            
            <div class="metric">
                <h3>Primary Intent</h3>
                <p>${analysis.primary_intent || 'Unknown'}</p>
            </div>
            
            <div class="metric">
                <h3>Operations</h3>
                <p>${analysis.operation_count} total operations</p>
            </div>
            
            <div class="metric">
                <h3>Most Active Files</h3>
                ${analysis.most_active_files?.map((file: string) => 
                    `<div class="activity">${file}</div>`
                ).join('') || '<p>No files tracked</p>'}
            </div>
            
            <div class="metric">
                <h3>Activity Breakdown</h3>
                ${Object.entries(analysis.activity_breakdown || {}).map(([activity, count]) =>
                    `<div class="activity">${activity}: ${count}</div>`
                ).join('')}
            </div>
        </body>
        </html>
    `;
}