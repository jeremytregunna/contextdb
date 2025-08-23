import * as vscode from 'vscode';
import { ActivityType } from './contextdb-client';

export interface ClassificationResult {
    activityType: ActivityType;
    confidence: number;
    evidence: string[];
    keywords: string[];
}

export class ActivityClassifier {
    private static readonly patterns = {
        debugging: [
            /console\.log\s*\(/gi,
            /console\.debug\s*\(/gi,
            /console\.error\s*\(/gi,
            /console\.warn\s*\(/gi,
            /print\s*\(/gi,
            /printf\s*\(/gi,
            /println\s*\(/gi,
            /debugger\s*;/gi,
            /breakpoint/gi,
            /debug\s*\(/gi,
            /trace\s*\(/gi,
            /\.debug\s*\(/gi,
            /logger\.debug/gi,
            /log\.debug/gi,
            /System\.out\.print/gi,
            /fmt\.Print/gi,
            /puts\s+/gi
        ],
        
        refactoring: [
            // Function/method extraction
            /(?:function|def|func|method|fn)\s+\w+\s*\(/gi,
            /(?:class|struct|interface|type)\s+\w+/gi,
            /(?:const|let|var)\s+\w+\s*=\s*(?:function|\()/gi,
            /=>\s*{/gi,
            
            // Import/export reorganization
            /^import\s+.*from/gmi,
            /^export\s+/gmi,
            /^from\s+.*import/gmi,
            /^using\s+/gmi,
            /^include\s+/gmi,
            
            // Variable extraction
            /(?:const|let|var|final|static)\s+\w+\s*=/gi,
            
            // Method signature changes
            /\w+\s*\([^)]*\)\s*(?:{|=>)/gi,
            
            // Rename indicators
            /\/\*\s*rename/gi,
            /\/\/\s*rename/gi,
            /TODO:\s*rename/gi
        ],
        
        testing: [
            /(?:test|it|describe|suite|should)\s*\(/gi,
            /expect\s*\(/gi,
            /assert\s*\(/gi,
            /\.toBe\s*\(/gi,
            /\.toEqual\s*\(/gi,
            /\.toHaveBeenCalled/gi,
            /mock\s*\(/gi,
            /spy\s*\(/gi,
            /beforeEach\s*\(/gi,
            /afterEach\s*\(/gi,
            /setUp\s*\(/gi,
            /tearDown\s*\(/gi,
            /@Test/gi,
            /\[Test\]/gi,
            /func\s+Test\w+/gi // Go tests
        ],
        
        documentation: [
            /\/\*\*[\s\S]*?\*\//gi, // JSDoc
            /\/\*[\s\S]*?\*\//gi,   // Block comments
            /\/\/.*$/gmi,           // Line comments
            /#.*$/gmi,              // Python/Shell comments
            /"""[\s\S]*?"""/gi,     // Python docstrings
            /'''[\s\S]*?'''/gi,
            /@param\s+/gi,
            /@return\s+/gi,
            /@throws\s+/gi,
            /README/gi,
            /\.md$/gi,
            /\.rst$/gi,
            /\.txt$/gi
        ],
        
        configuration: [
            /package\.json$/gi,
            /\.config\./gi,
            /\.json$/gi,
            /\.yaml$/gi,
            /\.yml$/gi,
            /\.toml$/gi,
            /\.ini$/gi,
            /\.env$/gi,
            /Dockerfile/gi,
            /docker-compose/gi,
            /\.gitignore$/gi,
            /\.eslintrc/gi,
            /\.prettierrc/gi,
            /tsconfig\.json$/gi,
            /webpack\.config/gi
        ],
        
        apiWork: [
            /fetch\s*\(/gi,
            /axios\./gi,
            /http\./gi,
            /request\s*\(/gi,
            /\.get\s*\(/gi,
            /\.post\s*\(/gi,
            /\.put\s*\(/gi,
            /\.delete\s*\(/gi,
            /async\s+function/gi,
            /await\s+/gi,
            /Promise\./gi,
            /\.then\s*\(/gi,
            /\.catch\s*\(/gi,
            /api\./gi,
            /endpoint/gi,
            /\/api\//gi,
            /REST/gi,
            /GraphQL/gi
        ],
        
        uiWork: [
            /render\s*\(/gi,
            /component/gi,
            /\.jsx?$/gi,
            /\.tsx?$/gi,
            /\.vue$/gi,
            /\.html$/gi,
            /\.css$/gi,
            /\.scss$/gi,
            /\.sass$/gi,
            /\.less$/gi,
            /className\s*=/gi,
            /style\s*=/gi,
            /onClick\s*=/gi,
            /onChange\s*=/gi,
            /useState\s*\(/gi,
            /useEffect\s*\(/gi,
            /\.component\./gi,
            /template/gi,
            /directive/gi
        ],
        
        database: [
            /SELECT\s+/gi,
            /INSERT\s+INTO/gi,
            /UPDATE\s+/gi,
            /DELETE\s+FROM/gi,
            /CREATE\s+TABLE/gi,
            /ALTER\s+TABLE/gi,
            /DROP\s+TABLE/gi,
            /\.sql$/gi,
            /query\s*\(/gi,
            /\.find\s*\(/gi,
            /\.save\s*\(/gi,
            /\.create\s*\(/gi,
            /\.update\s*\(/gi,
            /\.delete\s*\(/gi,
            /mongoose\./gi,
            /sequelize\./gi,
            /knex\./gi,
            /prisma\./gi
        ],
        
        errorHandling: [
            /try\s*{/gi,
            /catch\s*\(/gi,
            /finally\s*{/gi,
            /throw\s+/gi,
            /raises?\s+/gi,
            /except\s+/gi,
            /Error\s*\(/gi,
            /Exception\s*\(/gi,
            /\.error\s*\(/gi,
            /err\s*!=/gi,
            /if\s+err\s*!=/gi, // Go error handling
            /Result<.*,.*>/gi,  // Rust Result type
            /Option<.*>/gi,     // Rust Option type
        ]
    };

    static classifyTextChange(
        change: vscode.TextDocumentContentChangeEvent,
        document: vscode.TextDocument
    ): ActivityType {
        const result = this.classifyTextChangeDetailed(change, document);
        return result.activityType;
    }

    static classifyTextChangeDetailed(
        change: vscode.TextDocumentContentChangeEvent,
        document: vscode.TextDocument
    ): ClassificationResult {
        const text = change.text;
        const fileName = document.fileName;
        const language = document.languageId;
        
        // Get surrounding context (5 lines before and after the change)
        const contextLines = this.getContextLines(document, change.range, 5);
        const fullContext = text + '\n' + contextLines;

        const classifications = this.runClassifications(fullContext, fileName, language);
        
        // Find the classification with the highest confidence
        const topClassification = classifications.reduce((prev, current) => 
            (current.confidence > prev.confidence) ? current : prev
        );

        // If confidence is too low, default to code editing
        if (topClassification.confidence < 0.3) {
            return {
                activityType: ActivityType.CODE_EDIT,
                confidence: 0.5,
                evidence: ['Default classification for code changes'],
                keywords: []
            };
        }

        return topClassification;
    }

    private static runClassifications(text: string, fileName: string, language: string): ClassificationResult[] {
        const results: ClassificationResult[] = [];

        // Check for debugging patterns
        results.push(this.checkPatterns(text, this.patterns.debugging, ActivityType.CODE_DEBUG, 'debugging'));

        // Check for refactoring patterns
        results.push(this.checkPatterns(text, this.patterns.refactoring, ActivityType.CODE_REFACTOR, 'refactoring'));

        // Check for testing patterns
        results.push(this.checkPatterns(text, this.patterns.testing, 'test_create' as ActivityType, 'testing'));

        // Check for documentation patterns
        results.push(this.checkPatterns(text, this.patterns.documentation, 'documentation' as ActivityType, 'documentation'));

        // Check for configuration patterns
        results.push(this.checkPatterns(text, this.patterns.configuration, 'config_change' as ActivityType, 'configuration'));

        // Check for API work patterns
        results.push(this.checkPatterns(text, this.patterns.apiWork, 'api_work' as ActivityType, 'API development'));

        // Check for UI work patterns  
        results.push(this.checkPatterns(text, this.patterns.uiWork, 'ui_work' as ActivityType, 'UI development'));

        // Check for database patterns
        results.push(this.checkPatterns(text, this.patterns.database, 'database_work' as ActivityType, 'database work'));

        // Check for error handling patterns
        results.push(this.checkPatterns(text, this.patterns.errorHandling, 'error_handling' as ActivityType, 'error handling'));

        // File-based classifications
        if (fileName.includes('test') || fileName.includes('spec')) {
            results.push({
                activityType: 'test_create' as ActivityType,
                confidence: 0.7,
                evidence: ['File name indicates test file'],
                keywords: ['test', 'spec']
            });
        }

        if (fileName.toLowerCase().includes('readme') || fileName.endsWith('.md')) {
            results.push({
                activityType: 'documentation' as ActivityType,
                confidence: 0.8,
                evidence: ['File is documentation'],
                keywords: ['readme', 'markdown']
            });
        }

        // Language-based enhancements
        results.forEach(result => {
            result.confidence *= this.getLanguageMultiplier(language, result.activityType);
        });

        return results.filter(result => result.confidence > 0);
    }

    private static checkPatterns(
        text: string, 
        patterns: RegExp[], 
        activityType: ActivityType,
        category: string
    ): ClassificationResult {
        const matches: string[] = [];
        let totalMatches = 0;
        
        patterns.forEach(pattern => {
            const patternMatches = text.match(pattern);
            if (patternMatches) {
                totalMatches += patternMatches.length;
                matches.push(...patternMatches.map(match => match.trim()));
            }
        });

        // Calculate confidence based on number of matches and text length
        const textLength = text.length;
        const matchDensity = totalMatches / Math.max(textLength / 100, 1); // matches per 100 characters
        const confidence = Math.min(matchDensity * 0.3, 0.95); // Cap at 95%

        return {
            activityType,
            confidence,
            evidence: totalMatches > 0 ? [`Found ${totalMatches} ${category} indicators`] : [],
            keywords: matches.slice(0, 5) // Keep top 5 matches
        };
    }

    private static getLanguageMultiplier(language: string, activityType: ActivityType): number {
        const multipliers: { [key: string]: { [key in ActivityType]?: number } } = {
            'javascript': {
                [ActivityType.CODE_DEBUG]: 1.2, // console.log is common
                'ui_work' as ActivityType: 1.3,
                'api_work' as ActivityType: 1.2
            },
            'typescript': {
                [ActivityType.CODE_REFACTOR]: 1.2, // TS encourages refactoring
                'ui_work' as ActivityType: 1.3,
                'api_work' as ActivityType: 1.2
            },
            'python': {
                [ActivityType.CODE_DEBUG]: 1.1, // print statements
                'test_create' as ActivityType: 1.2, // pytest, unittest
                'api_work' as ActivityType: 1.1
            },
            'go': {
                [ActivityType.CODE_DEBUG]: 0.9, // Less common debug patterns
                'error_handling' as ActivityType: 1.3, // Go's explicit error handling
                'api_work' as ActivityType: 1.2
            },
            'rust': {
                [ActivityType.CODE_REFACTOR]: 1.3, // Rust encourages safe refactoring
                'error_handling' as ActivityType: 1.4, // Result and Option types
            },
            'java': {
                [ActivityType.CODE_REFACTOR]: 1.1, // OOP refactoring
                'test_create' as ActivityType: 1.2, // JUnit
            },
            'sql': {
                'database_work' as ActivityType: 1.5
            },
            'html': {
                'ui_work' as ActivityType: 1.4
            },
            'css': {
                'ui_work' as ActivityType: 1.4
            }
        };

        return multipliers[language]?.[activityType] || 1.0;
    }

    private static getContextLines(
        document: vscode.TextDocument,
        range: vscode.Range,
        lineCount: number
    ): string {
        const startLine = Math.max(0, range.start.line - lineCount);
        const endLine = Math.min(document.lineCount - 1, range.end.line + lineCount);
        
        const lines: string[] = [];
        for (let i = startLine; i <= endLine; i++) {
            lines.push(document.lineAt(i).text);
        }
        
        return lines.join('\n');
    }

    // Classify file operations
    static classifyFileOperation(
        operation: 'create' | 'delete' | 'rename',
        filePath: string
    ): ClassificationResult {
        const fileName = filePath.toLowerCase();
        let activityType: ActivityType;
        let confidence = 0.6;
        let evidence: string[] = [];
        let keywords: string[] = [];

        // Determine base activity type
        switch (operation) {
            case 'create':
                activityType = ActivityType.FILE_CREATE;
                break;
            case 'delete':
                activityType = ActivityType.FILE_DELETE;
                break;
            case 'rename':
                activityType = ActivityType.FILE_RENAME;
                break;
        }

        // Enhance classification based on file type
        if (fileName.includes('test') || fileName.includes('spec')) {
            activityType = 'test_create' as ActivityType;
            confidence = 0.8;
            evidence.push('File appears to be a test file');
            keywords.push('test');
        } else if (fileName.endsWith('.md') || fileName.includes('readme')) {
            activityType = 'documentation' as ActivityType;
            confidence = 0.8;
            evidence.push('File is documentation');
            keywords.push('documentation');
        } else if (fileName.includes('config') || fileName.endsWith('.json') || fileName.endsWith('.yaml')) {
            activityType = 'config_change' as ActivityType;
            confidence = 0.7;
            evidence.push('File is configuration');
            keywords.push('config');
        }

        return {
            activityType,
            confidence,
            evidence,
            keywords
        };
    }

    // Classify based on commit messages or git operations
    static classifyGitActivity(commitMessage?: string, operation?: string): ClassificationResult {
        let activityType: ActivityType = ActivityType.GIT_COMMIT;
        let confidence = 0.5;
        let evidence: string[] = [];
        let keywords: string[] = [];

        if (commitMessage) {
            const message = commitMessage.toLowerCase();
            
            if (message.includes('fix') || message.includes('bug')) {
                activityType = 'bug_fix' as ActivityType;
                confidence = 0.8;
                evidence.push('Commit message indicates bug fix');
                keywords.push('fix', 'bug');
            } else if (message.includes('refactor')) {
                activityType = ActivityType.CODE_REFACTOR;
                confidence = 0.8;
                evidence.push('Commit message indicates refactoring');
                keywords.push('refactor');
            } else if (message.includes('test')) {
                activityType = 'test_create' as ActivityType;
                confidence = 0.7;
                evidence.push('Commit message indicates testing work');
                keywords.push('test');
            } else if (message.includes('docs') || message.includes('readme')) {
                activityType = 'documentation' as ActivityType;
                confidence = 0.7;
                evidence.push('Commit message indicates documentation');
                keywords.push('docs');
            } else if (message.includes('feature') || message.includes('add')) {
                activityType = 'feature_development' as ActivityType;
                confidence = 0.7;
                evidence.push('Commit message indicates new feature');
                keywords.push('feature');
            }
        }

        return {
            activityType,
            confidence,
            evidence,
            keywords
        };
    }

    // Classify debug session activities
    static classifyDebugActivity(sessionType: string, action: 'start' | 'end'): ClassificationResult {
        return {
            activityType: action === 'start' ? ActivityType.DEBUG_START : ActivityType.DEBUG_END,
            confidence: 0.9,
            evidence: [`Debug session ${action} for ${sessionType}`],
            keywords: ['debug', sessionType, action]
        };
    }

    // Get human-readable description of activity type
    static getActivityDescription(activityType: ActivityType): string {
        const descriptions: { [key in ActivityType]: string } = {
            [ActivityType.CODE_EDIT]: 'Code editing',
            [ActivityType.CODE_REFACTOR]: 'Code refactoring',
            [ActivityType.CODE_DEBUG]: 'Debugging',
            [ActivityType.FILE_CREATE]: 'File creation',
            [ActivityType.FILE_DELETE]: 'File deletion',
            [ActivityType.FILE_RENAME]: 'File renaming',
            [ActivityType.FILE_SAVE]: 'File saving',
            [ActivityType.GIT_COMMIT]: 'Git commit',
            [ActivityType.GIT_STATUS]: 'Git status change',
            [ActivityType.GIT_BRANCH]: 'Git branch operation',
            [ActivityType.DEBUG_START]: 'Debug session start',
            [ActivityType.DEBUG_END]: 'Debug session end',
            [ActivityType.SESSION_START]: 'Development session start',
            [ActivityType.SESSION_END]: 'Development session end'
        };

        return descriptions[activityType] || activityType.toString().replace('_', ' ');
    }
}