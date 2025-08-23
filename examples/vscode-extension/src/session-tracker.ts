import * as vscode from 'vscode';
import { ContextDBClient, Operation, ActivityType } from './contextdb-client';
import { ActivityClassifier } from './activity-classifier';

export interface SessionStats {
    sessionId: string;
    startTime: number;
    duration: number;
    operationCount: number;
    filesModified: number;
    mostActiveFile: string;
    activityBreakdown: { [key: string]: number };
    languageBreakdown: { [key: string]: number };
    linesAdded: number;
    linesDeleted: number;
    averageOperationInterval: number;
}

export interface SessionAnalysis {
    sessionId: string;
    duration: number;
    operation_count: number;
    primary_intent: string;
    most_active_files: string[];
    activity_breakdown: { [key: string]: number };
    language_breakdown: { [key: string]: number };
    productivity_score: number;
    focus_score: number;
    collaboration_indicators: string[];
}

export class SessionTracker {
    private sessionId: string;
    private sessionStartTime: number;
    private operationHistory: Operation[] = [];
    private fileActivity: Map<string, number> = new Map();
    private activityCounts: Map<ActivityType, number> = new Map();
    private languageCounts: Map<string, number> = new Map();
    private operationTimestamps: number[] = [];
    private globalState: vscode.Memento;
    private contextDB: ContextDBClient;
    private sessionTimer: NodeJS.Timeout | null = null;
    private lastActivityTime: number;

    constructor(contextDB: ContextDBClient, globalState: vscode.Memento) {
        this.contextDB = contextDB;
        this.globalState = globalState;
        this.sessionId = this.generateSessionId();
        this.sessionStartTime = Date.now();
        this.lastActivityTime = this.sessionStartTime;
        
        // Load previous session data if available
        this.loadSessionData();
        
        // Start periodic session tracking
        this.startSessionTimer();
    }

    getSessionId(): string {
        return this.sessionId;
    }

    async startSession(): Promise<void> {
        const sessionOperation: Operation = {
            type: 'insert',
            position: {
                segments: [{
                    value: Date.now(),
                    author: this.getAuthor()
                }],
                hash: `session-start-${this.sessionId}`
            },
            content: `Development session started - VS Code ${vscode.version}`,
            author: this.getAuthor(),
            document_id: 'session',
            metadata: {
                session_id: this.sessionId,
                context: {
                    activity_type: ActivityType.SESSION_START,
                    workspace: vscode.workspace.name,
                    workspace_folders: vscode.workspace.workspaceFolders?.length || 0,
                    open_editors: vscode.window.visibleTextEditors.length,
                    extensions_count: vscode.extensions.all.length,
                    version: vscode.version,
                    platform: process.platform
                }
            }
        };

        try {
            await this.contextDB.createOperation(sessionOperation);
            console.log(`ContextDB: Started tracking session ${this.sessionId}`);
        } catch (error) {
            console.error('Failed to start session tracking:', error);
        }
    }

    trackOperation(operation: Operation): void {
        // Update last activity time
        this.lastActivityTime = Date.now();
        
        // Add to operation history (keep last 1000 operations)
        this.operationHistory.push(operation);
        if (this.operationHistory.length > 1000) {
            this.operationHistory.shift();
        }

        // Track file activity
        const currentCount = this.fileActivity.get(operation.document_id) || 0;
        this.fileActivity.set(operation.document_id, currentCount + 1);

        // Track activity types
        const activityType = this.extractActivityType(operation);
        if (activityType) {
            const currentActivityCount = this.activityCounts.get(activityType) || 0;
            this.activityCounts.set(activityType, currentActivityCount + 1);
        }

        // Track languages
        const language = this.extractLanguage(operation);
        if (language) {
            const currentLangCount = this.languageCounts.get(language) || 0;
            this.languageCounts.set(language, currentLangCount + 1);
        }

        // Track operation timing
        this.operationTimestamps.push(Date.now());
        if (this.operationTimestamps.length > 100) {
            this.operationTimestamps.shift();
        }

        // Save session data periodically
        if (this.operationHistory.length % 10 === 0) {
            this.saveSessionData();
        }
    }

    async getSessionStats(): Promise<SessionStats> {
        const now = Date.now();
        const duration = now - this.sessionStartTime;
        
        // Find most active file
        let mostActiveFile = 'none';
        let maxActivity = 0;
        this.fileActivity.forEach((count, file) => {
            if (count > maxActivity) {
                maxActivity = count;
                mostActiveFile = file;
            }
        });

        // Convert activity counts to breakdown
        const activityBreakdown: { [key: string]: number } = {};
        this.activityCounts.forEach((count, activity) => {
            activityBreakdown[ActivityClassifier.getActivityDescription(activity)] = count;
        });

        // Convert language counts to breakdown
        const languageBreakdown: { [key: string]: number } = {};
        this.languageCounts.forEach((count, language) => {
            languageBreakdown[language] = count;
        });

        // Calculate average operation interval
        let averageInterval = 0;
        if (this.operationTimestamps.length > 1) {
            const intervals = [];
            for (let i = 1; i < this.operationTimestamps.length; i++) {
                intervals.push(this.operationTimestamps[i] - this.operationTimestamps[i-1]);
            }
            averageInterval = intervals.reduce((sum, interval) => sum + interval, 0) / intervals.length;
        }

        return {
            sessionId: this.sessionId,
            startTime: this.sessionStartTime,
            duration,
            operationCount: this.operationHistory.length,
            filesModified: this.fileActivity.size,
            mostActiveFile,
            activityBreakdown,
            languageBreakdown,
            linesAdded: this.calculateLinesAdded(),
            linesDeleted: this.calculateLinesDeleted(),
            averageOperationInterval: averageInterval
        };
    }

    async analyzeCurrentSession(): Promise<SessionAnalysis> {
        const stats = await this.getSessionStats();
        
        // Get operation IDs for analysis
        const operationIds = this.operationHistory
            .filter(op => op.id)
            .map(op => op.id!)
            .slice(-50); // Analyze last 50 operations

        let primaryIntent = 'General development';
        let collaborationIndicators: string[] = [];

        // Analyze intent if we have operations
        if (operationIds.length > 0) {
            try {
                const intentAnalysis = await this.contextDB.analyzeIntent(operationIds);
                primaryIntent = intentAnalysis.collective_intent || intentAnalysis.basic_intent;
            } catch (error) {
                console.warn('Failed to analyze session intent:', error);
            }
        }

        // Calculate productivity score (operations per minute)
        const productivityScore = Math.min(
            (stats.operationCount / (stats.duration / 60000)) * 10,
            100
        );

        // Calculate focus score (based on file switching frequency)
        const focusScore = Math.max(
            100 - (stats.filesModified / stats.operationCount * 100),
            0
        );

        // Detect collaboration indicators
        const uniqueAuthors = new Set(this.operationHistory.map(op => op.author));
        if (uniqueAuthors.size > 1) {
            collaborationIndicators.push(`${uniqueAuthors.size} contributors`);
        }

        // Check for review/comment operations
        const reviewOperations = this.operationHistory.filter(op => 
            op.content.toLowerCase().includes('review') || 
            op.content.toLowerCase().includes('comment') ||
            op.content.toLowerCase().includes('todo')
        );
        if (reviewOperations.length > 0) {
            collaborationIndicators.push(`${reviewOperations.length} review/comment operations`);
        }

        // Get most active files (top 5)
        const sortedFiles = Array.from(this.fileActivity.entries())
            .sort((a, b) => b[1] - a[1])
            .slice(0, 5)
            .map(([file]) => file);

        return {
            sessionId: this.sessionId,
            duration: stats.duration,
            operation_count: stats.operationCount,
            primary_intent: primaryIntent,
            most_active_files: sortedFiles,
            activity_breakdown: stats.activityBreakdown,
            language_breakdown: stats.languageBreakdown,
            productivity_score: Math.round(productivityScore),
            focus_score: Math.round(focusScore),
            collaboration_indicators: collaborationIndicators
        };
    }

    async endSession(): Promise<void> {
        const sessionStats = await this.getSessionStats();
        
        const endOperation: Operation = {
            type: 'insert',
            position: {
                segments: [{
                    value: Date.now(),
                    author: this.getAuthor()
                }],
                hash: `session-end-${this.sessionId}`
            },
            content: `Development session ended - ${sessionStats.operationCount} operations in ${Math.round(sessionStats.duration / 60000)} minutes`,
            author: this.getAuthor(),
            document_id: 'session',
            metadata: {
                session_id: this.sessionId,
                context: {
                    activity_type: ActivityType.SESSION_END,
                    ...sessionStats
                }
            }
        };

        try {
            await this.contextDB.createOperation(endOperation);
            console.log(`ContextDB: Ended session ${this.sessionId} with ${sessionStats.operationCount} operations`);
        } catch (error) {
            console.error('Failed to end session tracking:', error);
        }

        // Stop session timer
        if (this.sessionTimer) {
            clearTimeout(this.sessionTimer);
            this.sessionTimer = null;
        }

        // Save final session data
        this.saveSessionData();
    }

    private startSessionTimer(): void {
        // Create periodic heartbeat to track session activity
        this.sessionTimer = setInterval(() => {
            this.checkSessionActivity();
        }, 60000); // Check every minute
    }

    private async checkSessionActivity(): Promise<void> {
        const now = Date.now();
        const timeSinceLastActivity = now - this.lastActivityTime;
        const fiveMinutes = 5 * 60 * 1000;

        // If no activity for 5 minutes, create an idle indicator
        if (timeSinceLastActivity > fiveMinutes) {
            const idleOperation: Operation = {
                type: 'insert',
                position: {
                    segments: [{
                        value: now,
                        author: this.getAuthor()
                    }],
                    hash: `idle-${this.sessionId}-${now}`
                },
                content: `Session idle for ${Math.round(timeSinceLastActivity / 60000)} minutes`,
                author: this.getAuthor(),
                document_id: 'session',
                metadata: {
                    session_id: this.sessionId,
                    context: {
                        activity_type: 'session_idle' as ActivityType,
                        idle_duration: timeSinceLastActivity
                    }
                }
            };

            try {
                await this.contextDB.createOperation(idleOperation);
            } catch (error) {
                console.error('Failed to track idle time:', error);
            }
        }
    }

    private generateSessionId(): string {
        const timestamp = Date.now().toString(36);
        const random = Math.random().toString(36).substr(2, 5);
        return `vscode-${timestamp}-${random}`;
    }

    private getAuthor(): string {
        // Try to get git user name, fallback to OS user
        const gitExtension = vscode.extensions.getExtension('vscode.git');
        if (gitExtension && gitExtension.isActive) {
            try {
                const gitApi = gitExtension.exports.getAPI(1);
                if (gitApi.repositories.length > 0) {
                    const config = gitApi.repositories[0].repository?.config;
                    if (config) {
                        const userName = config.get('user.name');
                        if (userName) return userName;
                    }
                }
            } catch (error) {
                // Ignore git errors
            }
        }
        
        return process.env.USER || process.env.USERNAME || 'vscode-user';
    }

    private extractActivityType(operation: Operation): ActivityType | null {
        const context = operation.metadata?.context;
        if (context && context.activity_type) {
            return context.activity_type as ActivityType;
        }
        return null;
    }

    private extractLanguage(operation: Operation): string | null {
        const context = operation.metadata?.context;
        if (context && context.language) {
            return context.language;
        }
        
        // Try to infer from file extension
        const ext = operation.document_id.split('.').pop()?.toLowerCase();
        const languageMap: { [key: string]: string } = {
            'js': 'javascript',
            'ts': 'typescript',
            'py': 'python',
            'go': 'go',
            'rs': 'rust',
            'java': 'java',
            'html': 'html',
            'css': 'css',
            'json': 'json',
            'md': 'markdown'
        };
        
        return ext && languageMap[ext] ? languageMap[ext] : null;
    }

    private calculateLinesAdded(): number {
        return this.operationHistory
            .filter(op => op.type === 'insert')
            .reduce((total, op) => {
                const lines = op.content.split('\n').length - 1;
                return total + Math.max(lines, 1);
            }, 0);
    }

    private calculateLinesDeleted(): number {
        return this.operationHistory
            .filter(op => op.type === 'delete')
            .reduce((total, op) => {
                const lines = op.content.split('\n').length - 1;
                return total + Math.max(lines, 1);
            }, 0);
    }

    private saveSessionData(): void {
        const sessionData = {
            sessionId: this.sessionId,
            startTime: this.sessionStartTime,
            operationCount: this.operationHistory.length,
            fileActivity: Array.from(this.fileActivity.entries()),
            activityCounts: Array.from(this.activityCounts.entries()),
            languageCounts: Array.from(this.languageCounts.entries()),
            lastActivityTime: this.lastActivityTime
        };

        this.globalState.update(`contextdb.session.${this.sessionId}`, sessionData);
        this.globalState.update('contextdb.currentSession', this.sessionId);
    }

    private loadSessionData(): void {
        const currentSessionId = this.globalState.get('contextdb.currentSession') as string;
        
        if (currentSessionId) {
            const sessionData = this.globalState.get(`contextdb.session.${currentSessionId}`) as any;
            
            if (sessionData && (Date.now() - sessionData.lastActivityTime) < 4 * 60 * 60 * 1000) { // 4 hours
                // Resume session if last activity was within 4 hours
                this.sessionId = sessionData.sessionId;
                this.sessionStartTime = sessionData.startTime;
                this.fileActivity = new Map(sessionData.fileActivity || []);
                this.activityCounts = new Map(sessionData.activityCounts || []);
                this.languageCounts = new Map(sessionData.languageCounts || []);
                this.lastActivityTime = sessionData.lastActivityTime;
                
                console.log(`ContextDB: Resumed session ${this.sessionId}`);
            }
        }
    }

    // Get session history for analysis
    getSessionHistory(): Operation[] {
        return [...this.operationHistory];
    }

    // Get sessions from storage
    async getStoredSessions(): Promise<string[]> {
        const sessions: string[] = [];
        const keys = this.globalState.keys();
        
        for (const key of keys) {
            if (key.startsWith('contextdb.session.') && key !== 'contextdb.currentSession') {
                const sessionId = key.replace('contextdb.session.', '');
                sessions.push(sessionId);
            }
        }
        
        return sessions.sort().reverse(); // Most recent first
    }

    // Clean up old sessions (keep last 10)
    async cleanupOldSessions(): Promise<void> {
        const sessions = await this.getStoredSessions();
        const sessionsToDelete = sessions.slice(10); // Keep last 10
        
        for (const sessionId of sessionsToDelete) {
            await this.globalState.update(`contextdb.session.${sessionId}`, undefined);
        }
        
        if (sessionsToDelete.length > 0) {
            console.log(`ContextDB: Cleaned up ${sessionsToDelete.length} old sessions`);
        }
    }
}