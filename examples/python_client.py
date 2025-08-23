#!/usr/bin/env python3
"""
ContextDB Python Client Example

This example demonstrates how to integrate with ContextDB from Python applications.
It shows basic operations, authentication, search, and intent analysis.
"""

import requests
import json
from typing import Dict, List, Optional
from dataclasses import dataclass
from datetime import datetime

@dataclass
class Operation:
    type: str
    position: Dict
    content: str
    author: str
    document_id: str
    id: Optional[str] = None

class ContextDBClient:
    def __init__(self, base_url: str = "http://localhost:8080/api/v1", api_key: Optional[str] = None):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        
        if api_key:
            self.session.headers.update({
                'Authorization': f'Bearer {api_key}',
                'Content-Type': 'application/json'
            })

    def create_operation(self, operation: Operation) -> Dict:
        """Create a new operation in ContextDB."""
        data = {
            'type': operation.type,
            'position': operation.position,
            'content': operation.content,
            'author': operation.author,
            'document_id': operation.document_id
        }
        
        response = self.session.post(f'{self.base_url}/operations', json=data)
        response.raise_for_status()
        return response.json()

    def get_operation(self, operation_id: str) -> Dict:
        """Retrieve a specific operation by ID."""
        response = self.session.get(f'{self.base_url}/operations/{operation_id}')
        response.raise_for_status()
        return response.json()

    def list_operations(self, document_id: Optional[str] = None, author: Optional[str] = None, 
                       limit: int = 50, offset: int = 0) -> Dict:
        """List operations with optional filtering."""
        params = {'limit': limit, 'offset': offset}
        if document_id:
            params['document_id'] = document_id
        if author:
            params['author'] = author
            
        response = self.session.get(f'{self.base_url}/operations', params=params)
        response.raise_for_status()
        return response.json()

    def search(self, query: str, content_type: Optional[str] = None, limit: int = 20, offset: int = 0) -> Dict:
        """Search operations and documents."""
        params = {'q': query, 'limit': limit, 'offset': offset}
        if content_type:
            params['type'] = content_type
            
        response = self.session.get(f'{self.base_url}/search', params=params)
        response.raise_for_status()
        return response.json()

    def analyze_intent(self, operation_ids: List[str]) -> Dict:
        """Analyze intent for multiple operations."""
        data = {'operations': operation_ids}
        response = self.session.post(f'{self.base_url}/analyze/intent', json=data)
        response.raise_for_status()
        return response.json()

    def get_operation_intent(self, operation_id: str) -> Dict:
        """Get intent analysis for a single operation."""
        response = self.session.get(f'{self.base_url}/operations/{operation_id}/intent')
        response.raise_for_status()
        return response.json()

    def create_api_key(self, name: str, author_id: str, permissions: List[str], 
                      expires_in: Optional[str] = None) -> Dict:
        """Create a new API key (requires admin permissions)."""
        data = {
            'name': name,
            'author_id': author_id,
            'permissions': permissions
        }
        if expires_in:
            data['expires_in'] = expires_in
            
        response = self.session.post(f'{self.base_url}/auth/keys', json=data)
        response.raise_for_status()
        return response.json()

    def health_check(self) -> Dict:
        """Check server health status."""
        response = self.session.get(f'{self.base_url}/health')
        response.raise_for_status()
        return response.json()

def main():
    """Example usage of the ContextDB client."""
    # Initialize client (add API key if authentication is enabled)
    client = ContextDBClient()
    
    # Check server health
    health = client.health_check()
    print(f"Server status: {health}")
    
    # Create a sample operation
    operation = Operation(
        type="insert",
        position={
            "segments": [{"value": 1, "author": "python-example"}],
            "hash": f"python-{datetime.now().isoformat()}"
        },
        content=f"Python integration test - {datetime.now()}",
        author="python-example",
        document_id="python-test.py"
    )
    
    try:
        # Create operation
        result = client.create_operation(operation)
        operation_id = result['data']['id']
        print(f"Created operation: {operation_id}")
        
        # Retrieve the operation
        retrieved = client.get_operation(operation_id)
        print(f"Retrieved operation: {retrieved['data']['content']}")
        
        # Search for operations
        search_results = client.search("Python")
        print(f"Search found {len(search_results['data']['results'])} results")
        
        # Analyze intent
        intent = client.get_operation_intent(operation_id)
        print(f"Operation intent: {intent['data']['basic_intent']}")
        
        # List operations for this document
        ops = client.list_operations(document_id="python-test.py")
        print(f"Found {len(ops['data'])} operations for python-test.py")
        
    except requests.exceptions.RequestException as e:
        print(f"API error: {e}")
    except KeyError as e:
        print(f"Unexpected response format: {e}")

if __name__ == "__main__":
    main()