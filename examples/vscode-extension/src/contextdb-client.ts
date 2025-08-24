import axios, { AxiosInstance, AxiosResponse } from 'axios';

export interface Operation {
    id?: string;
    type: 'insert' | 'delete';
    position: Position;
    content: string;
    content_type?: string;
    author: string;
    document_id: string;
    timestamp?: string;
    parents?: string[];
    metadata?: {
        session_id?: string;
        context?: Record<string, any>;
    };
}

export interface Position {
    segments: Segment[];
    hash: string;
}

export interface Segment {
    value: number;
    author: string;
}

export interface APIResponse<T = any> {
    success: boolean;
    data?: T;
    message?: string;
    error?: string;
    code?: string;
}

export interface SearchResult {
    results: SearchItem[];
    total: number;
}

export interface SearchItem {
    type: string;
    id: string;
    content: string;
    author: string;
    document_id: string;
    timestamp: string;
    relevance_score: number;
}

export interface IntentAnalysis {
    basic_intent: string;
    collective_intent?: string;
    evidence: Record<string, any>;
}

export enum ActivityType {
    CODE_EDIT = 'code_edit',
    CODE_REFACTOR = 'code_refactor',
    CODE_DEBUG = 'code_debug',
    FILE_CREATE = 'file_create',
    FILE_DELETE = 'file_delete',
    FILE_RENAME = 'file_rename',
    FILE_SAVE = 'file_save',
    GIT_COMMIT = 'git_commit',
    GIT_STATUS = 'git_status',
    GIT_BRANCH = 'git_branch',
    DEBUG_START = 'debug_start',
    DEBUG_END = 'debug_end',
    SESSION_START = 'session_start',
    SESSION_END = 'session_end'
}

export class ContextDBClient {
    private client: AxiosInstance;
    private baseUrl: string;
    private apiKey: string;

    constructor(baseUrl: string, apiKey: string = '') {
        this.baseUrl = baseUrl.replace(/\/$/, '');
        this.apiKey = apiKey;
        
        this.client = axios.create({
            baseURL: this.baseUrl,
            timeout: 10000,
            headers: {
                'Content-Type': 'application/json',
                ...(apiKey && { 'Authorization': `Bearer ${apiKey}` })
            }
        });

        // Response interceptor for error handling
        this.client.interceptors.response.use(
            (response) => response,
            (error) => {
                console.error('ContextDB API Error:', error.response?.data || error.message);
                throw new Error(`ContextDB API Error: ${error.response?.data?.error || error.message}`);
            }
        );
    }

    async createOperation(operation: Operation): Promise<Operation> {
        try {
            const response: AxiosResponse<APIResponse<Operation>> = await this.client.post('/operations', operation);
            
            if (!response.data.success) {
                throw new Error(response.data.error || 'Failed to create operation');
            }
            
            return response.data.data!;
        } catch (error) {
            console.error('Failed to create operation:', error);
            throw error;
        }
    }

    async createBatchOperations(operations: Operation[]): Promise<Operation[]> {
        // Since the API doesn't have a batch endpoint, send them individually in parallel
        try {
            const promises = operations.map(op => this.createOperation(op));
            const results = await Promise.allSettled(promises);
            
            const successful: Operation[] = [];
            const failed: string[] = [];
            
            results.forEach((result, index) => {
                if (result.status === 'fulfilled') {
                    successful.push(result.value);
                } else {
                    failed.push(`Operation ${index}: ${result.reason?.message}`);
                }
            });
            
            if (failed.length > 0) {
                console.warn(`Batch operation warnings: ${failed.join(', ')}`);
            }
            
            return successful;
        } catch (error) {
            console.error('Failed to create batch operations:', error);
            throw error;
        }
    }

    async getOperation(operationId: string): Promise<Operation> {
        try {
            const response: AxiosResponse<APIResponse<Operation>> = await this.client.get(`/operations/${operationId}`);
            
            if (!response.data.success) {
                throw new Error(response.data.error || 'Failed to get operation');
            }
            
            return response.data.data!;
        } catch (error) {
            console.error('Failed to get operation:', error);
            throw error;
        }
    }

    async listOperations(filters: {
        document_id?: string;
        author?: string;
        limit?: number;
        offset?: number;
    } = {}): Promise<Operation[]> {
        try {
            const params = new URLSearchParams();
            if (filters.document_id) params.append('document_id', filters.document_id);
            if (filters.author) params.append('author', filters.author);
            if (filters.limit) params.append('limit', filters.limit.toString());
            if (filters.offset) params.append('offset', filters.offset.toString());
            
            const response: AxiosResponse<APIResponse<Operation[]>> = await this.client.get(`/operations?${params}`);
            
            if (!response.data.success) {
                throw new Error(response.data.error || 'Failed to list operations');
            }
            
            return response.data.data!;
        } catch (error) {
            console.error('Failed to list operations:', error);
            throw error;
        }
    }

    async search(query: string, options: {
        type?: string;
        limit?: number;
        offset?: number;
    } = {}): Promise<SearchResult> {
        try {
            const params = new URLSearchParams();
            params.append('q', query);
            if (options.type) params.append('type', options.type);
            if (options.limit) params.append('limit', options.limit.toString());
            if (options.offset) params.append('offset', options.offset.toString());
            
            const response: AxiosResponse<APIResponse<SearchResult>> = await this.client.get(`/search?${params}`);
            
            if (!response.data.success) {
                throw new Error(response.data.error || 'Failed to search');
            }
            
            return response.data.data!;
        } catch (error) {
            console.error('Failed to search:', error);
            throw error;
        }
    }

    async analyzeIntent(operationIds: string[]): Promise<IntentAnalysis> {
        try {
            const response: AxiosResponse<APIResponse<IntentAnalysis>> = await this.client.post('/analyze/intent', {
                operations: operationIds
            });
            
            if (!response.data.success) {
                throw new Error(response.data.error || 'Failed to analyze intent');
            }
            
            return response.data.data!;
        } catch (error) {
            console.error('Failed to analyze intent:', error);
            throw error;
        }
    }

    async getOperationIntent(operationId: string): Promise<IntentAnalysis> {
        try {
            const response: AxiosResponse<APIResponse<IntentAnalysis>> = await this.client.get(`/operations/${operationId}/intent`);
            
            if (!response.data.success) {
                throw new Error(response.data.error || 'Failed to get operation intent');
            }
            
            return response.data.data!;
        } catch (error) {
            console.error('Failed to get operation intent:', error);
            throw error;
        }
    }

    async healthCheck(): Promise<boolean> {
        try {
            const response: AxiosResponse<APIResponse> = await this.client.get('/health');
            return response.data.success;
        } catch (error) {
            console.error('Health check failed:', error);
            return false;
        }
    }

    // Helper methods
    createPosition(value: number, author: string, hash?: string): Position {
        return {
            segments: [{ value, author }],
            hash: hash || `${author}-${value}-${Date.now()}`
        };
    }

    createInsertOperation(
        content: string,
        author: string,
        documentId: string,
        positionValue?: number,
        metadata?: Operation['metadata']
    ): Operation {
        const position = this.createPosition(
            positionValue || Date.now() + Math.random() * 1000,
            author
        );

        return {
            type: 'insert',
            position,
            content,
            author,
            document_id: documentId,
            metadata
        };
    }

    createDeleteOperation(
        content: string,
        author: string,
        documentId: string,
        positionValue?: number,
        metadata?: Operation['metadata']
    ): Operation {
        const position = this.createPosition(
            positionValue || Date.now() + Math.random() * 1000,
            author
        );

        return {
            type: 'delete',
            position,
            content,
            author,
            document_id: documentId,
            metadata
        };
    }
}

// Utility functions
export function generateOperationId(operation: Operation): string {
    // Simple ID generation based on operation content
    const content = JSON.stringify({
        type: operation.type,
        content: operation.content,
        author: operation.author,
        document_id: operation.document_id,
        timestamp: operation.timestamp || new Date().toISOString()
    });
    
    // Simple hash function (in real implementation, use proper crypto)
    let hash = 0;
    for (let i = 0; i < content.length; i++) {
        const char = content.charCodeAt(i);
        hash = ((hash << 5) - hash) + char;
        hash = hash & hash; // Convert to 32-bit integer
    }
    
    return Math.abs(hash).toString(16);
}

export function isTemporaryFile(filePath: string): boolean {
    const tempPatterns = [
        /\/tmp\//,
        /\\temp\\/,
        /\.tmp$/,
        /\.temp$/,
        /~$/,
        /\.swp$/,
        /\.swo$/
    ];
    
    return tempPatterns.some(pattern => pattern.test(filePath));
}

export function getFileLanguage(filePath: string): string {
    const ext = filePath.split('.').pop()?.toLowerCase();
    
    const languageMap: { [key: string]: string } = {
        'js': 'javascript',
        'ts': 'typescript',
        'jsx': 'javascriptreact',
        'tsx': 'typescriptreact',
        'py': 'python',
        'rb': 'ruby',
        'go': 'go',
        'rs': 'rust',
        'java': 'java',
        'c': 'c',
        'cpp': 'cpp',
        'h': 'c',
        'hpp': 'cpp',
        'cs': 'csharp',
        'php': 'php',
        'html': 'html',
        'css': 'css',
        'scss': 'scss',
        'sass': 'sass',
        'less': 'less',
        'json': 'json',
        'xml': 'xml',
        'yaml': 'yaml',
        'yml': 'yaml',
        'md': 'markdown',
        'sh': 'shellscript',
        'bash': 'shellscript',
        'zsh': 'shellscript',
        'fish': 'shellscript',
        'ps1': 'powershell',
        'sql': 'sql'
    };
    
    return languageMap[ext || ''] || 'plaintext';
}