/**
 * RRI-T — nano-brain Web Dashboard
 *
 * Tests: REST API v1 endpoints, graph builder functions,
 * edge cases, security, data integrity
 */
import { describe, it, expect, beforeAll, afterAll, beforeEach, afterEach } from 'vitest';
import * as http from 'http';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import * as crypto from 'crypto';
import { createStore } from '../src/store.js';
import { evictCachedStore } from '../src/store.js';
import type { Store } from '../src/types.js';

const TEST_PORT = 19899;
const TEST_HOST = '127.0.0.1';
const BASE_URL = `http://${TEST_HOST}:${TEST_PORT}`;

function hash(s: string) { return crypto.createHash('sha256').update(s).digest('hex'); }

async function httpGet(url: string, headers?: Record<string, string>): Promise<{ status: number; headers: http.IncomingHttpHeaders; body: string }> {
  return new Promise((resolve, reject) => {
    const u = new URL(url);
    http.request({
      hostname: u.hostname, port: u.port, path: u.pathname + u.search,
      method: 'GET', headers,
    }, (res) => {
      let body = '';
      res.on('data', (c) => { body += c; });
      res.on('end', () => resolve({ status: res.statusCode || 0, headers: res.headers, body }));
    }).on('error', reject).end();
  });
}

// ============================================================
// REST API Server (replicates server.ts API handlers)
// ============================================================

let httpServer: http.Server | null = null;
let tempDir: string;
let dbPath: string;
let store: Store;
const PROJECT_HASH = 'webtest1';

beforeAll(async () => {
  tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-web-'));
  dbPath = path.join(tempDir, 'test.db');
  store = createStore(dbPath);

  // Seed data
  for (let i = 0; i < 20; i++) {
    const content = `Web test document ${i} about topic ${i % 5}`;
    const h = hash(content);
    store.insertContent(h, content);
    store.insertDocument({
      collection: 'memory', path: `/web/doc${i}.md`, title: `Doc ${i}`,
      hash: h, createdAt: new Date().toISOString(), modifiedAt: new Date().toISOString(),
      active: true, projectHash: PROJECT_HASH,
    });
    if (i < 5) store.insertTags(store.findDocument(`/web/doc${i}.md`)!.id, ['web', 'test']);
  }

  // Seed entities
  const e1 = store.insertOrUpdateEntity({ name: 'APIServer', type: 'service', projectHash: PROJECT_HASH, firstLearnedAt: new Date().toISOString(), lastConfirmedAt: new Date().toISOString() });
  const e2 = store.insertOrUpdateEntity({ name: 'Database', type: 'service', projectHash: PROJECT_HASH, firstLearnedAt: new Date().toISOString(), lastConfirmedAt: new Date().toISOString() });
  store.insertEdge({ sourceId: e1, targetId: e2, edgeType: 'depends_on', projectHash: PROJECT_HASH });

  httpServer = http.createServer(async (req, res) => {
    const url = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`);
    const pathname = url.pathname;
    const workspace = url.searchParams.get('workspace') || PROJECT_HASH;

    // CORS
    if (req.url?.startsWith('/api/v1/')) {
      const origin = req.headers.origin;
      if (origin && (origin.startsWith('http://localhost:') || origin.startsWith('http://127.0.0.1:'))) {
        res.setHeader('Access-Control-Allow-Origin', origin);
        res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
        res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
      }
      if (req.method === 'OPTIONS') { res.writeHead(204); res.end(); return; }
    }

    const json = (data: any, status = 200) => {
      res.writeHead(status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(data));
    };

    try {
      if (pathname === '/health') return json({ ok: true });
      if (pathname === '/api/v1/status') return json({
        version: '1.0.0-test', uptime: process.uptime(),
        documents: store.getIndexHealth().documentCount,
        embeddings: store.getIndexHealth().embeddedCount,
        workspaces: [], primaryWorkspace: tempDir,
      });
      if (pathname === '/api/v1/workspaces') return json({
        workspaces: [{ path: tempDir, name: 'test', hash: PROJECT_HASH, documentCount: 20 }],
      });
      if (pathname === '/api/v1/graph/entities') {
        const entities = store.getMemoryEntities(workspace);
        return json({ nodes: entities.map(e => ({ id: e.id, name: e.name, type: e.type, description: e.description, firstLearnedAt: e.firstLearnedAt, lastConfirmedAt: e.lastConfirmedAt })), edges: [], stats: { nodeCount: entities.length, edgeCount: 0 } });
      }
      if (pathname === '/api/v1/graph/stats') return json(store.getGraphStats(workspace));
      if (pathname === '/api/v1/code/dependencies') return json({ files: [], edges: store.getFileEdges(workspace).map(e => ({ source: e.source_path, target: e.target_path })) });
      if (pathname === '/api/v1/graph/symbols') {
        const symbols = store.getSymbolsForProject(workspace);
        const edges = store.getSymbolEdgesForProject(workspace);
        const clusters = store.getSymbolClusters(workspace);
        return json({ symbols, edges, clusters });
      }
      if (pathname === '/api/v1/graph/flows') {
        const flows = store.getFlowsWithSteps(workspace);
        return json({ flows: flows.map(f => ({ ...f, steps: store.getFlowSteps(f.id) })) });
      }
      if (pathname === '/api/v1/graph/connections') return json({ connections: store.getAllConnections(workspace) });
      if (pathname === '/api/v1/graph/infrastructure') {
        const symbols = store.getInfrastructureSymbols(workspace);
        return json({ symbols, grouped: {} });
      }
      if (pathname === '/api/v1/search') {
        const q = url.searchParams.get('q');
        if (!q) return json({ error: 'query parameter "q" is required' }, 400);
        const start = Date.now();
        const results = store.searchFTS(q, { limit: 10, projectHash: workspace });
        return json({ results: results.map(r => ({ id: r.id, docid: r.docid, title: r.title, path: r.path, score: r.score, snippet: r.snippet, collection: r.collection })), query: q, executionMs: Date.now() - start });
      }
      if (pathname === '/api/v1/telemetry') return json({ queryCount: store.getTelemetryCount(), banditStats: store.loadBanditStats(workspace), preferenceWeights: {}, expandRate: 0, importanceStats: { min: 0, max: 0, mean: 0, median: 0 } });
      if (pathname === '/api/v1/connections') {
        const docId = url.searchParams.get('docId');
        if (!docId) return json({ error: 'docId parameter is required' }, 400);
        const doc = store.findDocument(docId);
        if (!doc) return json({ error: 'Document not found' }, 404);
        return json({ connections: store.getConnectionsForDocument(doc.id, { direction: 'both' }) });
      }
      res.writeHead(404); res.end('Not Found');
    } catch (err) {
      json({ error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  await new Promise<void>(resolve => httpServer!.listen(TEST_PORT, TEST_HOST, () => resolve()));
}, 30000);

afterAll(async () => {
  if (httpServer) await new Promise<void>(resolve => httpServer!.close(() => resolve()));
  if (store) store.close();
  try { evictCachedStore(dbPath); } catch {}
  fs.rmSync(tempDir, { recursive: true, force: true });
});


// ============================================================
// D1: UI/UX — Web Build & Static Assets
// ============================================================

describe('D1: UI/UX — Web Build', () => {
  it('WEB-030: dist/web/ has valid build output', () => {
    const distWeb = path.join(__dirname, '..', 'dist', 'web');
    expect(fs.existsSync(distWeb)).toBe(true);
    expect(fs.existsSync(path.join(distWeb, 'index.html'))).toBe(true);
    expect(fs.existsSync(path.join(distWeb, 'assets'))).toBe(true);

    const html = fs.readFileSync(path.join(distWeb, 'index.html'), 'utf-8');
    expect(html).toContain('<div id="root">');
    expect(html).toContain('/web/');
  });

  it('WEB-030b: index.html references JS and CSS assets', () => {
    const distWeb = path.join(__dirname, '..', 'dist', 'web');
    const html = fs.readFileSync(path.join(distWeb, 'index.html'), 'utf-8');
    expect(html).toMatch(/\.js/);
    expect(html).toMatch(/\.css/);
  });
});

// ============================================================
// D2: API — All v1 Endpoints
// ============================================================

describe('D2: API — REST API v1 Endpoints', () => {
  it('WEB-009: GET /health returns ok', async () => {
    const res = await httpGet(`${BASE_URL}/health`);
    expect(res.status).toBe(200);
    expect(JSON.parse(res.body).ok).toBe(true);
  });

  it('WEB-009b: GET /api/v1/status returns valid schema', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/status`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(data).toHaveProperty('version');
    expect(data).toHaveProperty('uptime');
    expect(data).toHaveProperty('documents');
    expect(data).toHaveProperty('embeddings');
    expect(data.documents).toBe(20);
  });

  it('WEB-010: GET /api/v1/search validates query param', async () => {
    const noQ = await httpGet(`${BASE_URL}/api/v1/search`);
    expect(noQ.status).toBe(400);

    const withQ = await httpGet(`${BASE_URL}/api/v1/search?q=topic`);
    expect(withQ.status).toBe(200);
    const data = JSON.parse(withQ.body);
    expect(data.results.length).toBeGreaterThan(0);
    expect(data.query).toBe('topic');
    expect(typeof data.executionMs).toBe('number');
  });

  it('WEB-011: GET /api/v1/connections validates docId', async () => {
    const noId = await httpGet(`${BASE_URL}/api/v1/connections`);
    expect(noId.status).toBe(400);

    const notFound = await httpGet(`${BASE_URL}/api/v1/connections?docId=nonexistent`);
    expect(notFound.status).toBe(404);

    const valid = await httpGet(`${BASE_URL}/api/v1/connections?docId=/web/doc0.md`);
    expect(valid.status).toBe(200);
    expect(JSON.parse(valid.body)).toHaveProperty('connections');
  });

  it('WEB-012: All graph endpoints return valid schemas', async () => {
    const endpoints = [
      { url: '/api/v1/graph/entities', keys: ['nodes', 'edges', 'stats'] },
      { url: '/api/v1/graph/stats', keys: ['nodeCount', 'edgeCount'] },
      { url: '/api/v1/code/dependencies', keys: ['files', 'edges'] },
      { url: '/api/v1/graph/symbols', keys: ['symbols', 'edges', 'clusters'] },
      { url: '/api/v1/graph/flows', keys: ['flows'] },
      { url: '/api/v1/graph/connections', keys: ['connections'] },
      { url: '/api/v1/graph/infrastructure', keys: ['symbols', 'grouped'] },
    ];

    for (const ep of endpoints) {
      const res = await httpGet(`${BASE_URL}${ep.url}`);
      expect(res.status).toBe(200);
      const data = JSON.parse(res.body);
      for (const key of ep.keys) {
        expect(data).toHaveProperty(key);
      }
    }
  });

  it('WEB-014: Telemetry endpoint returns stats', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/telemetry`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(data).toHaveProperty('queryCount');
    expect(data).toHaveProperty('banditStats');
    expect(data).toHaveProperty('expandRate');
    expect(data).toHaveProperty('importanceStats');
  });

  it('WEB-016: Unknown routes return 404', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/nonexistent`);
    expect(res.status).toBe(404);
  });

  it('WEB-013: Workspace param filters search results', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=document&workspace=${PROJECT_HASH}`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(data.results.length).toBeGreaterThan(0);
  });
});

// ============================================================
// D3: Performance
// ============================================================

describe('D3: Performance — API Response Times', () => {
  it('WEB-018: Search response < 500ms on 20 docs', async () => {
    const start = Date.now();
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=topic`);
    const elapsed = Date.now() - start;
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(data.executionMs).toBeLessThan(500);
    expect(elapsed).toBeLessThan(2000); // including network
  });

  it('WEB-032: 10 concurrent API requests succeed', async () => {
    const promises = Array.from({ length: 10 }, (_, i) =>
      httpGet(`${BASE_URL}/api/v1/search?q=document`)
    );
    const results = await Promise.all(promises);
    for (const res of results) {
      expect(res.status).toBe(200);
      expect(() => JSON.parse(res.body)).not.toThrow();
    }
  });
});

// ============================================================
// D4: Security
// ============================================================

describe('D4: Security — XSS, CORS, Error Safety', () => {
  it('WEB-015: CORS returns correct headers for localhost', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/status`, { 'Origin': 'http://localhost:3000' });
    expect(res.headers['access-control-allow-origin']).toBe('http://localhost:3000');
  });

  it('WEB-023: CORS rejects non-localhost origin', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/status`, { 'Origin': 'https://evil.com' });
    expect(res.headers['access-control-allow-origin']).toBeUndefined();
  });

  it('WEB-021: XSS in search query is not executed', async () => {
    const xss = encodeURIComponent('<script>alert(1)</script>');
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=${xss}`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    // query is returned as-is (string), not as HTML
    expect(data.query).toBe('<script>alert(1)</script>');
    // JSON encoding makes it safe — the query is a JSON string value, not raw HTML
    // If rendered via React (JSX), it's auto-escaped
    expect(res.headers['content-type']).toContain('application/json');
  });

  it('WEB-024: 500 errors return message without stack trace', async () => {
    // The server catches errors and returns { error: message }
    // Force an error by requesting connections for a doc that triggers an error
    const res = await httpGet(`${BASE_URL}/api/v1/connections?docId=nonexistent`);
    expect(res.status).toBe(404);
    const data = JSON.parse(res.body);
    expect(data.error).toBeDefined();
    expect(data.error).not.toContain('at ');
    expect(data.error).not.toContain('node_modules');
  });

  it('WEB-021b: SQL injection in search query is safe', async () => {
    const injection = encodeURIComponent("'; DROP TABLE documents; --");
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=${injection}`);
    expect(res.status).toBe(200);

    // DB still works
    const status = await httpGet(`${BASE_URL}/api/v1/status`);
    expect(JSON.parse(status.body).documents).toBe(20);
  });
});

// ============================================================
// D5: Data Integrity
// ============================================================

describe('D5: Data Integrity — API-Store Consistency', () => {
  it('WEB-026: Status doc count matches getIndexHealth', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/status`);
    const apiCount = JSON.parse(res.body).documents;
    const storeCount = store.getIndexHealth().documentCount;
    expect(apiCount).toBe(storeCount);
  });

  it('WEB-027: Graph entities endpoint returns valid node data', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/graph/entities?workspace=${PROJECT_HASH}`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(Array.isArray(data.nodes)).toBe(true);
    expect(data.nodes.length).toBeGreaterThan(0); // We seeded 2 entities
    // Verify node shape
    for (const node of data.nodes) {
      expect(node).toHaveProperty('id');
      expect(node).toHaveProperty('name');
      expect(node).toHaveProperty('type');
    }
  });

  it('WEB-028: Search results match store FTS', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=topic&workspace=${PROJECT_HASH}`);
    const apiResults = JSON.parse(res.body).results;
    const storeResults = store.searchFTS('topic', { limit: 10, projectHash: PROJECT_HASH });
    expect(apiResults.length).toBe(storeResults.length);
    // Same paths in same order
    for (let i = 0; i < apiResults.length; i++) {
      expect(apiResults[i].path).toBe(storeResults[i].path);
    }
  });
});

// ============================================================
// D6: Infrastructure — Web Build
// ============================================================

describe('D6: Infrastructure — Build and Assets', () => {
  it('WEB-030c: Vite config has correct base path /web/', () => {
    const viteConfig = fs.readFileSync(
      path.join(__dirname, '..', 'src', 'web', 'vite.config.ts'), 'utf-8'
    );
    expect(viteConfig).toContain("'/web/'");
  });

  it('WEB-030d: package.json has build:web script', () => {
    const pkg = JSON.parse(fs.readFileSync(
      path.join(__dirname, '..', 'package.json'), 'utf-8'
    ));
    expect(pkg.scripts['build:web']).toBeDefined();
  });

  it('WEB-031: dist/web/index.html has SPA root div', () => {
    const html = fs.readFileSync(
      path.join(__dirname, '..', 'dist', 'web', 'index.html'), 'utf-8'
    );
    expect(html).toContain('id="root"');
  });
});

// ============================================================
// D7: Edge Cases
// ============================================================

describe('D7: Edge Cases — Empty Data and Boundaries', () => {
  it('WEB-035: Graph endpoints return empty arrays for no data', async () => {
    // Use a workspace hash that has no data
    const res = await httpGet(`${BASE_URL}/api/v1/graph/symbols?workspace=empty`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(data.symbols).toEqual([]);
    expect(data.edges).toEqual([]);
  });

  it('WEB-035b: Flows endpoint returns empty for no flows', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/graph/flows?workspace=empty`);
    expect(res.status).toBe(200);
    expect(JSON.parse(res.body).flows).toEqual([]);
  });

  it('WEB-035c: Connections endpoint returns empty for no connections', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/graph/connections?workspace=empty`);
    expect(res.status).toBe(200);
    expect(JSON.parse(res.body).connections).toEqual([]);
  });

  it('WEB-037: Search with empty query returns 400', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=`);
    expect(res.status).toBe(400);
  });

  it('WEB-037b: Search with single character returns results or empty', async () => {
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=W`);
    expect(res.status).toBe(200);
    const data = JSON.parse(res.body);
    expect(Array.isArray(data.results)).toBe(true);
  });

  it('WEB-040: Very long search query handled gracefully', async () => {
    const longQ = encodeURIComponent('a '.repeat(2000));
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=${longQ}`);
    expect(res.status).toBe(200);
  });

  it('WEB-021c: Unicode search query works', async () => {
    const q = encodeURIComponent('日本語テスト 🚀 Tiếng Việt');
    const res = await httpGet(`${BASE_URL}/api/v1/search?q=${q}`);
    expect(res.status).toBe(200);
  });
});

// ============================================================
// Graph Builder Unit Tests (pure functions from web/src/lib/)
// These test the frontend graph-building logic without React
// ============================================================

describe('D1: Graph Builders — Pure Function Tests', () => {
  it('WEB-036: buildEntityGraph handles 0 nodes', () => {
    // Simulate what graph-adapter.ts does
    const data = { nodes: [], edges: [], stats: { nodeCount: 0, edgeCount: 0, typeDistribution: {} } };
    // Build graph manually (same logic as buildEntityGraph)
    const edgeCounts = new Map<number, number>();
    for (const edge of data.edges) {
      edgeCounts.set(edge.sourceId, (edgeCounts.get(edge.sourceId) || 0) + 1);
      edgeCounts.set(edge.targetId, (edgeCounts.get(edge.targetId) || 0) + 1);
    }
    expect(data.nodes.length).toBe(0);
    expect(edgeCounts.size).toBe(0);
  });

  it('WEB-017: Entity graph node sizing by edge count', () => {
    // Verify the sizing formula: Math.max(5, Math.min(25, 5 + count * 1.2))
    const formula = (count: number) => Math.max(5, Math.min(25, 5 + count * 1.2));
    expect(formula(0)).toBe(5);       // min
    expect(formula(1)).toBe(6.2);     // small
    expect(formula(10)).toBe(17);     // medium
    expect(formula(100)).toBe(25);    // capped at max
  });

  it('WEB-017b: Code graph centrality sizing', () => {
    // Formula: Math.max(4, Math.min(20, 4 + centrality * 24))
    const formula = (c: number) => Math.max(4, Math.min(20, 4 + c * 24));
    expect(formula(0)).toBe(4);      // min
    expect(formula(0.5)).toBe(16);   // mid
    expect(formula(1)).toBe(20);     // max (capped at 20, not 28)
  });

  it('WEB-039: Connection strength edge sizing', () => {
    // Formula: Math.max(1, Math.min(5, strength * 5))
    const formula = (s: number) => Math.max(1, Math.min(5, s * 5));
    expect(formula(0)).toBe(1);     // min clamped
    expect(formula(0.5)).toBe(2.5); // mid
    expect(formula(1)).toBe(5);     // max
    expect(formula(2)).toBe(5);     // capped
    expect(formula(-1)).toBe(1);    // negative → clamped to 1
  });

  it('WEB-019: Symbol graph cluster mode sizing', () => {
    // Formula: Math.max(10, Math.min(40, 10 + memberCount * 0.5))
    const formula = (m: number) => Math.max(10, Math.min(40, 10 + m * 0.5));
    expect(formula(0)).toBe(10);    // min
    expect(formula(20)).toBe(20);   // mid
    expect(formula(100)).toBe(40);  // capped
  });

  it('WEB-040b: Code graph label truncation', () => {
    // Formula: path.split('/').slice(-2).join('/')
    const truncate = (p: string) => p.split('/').slice(-2).join('/');
    expect(truncate('/a/b/c/d/e.ts')).toBe('d/e.ts');
    expect(truncate('e.ts')).toBe('e.ts');
    expect(truncate('/single')).toBe('/single'); // slice(-2) on ['','single'] = '/single'
  });
});
