import { ContextDBClient, Operation } from './contextdb-client';

export class OperationBatcher {
    private batch: Operation[] = [];
    private batchTimeout: NodeJS.Timeout | null = null;
    private flushPromise: Promise<void> | null = null;
    private readonly maxBatchSize: number;
    private readonly flushInterval: number;
    private readonly contextDB: ContextDBClient;
    private isFlushingBatch = false;

    constructor(contextDB: ContextDBClient, maxBatchSize: number = 10, flushInterval: number = 5000) {
        this.contextDB = contextDB;
        this.maxBatchSize = maxBatchSize;
        this.flushInterval = flushInterval;
    }

    async addOperation(operation: Operation): Promise<void> {
        // Add timestamp if not present
        if (!operation.timestamp) {
            operation.timestamp = new Date().toISOString();
        }

        this.batch.push(operation);

        // Immediate flush if batch size exceeded
        if (this.batch.length >= this.maxBatchSize) {
            await this.flushBatch();
            return;
        }

        // Schedule delayed flush
        this.scheduleFlush();
    }

    private scheduleFlush(): void {
        // Clear existing timeout
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
        }

        // Schedule new flush
        this.batchTimeout = setTimeout(async () => {
            await this.flushBatch();
        }, this.flushInterval);
    }

    async flushBatch(): Promise<void> {
        // Prevent concurrent flushes
        if (this.isFlushingBatch) {
            if (this.flushPromise) {
                await this.flushPromise;
            }
            return;
        }

        // No operations to flush
        if (this.batch.length === 0) {
            return;
        }

        // Clear timeout since we're flushing now
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
            this.batchTimeout = null;
        }

        // Mark as flushing and create promise
        this.isFlushingBatch = true;
        this.flushPromise = this.performFlush();

        try {
            await this.flushPromise;
        } finally {
            this.isFlushingBatch = false;
            this.flushPromise = null;
        }
    }

    private async performFlush(): Promise<void> {
        const operationsToFlush = [...this.batch];
        this.batch = []; // Clear batch immediately

        if (operationsToFlush.length === 0) {
            return;
        }

        try {
            console.log(`ContextDB: Flushing ${operationsToFlush.length} operations`);

            // Attempt to send all operations
            const results = await this.contextDB.createBatchOperations(operationsToFlush);
            
            console.log(`ContextDB: Successfully sent ${results.length}/${operationsToFlush.length} operations`);

            // If some operations failed, they're logged by the client
            if (results.length < operationsToFlush.length) {
                console.warn(`ContextDB: ${operationsToFlush.length - results.length} operations failed to send`);
            }

        } catch (error) {
            console.error('ContextDB: Failed to flush operations:', error);
            
            // Decide whether to retry or drop operations
            if (this.shouldRetryOperations(operationsToFlush, error)) {
                // Add failed operations back to the beginning of the batch for retry
                this.batch.unshift(...operationsToFlush.slice(0, Math.min(operationsToFlush.length, 5))); // Limit retries
                console.log(`ContextDB: Queued ${Math.min(operationsToFlush.length, 5)} operations for retry`);
            } else {
                console.warn('ContextDB: Dropping failed operations to prevent infinite retry');
            }
        }
    }

    private shouldRetryOperations(operations: Operation[], error: any): boolean {
        // Don't retry if it's an authentication error
        if (error.message?.includes('401') || error.message?.includes('Unauthorized')) {
            return false;
        }

        // Don't retry if it's a client error (4xx)
        if (error.message?.includes('400') || error.message?.includes('Bad Request')) {
            return false;
        }

        // Don't retry very old operations (older than 5 minutes)
        const fiveMinutesAgo = Date.now() - 5 * 60 * 1000;
        const hasOldOperations = operations.some(op => {
            const timestamp = op.timestamp ? new Date(op.timestamp).getTime() : Date.now();
            return timestamp < fiveMinutesAgo;
        });

        if (hasOldOperations) {
            return false;
        }

        // Retry for network errors, server errors (5xx)
        return true;
    }

    getBatchInfo(): BatchInfo {
        return {
            pendingOperations: this.batch.length,
            isFlushScheduled: this.batchTimeout !== null,
            isFlushing: this.isFlushingBatch,
            maxBatchSize: this.maxBatchSize,
            flushInterval: this.flushInterval
        };
    }

    // Immediately flush and wait for completion - useful for extension deactivation
    async forceFlush(): Promise<void> {
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
            this.batchTimeout = null;
        }
        
        await this.flushBatch();
    }

    // Clear all pending operations without sending them
    clearBatch(): void {
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
            this.batchTimeout = null;
        }
        
        const cleared = this.batch.length;
        this.batch = [];
        
        if (cleared > 0) {
            console.log(`ContextDB: Cleared ${cleared} pending operations`);
        }
    }

    // Add multiple operations at once
    async addBatch(operations: Operation[]): Promise<void> {
        for (const operation of operations) {
            // Add timestamp if not present
            if (!operation.timestamp) {
                operation.timestamp = new Date().toISOString();
            }
            
            this.batch.push(operation);
        }

        // Flush if we've exceeded the batch size
        if (this.batch.length >= this.maxBatchSize) {
            await this.flushBatch();
        } else {
            this.scheduleFlush();
        }
    }

    // Update configuration
    updateConfig(maxBatchSize: number, flushInterval: number): void {
        this.maxBatchSize = maxBatchSize;
        this.flushInterval = flushInterval;
        
        // Reschedule flush with new interval if there's a pending flush
        if (this.batchTimeout && this.batch.length > 0) {
            this.scheduleFlush();
        }
    }
}

export interface BatchInfo {
    pendingOperations: number;
    isFlushScheduled: boolean;
    isFlushing: boolean;
    maxBatchSize: number;
    flushInterval: number;
}

// Utility class for operation deduplication
export class OperationDeduplicator {
    private recentOperations = new Map<string, number>();
    private readonly maxAge = 60000; // 1 minute
    private readonly cleanupInterval: NodeJS.Timeout;

    constructor() {
        // Clean up old entries every 30 seconds
        this.cleanupInterval = setInterval(() => {
            this.cleanup();
        }, 30000);
    }

    isDuplicate(operation: Operation): boolean {
        const key = this.getOperationKey(operation);
        const now = Date.now();
        
        const lastSeen = this.recentOperations.get(key);
        if (lastSeen && (now - lastSeen) < this.maxAge) {
            return true;
        }
        
        this.recentOperations.set(key, now);
        return false;
    }

    private getOperationKey(operation: Operation): string {
        // Create a key based on operation characteristics
        return `${operation.type}:${operation.document_id}:${operation.content.substring(0, 50)}:${operation.author}`;
    }

    private cleanup(): void {
        const now = Date.now();
        const toDelete: string[] = [];
        
        this.recentOperations.forEach((timestamp, key) => {
            if (now - timestamp > this.maxAge) {
                toDelete.push(key);
            }
        });
        
        toDelete.forEach(key => {
            this.recentOperations.delete(key);
        });
    }

    dispose(): void {
        clearInterval(this.cleanupInterval);
        this.recentOperations.clear();
    }
}

// Smart batching that groups related operations
export class SmartOperationBatcher extends OperationBatcher {
    private documentBatches = new Map<string, Operation[]>();

    async addOperation(operation: Operation): Promise<void> {
        // Group operations by document
        if (!this.documentBatches.has(operation.document_id)) {
            this.documentBatches.set(operation.document_id, []);
        }
        
        this.documentBatches.get(operation.document_id)!.push(operation);
        
        // Check if any document batch is ready to flush
        const totalOperations = Array.from(this.documentBatches.values())
            .reduce((sum, batch) => sum + batch.length, 0);
        
        if (totalOperations >= this.maxBatchSize) {
            await this.flushBatch();
        } else {
            this.scheduleFlush();
        }
    }

    protected async performFlush(): Promise<void> {
        if (this.documentBatches.size === 0) {
            return;
        }

        // Collect all operations from all documents
        const allOperations: Operation[] = [];
        this.documentBatches.forEach(operations => {
            allOperations.push(...operations);
        });

        // Clear document batches
        this.documentBatches.clear();

        if (allOperations.length === 0) {
            return;
        }

        try {
            console.log(`ContextDB Smart Batcher: Flushing ${allOperations.length} operations from ${this.documentBatches.size} documents`);
            
            const results = await this.contextDB.createBatchOperations(allOperations);
            console.log(`ContextDB Smart Batcher: Successfully sent ${results.length}/${allOperations.length} operations`);

        } catch (error) {
            console.error('ContextDB Smart Batcher: Failed to flush operations:', error);
            
            // On error, reorganize failed operations back by document
            if (this.shouldRetryOperations(allOperations, error)) {
                const retryOperations = allOperations.slice(0, Math.min(allOperations.length, 5));
                retryOperations.forEach(op => {
                    if (!this.documentBatches.has(op.document_id)) {
                        this.documentBatches.set(op.document_id, []);
                    }
                    this.documentBatches.get(op.document_id)!.push(op);
                });
            }
        }
    }

    getBatchInfo(): BatchInfo & { documentBatches: number } {
        const totalOperations = Array.from(this.documentBatches.values())
            .reduce((sum, batch) => sum + batch.length, 0);
            
        return {
            ...super.getBatchInfo(),
            pendingOperations: totalOperations,
            documentBatches: this.documentBatches.size
        };
    }
}