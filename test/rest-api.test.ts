import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import * as http from 'http';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import { createStore } from '../src/store.js';
import type { Store } from '../src/types.js';

const TEST_PORT = 19877;
const TEST_HOST = '127.0.0.1';
const BASE_URL = `http://${TEST_HOST}:${TEST_PORT}`;

let httpServer: http.Server | null = null;
let tempDir: string;
let dbPath: string;
let store: Store;

async function fetch(url: string, options: { method?: string; headers?: Record<string, string> } = {}): Promise<{ status: number; headers: http.IncomingHttpHeaders; body: string }> {
  return new Promise((resolve, reject) => {
    const urlObj = new URL(url);
    const req = http.request({
      hostname: urlObj.hostname,
      port: urlObj.port,
      path: urlObj.pathname + urlObj.search,
      method: options.method || 'GET',
      headers: options.headers,
    }, (res) => {
      let body = '';
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => {
        resolve({ status: res.statusCode || 0, headers: res.headers, body });
      });
    });
    req.on('error', reject);
    req.end();
  });
}

describe('REST API v1', () => {
  beforeAll(async () => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nano-brain-rest-test-'));
    dbPath = path.join(tempDir, 'test.sqlite');
    
    store = createStore(dbPath);
    store.insertContent('hash1', 'Test document content for searching');
    store.insertDocument({
      collection: 'test',
      path: '/test/doc1.md',
      title: 'Test Doc 1',
      hash: 'hash1',
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'testproject1',
    });
    
    const currentProjectHash = 'testproject1';
    const resolvedWorkspaceRoot = tempDir;

    httpServer = http.createServer(async (req, res) => {
      const url = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`);
      const pathname = url.pathname;

      if (req.url?.startsWith('/api/v1/') || req.url?.startsWith('/web/') || req.url === '/web') {
        const origin = req.headers.origin;
        if (!origin || origin.startsWith('http://localhost:') || origin.startsWith('http://127.0.0.1:')) {
          res.setHeader('Access-Control-Allow-Origin', origin || '*');
          res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
          res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
        }
        if (req.method === 'OPTIONS') {
          res.writeHead(204);
          res.end();
          return;
        }
      }

      if (req.method === 'GET' && pathname === '/api/v1/status') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          version: '1.0.0-test',
          uptime: process.uptime(),
          documents: store.getIndexHealth().documentCount,
          embeddings: store.getIndexHealth().embeddedCount,
          workspaces: [],
          primaryWorkspace: resolvedWorkspaceRoot,
        }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/workspaces') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          workspaces: [{
            path: resolvedWorkspaceRoot,
            name: path.basename(resolvedWorkspaceRoot),
            hash: currentProjectHash,
            documentCount: store.getIndexHealth().documentCount,
          }],
        }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/graph/entities') {
        const entities = store.getMemoryEntities(currentProjectHash);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          nodes: entities.map(e => ({
            id: e.id,
            name: e.name,
            type: e.type,
          })),
          edges: [],
          stats: { nodeCount: entities.length, edgeCount: 0, typeDistribution: {} },
        }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/graph/stats') {
        const stats = store.getGraphStats(currentProjectHash);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(stats));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/code/dependencies') {
        const edges = store.getFileEdges(currentProjectHash);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          files: [],
          edges: edges.map(e => ({ source: e.source_path, target: e.target_path })),
        }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/search') {
        const query = url.searchParams.get('q');
        if (!query) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'query parameter "q" is required' }));
          return;
        }
        const startTime = Date.now();
        const results = store.searchFTS(query, { limit: 10, projectHash: currentProjectHash });
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          results: results.map(r => ({
            id: r.id,
            docid: r.docid,
            title: r.title,
            path: r.path,
            score: r.score,
            snippet: r.snippet,
            collection: r.collection,
          })),
          query,
          executionMs: Date.now() - startTime,
        }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/telemetry') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          queryCount: store.getTelemetryCount(),
          banditStats: store.loadBanditStats(currentProjectHash),
          preferenceWeights: {},
          expandRate: 0,
          importanceStats: { min: 0, max: 0, mean: 0, median: 0 },
        }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/connections') {
        const docId = url.searchParams.get('docId');
        if (!docId) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'docId parameter is required' }));
          return;
        }
        const doc = store.findDocument(docId);
        if (!doc) {
          res.writeHead(404, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'Document not found' }));
          return;
        }
        const connections = store.getConnectionsForDocument(doc.id, { direction: 'both' });
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ connections }));
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/graph/symbols') {
        try {
          const symbols = store.getSymbolsForProject(currentProjectHash);
          const edges = store.getSymbolEdgesForProject(currentProjectHash);
          const clusters = store.getSymbolClusters(currentProjectHash);
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ symbols, edges, clusters }));
        } catch (err) {
          res.writeHead(500, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: err instanceof Error ? err.message : String(err) }));
        }
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/graph/flows') {
        try {
          const flows = store.getFlowsWithSteps(currentProjectHash);
          const flowsWithSteps = flows.map(flow => ({
            ...flow,
            steps: store.getFlowSteps(flow.id),
          }));
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ flows: flowsWithSteps }));
        } catch (err) {
          res.writeHead(500, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: err instanceof Error ? err.message : String(err) }));
        }
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/graph/connections') {
        try {
          const connections = store.getAllConnections(currentProjectHash);
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ connections }));
        } catch (err) {
          res.writeHead(500, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: err instanceof Error ? err.message : String(err) }));
        }
        return;
      }

      if (req.method === 'GET' && pathname === '/api/v1/graph/infrastructure') {
        try {
          const symbols = store.getInfrastructureSymbols(currentProjectHash);
          const grouped: Record<string, Array<{
            pattern: string;
            operations: Array<{ op: string; repo: string; file: string; line: number }>;
          }>> = {};
          for (const sym of symbols) {
            if (!grouped[sym.type]) grouped[sym.type] = [];
            let patternEntry = grouped[sym.type].find(p => p.pattern === sym.pattern);
            if (!patternEntry) {
              patternEntry = { pattern: sym.pattern, operations: [] };
              grouped[sym.type].push(patternEntry);
            }
            patternEntry.operations.push({
              op: sym.operation,
              repo: sym.repo,
              file: sym.filePath,
              line: sym.lineNumber,
            });
          }
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ symbols, grouped }));
        } catch (err) {
          res.writeHead(500, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: err instanceof Error ? err.message : String(err) }));
        }
        return;
      }

      res.writeHead(404);
      res.end('Not Found');
    });

    await new Promise<void>((resolve) => {
      httpServer!.listen(TEST_PORT, TEST_HOST, () => resolve());
    });
  }, 30000);

  afterAll(async () => {
    if (httpServer) {
      await new Promise<void>((resolve) => httpServer!.close(() => resolve()));
    }
    if (store) {
      store.close();
    }
    try {
      fs.rmSync(tempDir, { recursive: true, force: true });
    } catch {}
  });

  describe('CORS', () => {
    it('should return CORS headers for /api/v1/ routes', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/status`, {
        headers: { 'Origin': 'http://localhost:3000' },
      });
      expect(res.headers['access-control-allow-origin']).toBe('http://localhost:3000');
      expect(res.headers['access-control-allow-methods']).toBe('GET, POST, OPTIONS');
    });

    it('should handle OPTIONS preflight request', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/status`, {
        method: 'OPTIONS',
        headers: { 'Origin': 'http://localhost:3000' },
      });
      expect(res.status).toBe(204);
    });
  });

  describe('GET /api/v1/status', () => {
    it('should return system status as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/status`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('version');
      expect(data).toHaveProperty('uptime');
      expect(typeof data.uptime).toBe('number');
      expect(data).toHaveProperty('documents');
      expect(data).toHaveProperty('embeddings');
      expect(data).toHaveProperty('workspaces');
      expect(data).toHaveProperty('primaryWorkspace');
    });
  });

  describe('GET /api/v1/workspaces', () => {
    it('should return workspace list as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/workspaces`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('workspaces');
      expect(Array.isArray(data.workspaces)).toBe(true);
      expect(data.workspaces.length).toBeGreaterThan(0);
      expect(data.workspaces[0]).toHaveProperty('path');
      expect(data.workspaces[0]).toHaveProperty('name');
      expect(data.workspaces[0]).toHaveProperty('hash');
      expect(data.workspaces[0]).toHaveProperty('documentCount');
    });
  });

  describe('GET /api/v1/graph/entities', () => {
    it('should return graph entities as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/entities`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('nodes');
      expect(data).toHaveProperty('edges');
      expect(data).toHaveProperty('stats');
      expect(Array.isArray(data.nodes)).toBe(true);
      expect(Array.isArray(data.edges)).toBe(true);
      expect(data.stats).toHaveProperty('nodeCount');
      expect(data.stats).toHaveProperty('edgeCount');
    });
  });

  describe('GET /api/v1/graph/stats', () => {
    it('should return graph stats as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/stats`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('nodeCount');
      expect(data).toHaveProperty('edgeCount');
      expect(data).toHaveProperty('clusterCount');
    });
  });

  describe('GET /api/v1/code/dependencies', () => {
    it('should return code dependencies as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/code/dependencies`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('files');
      expect(data).toHaveProperty('edges');
      expect(Array.isArray(data.files)).toBe(true);
      expect(Array.isArray(data.edges)).toBe(true);
    });
  });

  describe('GET /api/v1/search', () => {
    it('should return 400 when query is missing', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/search`);
      expect(res.status).toBe(400);
      const data = JSON.parse(res.body);
      expect(data.error).toContain('required');
    });

    it('should return search results as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/search?q=test`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('results');
      expect(data).toHaveProperty('query');
      expect(data).toHaveProperty('executionMs');
      expect(Array.isArray(data.results)).toBe(true);
      expect(data.query).toBe('test');
      expect(typeof data.executionMs).toBe('number');
    });

    it('should find indexed documents', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/search?q=document`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data.results.length).toBeGreaterThan(0);
    });
  });

  describe('GET /api/v1/telemetry', () => {
    it('should return telemetry data as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/telemetry`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('queryCount');
      expect(data).toHaveProperty('banditStats');
      expect(data).toHaveProperty('preferenceWeights');
      expect(data).toHaveProperty('expandRate');
      expect(data).toHaveProperty('importanceStats');
      expect(typeof data.queryCount).toBe('number');
    });
  });

  describe('GET /api/v1/connections', () => {
    it('should return 400 when docId is missing', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/connections`);
      expect(res.status).toBe(400);
      const data = JSON.parse(res.body);
      expect(data.error).toContain('docId');
    });

    it('should return 404 for non-existent document', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/connections?docId=nonexistent`);
      expect(res.status).toBe(404);
    });

    it('should return connections for existing document', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/connections?docId=/test/doc1.md`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('connections');
      expect(Array.isArray(data.connections)).toBe(true);
    });
  });

  describe('GET /api/v1/graph/symbols', () => {
    it('should return symbols, edges, and clusters as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/symbols`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('symbols');
      expect(data).toHaveProperty('edges');
      expect(data).toHaveProperty('clusters');
      expect(Array.isArray(data.symbols)).toBe(true);
      expect(Array.isArray(data.edges)).toBe(true);
      expect(Array.isArray(data.clusters)).toBe(true);
    });

    it('should return empty arrays when no symbols exist', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/symbols`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data.symbols).toEqual([]);
      expect(data.edges).toEqual([]);
      expect(data.clusters).toEqual([]);
    });
  });

  describe('GET /api/v1/graph/flows', () => {
    it('should return flows as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/flows`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('flows');
      expect(Array.isArray(data.flows)).toBe(true);
    });

    it('should return empty array when no flows exist', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/flows`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data.flows).toEqual([]);
    });
  });

  describe('GET /api/v1/graph/connections', () => {
    it('should return connections as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/connections`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('connections');
      expect(Array.isArray(data.connections)).toBe(true);
    });

    it('should return empty array when no connections exist', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/connections`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data.connections).toEqual([]);
    });
  });

  describe('GET /api/v1/graph/infrastructure', () => {
    it('should return infrastructure symbols as JSON', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/infrastructure`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data).toHaveProperty('symbols');
      expect(data).toHaveProperty('grouped');
      expect(Array.isArray(data.symbols)).toBe(true);
      expect(typeof data.grouped).toBe('object');
    });

    it('should return empty data when no infrastructure symbols exist', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/graph/infrastructure`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      expect(data.symbols).toEqual([]);
      expect(data.grouped).toEqual({});
    });
  });

  describe('404 handling', () => {
    it('should return 404 for unknown routes', async () => {
      const res = await fetch(`${BASE_URL}/api/v1/unknown`);
      expect(res.status).toBe(404);
    });
  });
});
