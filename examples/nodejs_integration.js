#!/usr/bin/env node
/**
 * ContextDB Node.js Integration Example
 * 
 * This example shows how to integrate ContextDB with Node.js applications,
 * including operations management, real-time updates, and search functionality.
 */

const axios = require('axios');
const crypto = require('crypto');

class ContextDBClient {
    constructor(baseURL = 'http://localhost:8080/api/v1', apiKey = null) {
        this.baseURL = baseURL;
        this.client = axios.create({
            baseURL: this.baseURL,
            timeout: 10000,
            headers: {
                'Content-Type': 'application/json',
                ...(apiKey && { 'Authorization': `Bearer ${apiKey}` })
            }
        });

        // Add response interceptor for error handling
        this.client.interceptors.response.use(
            response => response,
            error => {
                console.error('API Error:', error.response?.data || error.message);
                throw error;
            }
        );
    }

    async createOperation(operation) {
        const response = await this.client.post('/operations', operation);
        return response.data;
    }

    async getOperation(operationId) {
        const response = await this.client.get(`/operations/${operationId}`);
        return response.data;
    }

    async listOperations(filters = {}) {
        const response = await this.client.get('/operations', { params: filters });
        return response.data;
    }

    async search(query, options = {}) {
        const params = { q: query, ...options };
        const response = await this.client.get('/search', { params });
        return response.data;
    }

    async analyzeIntent(operationIds) {
        const response = await this.client.post('/analyze/intent', {
            operations: operationIds
        });
        return response.data;
    }

    async getOperationIntent(operationId) {
        const response = await this.client.get(`/operations/${operationId}/intent`);
        return response.data;
    }

    async healthCheck() {
        const response = await this.client.get('/health');
        return response.data;
    }

    // Utility method to create a position hash
    createPositionHash(segments) {
        const content = JSON.stringify(segments);
        return crypto.createHash('sha256').update(content).digest('hex').substring(0, 16);
    }

    // Helper to create insert operation
    createInsertOperation(content, author, documentId, position = 1) {
        return {
            type: 'insert',
            position: {
                segments: [{ value: position, author }],
                hash: this.createPositionHash([{ value: position, author }])
            },
            content,
            author,
            document_id: documentId
        };
    }

    // Helper to create delete operation
    createDeleteOperation(targetId, author) {
        return {
            type: 'delete',
            position: {
                segments: [{ value: Date.now(), author }],
                hash: this.createPositionHash([{ value: Date.now(), author }])
            },
            content: targetId,
            author,
            document_id: 'operations'
        };
    }
}

// Example AI Integration class
class AIContextIntegration {
    constructor(contextClient, aiModel = 'gpt-4') {
        this.context = contextClient;
        this.aiModel = aiModel;
    }

    async captureCodeChange(filePath, beforeContent, afterContent, author) {
        // Calculate diff and create operation
        const operation = this.context.createInsertOperation(
            `Code change in ${filePath}:\n${afterContent}`,
            author,
            filePath,
            Date.now()
        );

        const result = await this.context.createOperation(operation);
        console.log(`Captured code change: ${result.data.id}`);
        return result.data.id;
    }

    async analyzeCodeContext(documentId) {
        // Get all operations for the document
        const operations = await this.context.listOperations({ document_id: documentId });
        
        if (operations.data.length === 0) {
            return { context: 'No operations found', operations: [] };
        }

        // Analyze intent of recent operations
        const recentOps = operations.data.slice(-5).map(op => op.id);
        const intentAnalysis = await this.context.analyzeIntent(recentOps);

        return {
            context: `Document has ${operations.data.length} operations`,
            recent_intent: intentAnalysis.data.collective_intent,
            operations: operations.data.length
        };
    }

    async searchRelevantContext(query) {
        const searchResults = await this.context.search(query, { limit: 10 });
        
        const context = searchResults.data.results.map(result => ({
            type: result.type,
            content: result.content.substring(0, 200) + '...',
            relevance: result.relevance_score
        }));

        return {
            query,
            results_count: searchResults.data.results.length,
            context: context
        };
    }
}

// Example usage and testing
async function main() {
    console.log('🚀 ContextDB Node.js Integration Example');
    
    const client = new ContextDBClient();
    const aiIntegration = new AIContextIntegration(client);

    try {
        // Health check
        const health = await client.healthCheck();
        console.log('✅ Server health:', health.message);

        // Create sample operations
        console.log('\n📝 Creating sample operations...');
        
        const operations = [
            client.createInsertOperation(
                'function calculateSum(a, b) { return a + b; }',
                'nodejs-example',
                'calculator.js',
                1
            ),
            client.createInsertOperation(
                'function calculateProduct(a, b) { return a * b; }',
                'nodejs-example',
                'calculator.js',
                2
            ),
            client.createInsertOperation(
                'console.log("Calculator ready");',
                'nodejs-example',
                'calculator.js',
                3
            )
        ];

        const createdOps = [];
        for (const op of operations) {
            const result = await client.createOperation(op);
            createdOps.push(result.data.id);
            console.log(`  ✅ Created operation: ${result.data.id}`);
        }

        // Search functionality
        console.log('\n🔍 Testing search functionality...');
        const searchResults = await client.search('function');
        console.log(`  Found ${searchResults.data.results.length} results for 'function'`);

        // Intent analysis
        console.log('\n🧠 Analyzing operation intent...');
        const intentAnalysis = await client.analyzeIntent(createdOps);
        console.log(`  Collective intent: ${intentAnalysis.data.collective_intent}`);
        console.log(`  Evidence: ${JSON.stringify(intentAnalysis.data.evidence, null, 2)}`);

        // AI integration example
        console.log('\n🤖 AI Integration Examples...');
        
        const codeContext = await aiIntegration.analyzeCodeContext('calculator.js');
        console.log(`  Code context: ${codeContext.context}`);
        console.log(`  Recent intent: ${codeContext.recent_intent}`);

        const relevantContext = await aiIntegration.searchRelevantContext('calculate');
        console.log(`  Found ${relevantContext.results_count} relevant context items`);
        
        // List all operations for the document
        console.log('\n📋 Listing operations for calculator.js...');
        const allOps = await client.listOperations({ document_id: 'calculator.js' });
        allOps.data.forEach((op, index) => {
            console.log(`  ${index + 1}. ${op.id.substring(0, 8)}... - ${op.content.substring(0, 50)}...`);
        });

        console.log('\n✅ Integration example completed successfully!');
        
    } catch (error) {
        console.error('❌ Example failed:', error.message);
        process.exit(1);
    }
}

// Performance monitoring example
async function performanceTest() {
    console.log('\n⚡ Running performance test...');
    
    const client = new ContextDBClient();
    const startTime = Date.now();
    
    // Create 10 operations concurrently
    const promises = Array.from({ length: 10 }, (_, i) => {
        const op = client.createInsertOperation(
            `Performance test operation ${i}`,
            'perf-test',
            'performance.js',
            i + 1
        );
        return client.createOperation(op);
    });

    const results = await Promise.all(promises);
    const duration = Date.now() - startTime;
    
    console.log(`  ⚡ Created ${results.length} operations in ${duration}ms`);
    console.log(`  📊 Throughput: ${(results.length / duration * 1000).toFixed(2)} ops/sec`);
}

// Run examples if this file is executed directly
if (require.main === module) {
    main()
        .then(() => performanceTest())
        .catch(console.error);
}

module.exports = {
    ContextDBClient,
    AIContextIntegration
};