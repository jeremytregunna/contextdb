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
    
    // Enhanced text document changes with debouncing
    if (config.get('captureTextChanges', true)) {
        const changeBuffer = new Map<string, any[]>();
        const debounceTimers = new Map<string, NodeJS.Timeout>();
        const originalContent = new Map<string, string[]>();
        
        const textChangeHandler = vscode.workspace.onDidChangeTextDocument(async (event) => {
            if (!isEnabled || !shouldCaptureDocument(event.document)) return;
            
            const documentUri = event.document.uri.toString();
            
            // Initialize original content tracking
            if (!originalContent.has(documentUri)) {
                originalContent.set(documentUri, event.document.getText().split('\n'));
            }
            
            // Accumulate changes
            if (!changeBuffer.has(documentUri)) {
                changeBuffer.set(documentUri, []);
            }
            changeBuffer.get(documentUri)!.push({
                document: event.document,
                changes: event.contentChanges,
                timestamp: Date.now()
            });
            
            // Clear existing timer
            const existingTimer = debounceTimers.get(documentUri);
            if (existingTimer) {
                clearTimeout(existingTimer);
            }
            
            // Set new debounce timer
            const timer = setTimeout(async () => {
                await processAccumulatedChanges(
                    documentUri,
                    changeBuffer.get(documentUri) || [],
                    originalContent
                );
                changeBuffer.delete(documentUri);
                debounceTimers.delete(documentUri);
            }, 2000); // 2 second debounce
            
            debounceTimers.set(documentUri, timer);
        });
        
        // Process accumulated changes with meaningful content detection
        async function processAccumulatedChanges(
            documentUri: string,
            changes: any[],
            contentTracker: Map<string, string[]>
        ) {
            if (changes.length === 0) return;
            
            const lastChange = changes[changes.length - 1];
            const document = lastChange.document;
            
            const originalLines = contentTracker.get(documentUri) || [];
            const currentLines = document.getText().split('\n');
            
            // Detect meaningful changes
            const changeInfo = detectMeaningfulChange(originalLines, currentLines);
            
            if (changeInfo) {
                const operation = createEnhancedTextChangeOperation(
                    document,
                    changeInfo,
                    changes.length
                );
                
                await batcher.addOperation(operation);
                sessionTracker.trackOperation(operation);
            }
            
            // Update content tracker
            contentTracker.set(documentUri, [...currentLines]);
        }
        
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
function detectMeaningfulChange(original: string[], current: string[]) {
    const linesDeleted = Math.max(0, original.length - current.length);
    const linesAdded = Math.max(0, current.length - original.length);
    
    // Skip if no meaningful change
    if (linesDeleted === 0 && linesAdded === 0) {
        // Check for content changes within lines
        const hasContentChange = original.some((line, i) => line !== (current[i] || ''));
        if (!hasContentChange) return null;
    }
    
    // Pure deletion
    if (linesDeleted > 0 && linesAdded === 0) {
        const deletedLines = findDeletedLines(original, current);
        return {
            operationType: 'delete' as const,
            content: {
                type: 'delete',
                deleted: deletedLines.join('\n')
            },
            linesDeleted,
            linesAdded: 0
        };
    }
    
    // Pure addition
    if (linesAdded > 0 && linesDeleted === 0) {
        const addedLines = findAddedLines(original, current);
        return {
            operationType: 'insert' as const,
            content: {
                type: 'insert',
                added: addedLines.join('\n')
            },
            linesDeleted: 0,
            linesAdded
        };
    }
    
    // Mixed changes or replacement
    const { oldContent, newContent } = findChangedContent(original, current);
    return {
        operationType: 'insert' as const,
        content: {
            type: 'replace',
            old: oldContent.join('\n'),
            new: newContent.join('\n')
        },
        linesDeleted,
        linesAdded
    };
}

function findDeletedLines(original: string[], current: string[]): string[] {
    const deleted: string[] = [];
    let commonPrefix = 0;
    
    // Find common prefix
    for (let i = 0; i < Math.min(original.length, current.length); i++) {
        if (original[i] === current[i]) {
            commonPrefix = i + 1;
        } else {
            break;
        }
    }
    
    // Extract deleted lines
    const deletedCount = original.length - current.length;
    for (let i = commonPrefix; i < commonPrefix + deletedCount; i++) {
        if (original[i]) {
            deleted.push(original[i]);
        }
    }
    
    return deleted;
}

function findAddedLines(original: string[], current: string[]): string[] {
    const added: string[] = [];
    let commonPrefix = 0;
    
    // Find common prefix
    for (let i = 0; i < Math.min(original.length, current.length); i++) {
        if (original[i] === current[i]) {
            commonPrefix = i + 1;
        } else {
            break;
        }
    }
    
    // Extract added lines
    const addedCount = current.length - original.length;
    for (let i = commonPrefix; i < commonPrefix + addedCount; i++) {
        if (current[i]) {
            added.push(current[i]);
        }
    }
    
    return added;
}

function findChangedContent(original: string[], current: string[]) {
    // Find first difference
    let firstDiff = 0;
    for (let i = 0; i < Math.min(original.length, current.length); i++) {
        if (original[i] !== current[i]) {
            firstDiff = i;
            break;
        }
    }
    
    // Find last difference
    let lastDiffOld = original.length - 1;
    let lastDiffNew = current.length - 1;
    
    for (let i = 0; i < Math.min(original.length, current.length); i++) {
        const oldIdx = original.length - 1 - i;
        const newIdx = current.length - 1 - i;
        if (original[oldIdx] !== current[newIdx]) {
            lastDiffOld = oldIdx;
            lastDiffNew = newIdx;
            break;
        }
    }
    
    return {
        oldContent: original.slice(firstDiff, lastDiffOld + 1),
        newContent: current.slice(firstDiff, lastDiffNew + 1)
    };
}

function createEnhancedTextChangeOperation(
    document: vscode.TextDocument,
    changeInfo: any,
    changeCount: number
): Operation {
    const position = {
        segments: [{
            value: Date.now(),
            author: getAuthor()
        }],
        hash: `${document.fileName}-${Date.now()}`
    };
    
    return {
        type: changeInfo.operationType,
        position,
        content: JSON.stringify(changeInfo.content),
        content_type: 'json',
        author: getAuthor(),
        document_id: document.fileName,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                language: document.languageId,
                file_size: document.getText().length.toString(),
                line_count: document.lineCount.toString(),
                lines_deleted: changeInfo.linesDeleted.toString(),
                lines_added: changeInfo.linesAdded.toString(),
                change_count: changeCount.toString(),
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
    
    const content = {
        type: 'session',
        event: `file_${operation}`,
        message: `File ${operation}: ${filePath}${additional?.old_path ? ` (from ${additional.old_path})` : ''}`
    };
    
    return {
        type: 'insert',
        position,
        content: JSON.stringify(content),
        content_type: 'json',
        author: getAuthor(),
        document_id: filePath,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                operation_type: `file_${operation}`,
                file_type: getFileExtension(filePath),
                old_path: additional?.old_path || '',
                workspace: vscode.workspace.name || 'unknown'
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
    
    const content = {
        type: 'session',
        event: 'file_save',
        message: `Saved: ${document.fileName}`
    };
    
    return {
        type: 'insert',
        position,
        content: JSON.stringify(content),
        content_type: 'json',
        author: getAuthor(),
        document_id: document.fileName,
        metadata: {
            session_id: sessionTracker.getSessionId(),
            context: {
                operation_type: 'file_save',
                language: document.languageId,
                file_size: document.getText().length.toString(),
                line_count: document.lineCount.toString(),
                workspace: vscode.workspace.name || 'unknown'
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