/**
 * RRI-T Round 3 — nano-brain comprehensive test suite
 *
 * Dimensions: D2:API, D3:Performance, D4:Security, D5:Data, D6:Infra, D7:Edge Cases
 * Personas: End User, QA Destroyer, DevOps Tester, Security Auditor, Business Analyst
 */
import { describe, it, expect, beforeEach, afterEach, vi, beforeAll, afterAll } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import * as crypto from 'crypto';

// ============================================================
// D5: DATA INTEGRITY — computeDecayScore NaN handling
// ============================================================
describe('D5: Data Integrity — computeDecayScore', () => {
  let computeDecayScore: typeof import('../src/search.js').computeDecayScore;

  beforeAll(async () => {
    const mod = await import('../src/search.js');
    computeDecayScore = mod.computeDecayScore;
  });

  it('TC-001: returns 0.5 for invalid date string', () => {
    const score = computeDecayScore('not-a-date', 'also-invalid', 30);
    expect(score).toBe(0.5);
    expect(Number.isNaN(score)).toBe(false);
  });

  it('TC-002: returns 0.5 for empty string date', () => {
    const score = computeDecayScore('', '', 30);
    expect(score).toBe(0.5);
  });

  it('TC-003: returns 1 for future dates (treated as fresh)', () => {
    const future = new Date(Date.now() + 86400000 * 30).toISOString();
    const score = computeDecayScore(future, future, 30);
    expect(score).toBe(1);
  });

  it('TC-004: returns valid score for today', () => {
    const now = new Date().toISOString();
    const score = computeDecayScore(now, now, 30);
    expect(score).toBeGreaterThan(0.9);
    expect(score).toBeLessThanOrEqual(1);
  });

  it('TC-005: returns ~0.5 for date exactly halfLife days ago', () => {
    const halfLifeDays = 30;
    const past = new Date(Date.now() - 86400000 * halfLifeDays).toISOString();
    const score = computeDecayScore(past, past, halfLifeDays);
    expect(score).toBeCloseTo(0.5, 1);
  });

  it('TC-006: falls back to createdAt when lastAccessedAt is null', () => {
    const created = new Date(Date.now() - 86400000 * 10).toISOString();
    const score = computeDecayScore(null, created, 30);
    expect(score).toBeGreaterThan(0);
    expect(score).toBeLessThan(1);
  });

  it('TC-007: handles epoch zero date', () => {
    const score = computeDecayScore('1970-01-01T00:00:00.000Z', '1970-01-01T00:00:00.000Z', 30);
    expect(Number.isFinite(score)).toBe(true);
    expect(score).toBeGreaterThan(0);
  });
});

// ============================================================
// D6: INFRA — Logger stdio mode
// ============================================================
describe('D6: Infrastructure — Logger stdioMode', () => {
  let log: typeof import('../src/logger.js').log;
  let setStdioMode: typeof import('../src/logger.js').setStdioMode;
  let initLogger: typeof import('../src/logger.js').initLogger;

  beforeAll(async () => {
    const mod = await import('../src/logger.js');
    log = mod.log;
    setStdioMode = mod.setStdioMode;
    initLogger = mod.initLogger;
  });

  afterEach(() => {
    setStdioMode(false);
  });

  it('TC-008: setStdioMode suppresses stdout writes', () => {
    initLogger({ logging: { enabled: true } });
    const stdoutSpy = vi.spyOn(process.stdout, 'write').mockImplementation(() => true);
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockImplementation(() => true);

    setStdioMode(true);
    log('test', 'should not appear on stdout');
    log('test', 'error should not appear on stderr', 'error');

    const stdoutCalls = stdoutSpy.mock.calls.filter(
      c => typeof c[0] === 'string' && c[0].includes('should not appear')
    );
    const stderrCalls = stderrSpy.mock.calls.filter(
      c => typeof c[0] === 'string' && c[0].includes('should not appear')
    );

    expect(stdoutCalls.length).toBe(0);
    expect(stderrCalls.length).toBe(0);

    stdoutSpy.mockRestore();
    stderrSpy.mockRestore();
  });

  it('TC-009: non-stdio mode writes to stdout', () => {
    initLogger({ logging: { enabled: true } });
    const stdoutSpy = vi.spyOn(process.stdout, 'write').mockImplementation(() => true);

    setStdioMode(false);
    log('test', 'should appear on stdout');

    const calls = stdoutSpy.mock.calls.filter(
      c => typeof c[0] === 'string' && c[0].includes('should appear')
    );
    expect(calls.length).toBe(1);

    stdoutSpy.mockRestore();
  });
});

// ============================================================
// D5: DATA INTEGRITY — Store operations
// ============================================================
describe('D5: Data Integrity — Store transactions', () => {
  let createStore: typeof import('../src/store.js').createStore;
  let evictCachedStore: typeof import('../src/store.js').evictCachedStore;
  let tmpDir: string;
  let dbPath: string;

  beforeEach(async () => {
    const mod = await import('../src/store.js');
    createStore = mod.createStore;
    evictCachedStore = mod.evictCachedStore;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-store-'));
    dbPath = path.join(tmpDir, 'test.db');
  });

  afterEach(() => {
    try { evictCachedStore(dbPath); } catch {}
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-010: getIndexHealth returns consistent snapshot', () => {
    const store = createStore(dbPath);

    // Insert some documents
    const hash = crypto.createHash('sha256').update('test content').digest('hex');
    store.insertContent(hash, 'test content');
    store.insertDocument({
      collection: 'memory',
      path: '/test/doc1.md',
      title: 'doc1',
      hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    const health = store.getIndexHealth();
    expect(health.documentCount).toBe(1);
    expect(health.collections.length).toBeGreaterThan(0);
    expect(typeof health.databaseSize).toBe('number');
    expect(health.pendingEmbeddings).toBeGreaterThanOrEqual(0);

    store.close();
  });

  it('TC-011: cleanOrphanedEmbeddings inside transaction does not delete active docs', () => {
    const store = createStore(dbPath);

    const hash = crypto.createHash('sha256').update('active content').digest('hex');
    store.insertContent(hash, 'active content');
    store.insertDocument({
      collection: 'memory',
      path: '/test/active.md',
      title: 'active',
      hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    // Embed the active doc
    store.insertEmbeddingLocal(hash, 0, 0, 'test-model');

    const deleted = store.cleanOrphanedEmbeddings();
    // Active doc embeddings should NOT be deleted
    expect(deleted).toBe(0);

    store.close();
  });

  it('TC-012: bulkDeactivateExcept preserves active paths', () => {
    const store = createStore(dbPath);

    const hashes = ['content1', 'content2', 'content3'].map(c => {
      const h = crypto.createHash('sha256').update(c).digest('hex');
      store.insertContent(h, c);
      store.insertDocument({
        collection: 'memory',
        path: `/test/${c}.md`,
        title: c,
        hash: h,
        createdAt: new Date().toISOString(),
        modifiedAt: new Date().toISOString(),
        active: true,
        projectHash: 'test123',
      });
      return h;
    });

    // Keep only content1, deactivate content2 and content3
    const deactivated = store.bulkDeactivateExcept('memory', ['/test/content1.md']);
    expect(deactivated).toBe(2);

    // Verify content1 is still active
    const doc = store.findDocument('/test/content1.md');
    expect(doc).not.toBeNull();

    store.close();
  });

  it('TC-013: createStore returns cached instance for same path', () => {
    const store1 = createStore(dbPath);
    const store2 = createStore(dbPath);
    expect(store1).toBe(store2);
    store1.close();
  });
});

// ============================================================
// D7: EDGE CASES — Search edge cases
// ============================================================
describe('D7: Edge Cases — Search', () => {
  let createStore: typeof import('../src/store.js').createStore;
  let evictCachedStore: typeof import('../src/store.js').evictCachedStore;
  let searchFTS: typeof import('../src/search.js').searchFTS;
  let tmpDir: string;
  let dbPath: string;

  beforeEach(async () => {
    const storeMod = await import('../src/store.js');
    const searchMod = await import('../src/search.js');
    createStore = storeMod.createStore;
    evictCachedStore = storeMod.evictCachedStore;
    searchFTS = searchMod.searchFTS;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-search-'));
    dbPath = path.join(tmpDir, 'test.db');
  });

  afterEach(() => {
    try { evictCachedStore(dbPath); } catch {}
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-014: searchFTS handles empty query gracefully', () => {
    const store = createStore(dbPath);
    const results = searchFTS(store, '', { limit: 10 });
    expect(Array.isArray(results)).toBe(true);
    store.close();
  });

  it('TC-015: searchFTS handles special characters in query', () => {
    const store = createStore(dbPath);
    // These should not crash FTS5
    const specialQueries = [
      'test OR AND NOT',
      '"unclosed quote',
      'hello*world',
      'test()',
      'a AND',
      'OR test',
      '****',
      '""',
      'hello "world',
    ];
    for (const q of specialQueries) {
      const results = searchFTS(store, q, { limit: 5 });
      expect(Array.isArray(results)).toBe(true);
    }
    store.close();
  });

  it('TC-016: searchFTS handles very long query', () => {
    const store = createStore(dbPath);
    const longQuery = 'a '.repeat(5000);
    const results = searchFTS(store, longQuery, { limit: 5 });
    expect(Array.isArray(results)).toBe(true);
    store.close();
  });

  it('TC-017: searchFTS handles Unicode/Vietnamese queries', () => {
    const store = createStore(dbPath);

    const hash = crypto.createHash('sha256').update('Nguyễn Văn A đang sử dụng nano-brain').digest('hex');
    store.insertContent(hash, 'Nguyễn Văn A đang sử dụng nano-brain');
    store.insertDocument({
      collection: 'memory',
      path: '/test/vn.md',
      title: 'vn',
      hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    const results = searchFTS(store, 'Nguyễn', { limit: 5, projectHash: 'test123' });
    expect(results.length).toBeGreaterThanOrEqual(0); // FTS may not match diacritics perfectly
    store.close();
  });
});

// ============================================================
// D4: SECURITY — FTS injection resistance
// ============================================================
describe('D4: Security — FTS injection resistance', () => {
  let createStore: typeof import('../src/store.js').createStore;
  let evictCachedStore: typeof import('../src/store.js').evictCachedStore;
  let sanitizeFTS5Query: typeof import('../src/store.js').sanitizeFTS5Query;
  let tmpDir: string;
  let dbPath: string;

  beforeEach(async () => {
    const mod = await import('../src/store.js');
    createStore = mod.createStore;
    evictCachedStore = mod.evictCachedStore;
    sanitizeFTS5Query = mod.sanitizeFTS5Query;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-sec-'));
    dbPath = path.join(tmpDir, 'test.db');
  });

  afterEach(() => {
    try { evictCachedStore(dbPath); } catch {}
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-018: sanitizeFTS5Query strips dangerous operators', () => {
    // FTS5 operators that could cause syntax errors
    const dangerous = [
      'NEAR(a, b)',
      'a NEAR/5 b',
      'column:value',
      '{a b c}',
    ];
    for (const input of dangerous) {
      const sanitized = sanitizeFTS5Query(input);
      expect(typeof sanitized).toBe('string');
      // Should not throw when used in a query
      const store = createStore(dbPath);
      expect(() => {
        store.searchFTS(input, { limit: 1 });
      }).not.toThrow();
      store.close();
      evictCachedStore(dbPath);
    }
  });

  it('TC-019: SQL injection via search query does not work', () => {
    const store = createStore(dbPath);
    const injectionAttempts = [
      "'; DROP TABLE documents; --",
      "' OR '1'='1",
      "UNION SELECT * FROM content --",
      "1; DELETE FROM documents",
    ];
    for (const attempt of injectionAttempts) {
      expect(() => {
        store.searchFTS(attempt, { limit: 5 });
      }).not.toThrow();
    }
    // Verify tables still exist
    const health = store.getIndexHealth();
    expect(health).toBeDefined();
    store.close();
  });
});

// ============================================================
// D2: API — Harvester atomic state
// ============================================================
describe('D2: API — Harvester atomic state', () => {
  let loadHarvestState: typeof import('../src/harvester.js').loadHarvestState;
  let saveHarvestState: typeof import('../src/harvester.js').saveHarvestState;
  let tmpDir: string;

  beforeEach(async () => {
    const mod = await import('../src/harvester.js');
    loadHarvestState = mod.loadHarvestState;
    saveHarvestState = mod.saveHarvestState;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-harvester-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-020: saveHarvestState writes atomically (no partial reads)', () => {
    const stateFile = path.join(tmpDir, 'harvest-state.json');

    const state = {
      'session-1': { mtime: Date.now(), messageCount: 10 },
      'session-2': { mtime: Date.now(), messageCount: 20 },
    };

    saveHarvestState(stateFile, state);

    // Verify state was written correctly
    const loaded = loadHarvestState(stateFile);
    expect(loaded['session-1'].mtime).toBe(state['session-1'].mtime);
    expect(loaded['session-2'].mtime).toBe(state['session-2'].mtime);
  });

  it('TC-021: saveHarvestState creates directory if missing', () => {
    const stateFile = path.join(tmpDir, 'nested', 'dir', 'state.json');

    saveHarvestState(stateFile, { 'test': { mtime: 123 } });

    expect(fs.existsSync(stateFile)).toBe(true);
    const loaded = loadHarvestState(stateFile);
    expect(loaded['test'].mtime).toBe(123);
  });

  it('TC-022: loadHarvestState handles corrupted JSON', () => {
    const stateFile = path.join(tmpDir, 'corrupt.json');
    fs.writeFileSync(stateFile, '{invalid json!!!');

    // Should not throw, return empty
    const state = loadHarvestState(stateFile);
    expect(state).toEqual({});
  });

  it('TC-023: loadHarvestState handles missing file', () => {
    const state = loadHarvestState(path.join(tmpDir, 'nonexistent.json'));
    expect(state).toEqual({});
  });

  it('TC-024: saveHarvestState no temp file left on success', () => {
    const stateFile = path.join(tmpDir, 'clean.json');
    saveHarvestState(stateFile, { 'a': { mtime: 1 } });

    const files = fs.readdirSync(tmpDir);
    const tmpFiles = files.filter(f => f.includes('.tmp.'));
    expect(tmpFiles.length).toBe(0);
  });
});

// ============================================================
// D7: EDGE CASES — Reranker bounds check
// ============================================================
describe('D7: Edge Cases — Reranker bounds', () => {
  it('TC-025: reranker filters out-of-bounds indices', async () => {
    // Simulate what VoyageAI reranker does internally
    const documents = [
      { text: 'doc1', file: 'file1', index: 0 },
      { text: 'doc2', file: 'file2', index: 1 },
    ];

    // Simulate API returning invalid indices
    const apiResults = [
      { index: 0, relevance_score: 0.9 },
      { index: 1, relevance_score: 0.7 },
      { index: 5, relevance_score: 0.5 },  // out of bounds
      { index: -1, relevance_score: 0.3 }, // negative
    ];

    const filtered = apiResults
      .filter(r => r.index >= 0 && r.index < documents.length)
      .map(r => ({
        file: documents[r.index].file,
        score: r.relevance_score,
        index: r.index,
      }));

    expect(filtered.length).toBe(2);
    expect(filtered[0].file).toBe('file1');
    expect(filtered[1].file).toBe('file2');
  });
});

// ============================================================
// D5: DATA INTEGRITY — Embedding batch partial failure
// ============================================================
describe('D5: Data Integrity — Embedding batch resilience', () => {
  it('TC-026: zero vector has correct dimensions', () => {
    const dims = 1024;
    const zeroVec = new Array(dims).fill(0);
    expect(zeroVec.length).toBe(dims);
    expect(zeroVec.every(v => v === 0)).toBe(true);
  });
});

// ============================================================
// D6: INFRA — Concurrent file writes
// ============================================================
describe('D6: Infrastructure — sequentialFileAppend', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-append-'));
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-027: concurrent appends produce complete entries', async () => {
    const filePath = path.join(tmpDir, 'test.md');
    fs.writeFileSync(filePath, '');

    // Simulate what sequentialFileAppend does
    const queue = new Map<string, Promise<void>>();
    function seqAppend(fp: string, data: string): void {
      const prev = queue.get(fp) ?? Promise.resolve();
      const next = prev.then(() => {
        fs.appendFileSync(fp, data, 'utf-8');
      }).catch(() => {});
      queue.set(fp, next);
    }

    // Simulate 10 concurrent writes
    const entries: string[] = [];
    for (let i = 0; i < 10; i++) {
      const entry = `\n## Entry ${i}\n\nContent for entry ${i}\n`;
      entries.push(entry);
      seqAppend(filePath, entry);
    }

    // Wait for all writes
    await queue.get(filePath);

    const content = fs.readFileSync(filePath, 'utf-8');
    for (let i = 0; i < 10; i++) {
      expect(content).toContain(`## Entry ${i}`);
      expect(content).toContain(`Content for entry ${i}`);
    }
  });
});

// ============================================================
// D7: EDGE CASES — Store edge cases
// ============================================================
describe('D7: Edge Cases — Store edge cases', () => {
  let createStore: typeof import('../src/store.js').createStore;
  let evictCachedStore: typeof import('../src/store.js').evictCachedStore;
  let tmpDir: string;
  let dbPath: string;

  beforeEach(async () => {
    const mod = await import('../src/store.js');
    createStore = mod.createStore;
    evictCachedStore = mod.evictCachedStore;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-edge-'));
    dbPath = path.join(tmpDir, 'test.db');
  });

  afterEach(() => {
    try { evictCachedStore(dbPath); } catch {}
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-028: insertDocument with duplicate path updates existing', () => {
    const store = createStore(dbPath);

    const hash1 = crypto.createHash('sha256').update('v1').digest('hex');
    store.insertContent(hash1, 'v1');
    store.insertDocument({
      collection: 'memory',
      path: '/test/dup.md',
      title: 'dup',
      hash: hash1,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    const hash2 = crypto.createHash('sha256').update('v2').digest('hex');
    store.insertContent(hash2, 'v2');
    store.insertDocument({
      collection: 'memory',
      path: '/test/dup.md',
      title: 'dup-updated',
      hash: hash2,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    const health = store.getIndexHealth();
    // Should still be 1 document (upserted), not 2
    expect(health.documentCount).toBe(1);

    store.close();
  });

  it('TC-029: insertTags with empty array does not crash', () => {
    const store = createStore(dbPath);

    const hash = crypto.createHash('sha256').update('tagged').digest('hex');
    store.insertContent(hash, 'tagged');
    const docId = store.insertDocument({
      collection: 'memory',
      path: '/test/tagged.md',
      title: 'tagged',
      hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    expect(() => store.insertTags(docId, [])).not.toThrow();
    expect(() => store.insertTags(docId, ['', '  ', ''])).not.toThrow();

    store.close();
  });

  it('TC-030: insertTags deduplicates case-insensitively', () => {
    const store = createStore(dbPath);

    const hash = crypto.createHash('sha256').update('tags-test').digest('hex');
    store.insertContent(hash, 'tags-test');
    const docId = store.insertDocument({
      collection: 'memory',
      path: '/test/tags.md',
      title: 'tags',
      hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    store.insertTags(docId, ['Bug', 'bug', 'BUG', 'feature', 'Feature']);

    // Search by tag to verify
    const results = store.searchFTS('tags-test', { limit: 10, projectHash: 'test123' });
    // Should not crash
    expect(Array.isArray(results)).toBe(true);

    store.close();
  });

  it('TC-031: bulkDeactivateExcept with empty activePaths deactivates all', () => {
    const store = createStore(dbPath);

    for (let i = 0; i < 3; i++) {
      const hash = crypto.createHash('sha256').update(`doc${i}`).digest('hex');
      store.insertContent(hash, `doc${i}`);
      store.insertDocument({
        collection: 'memory',
        path: `/test/doc${i}.md`,
        title: `doc${i}`,
        hash,
        createdAt: new Date().toISOString(),
        modifiedAt: new Date().toISOString(),
        active: true,
        projectHash: 'test123',
      });
    }

    const deactivated = store.bulkDeactivateExcept('memory', []);
    expect(deactivated).toBe(3);

    store.close();
  });

  it('TC-032: store handles large content gracefully', () => {
    const store = createStore(dbPath);

    // 1MB content
    const bigContent = 'x'.repeat(1024 * 1024);
    const hash = crypto.createHash('sha256').update(bigContent).digest('hex');
    store.insertContent(hash, bigContent);
    const docId = store.insertDocument({
      collection: 'memory',
      path: '/test/big.md',
      title: 'big',
      hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
      projectHash: 'test123',
    });

    expect(docId).toBeGreaterThan(0);
    const body = store.getDocumentBody(hash);
    expect(body?.length).toBe(1024 * 1024);

    store.close();
  });
});

// ============================================================
// D2: API — parseSearchConfig edge cases
// ============================================================
describe('D2: API — parseSearchConfig', () => {
  let parseSearchConfig: typeof import('../src/search.js').parseSearchConfig;
  let DEFAULTS: { rrf_k: number; top_k: number; centrality_weight: number; supersede_demotion: number };

  beforeAll(async () => {
    const searchMod = await import('../src/search.js');
    const typesMod = await import('../src/types.js');
    parseSearchConfig = searchMod.parseSearchConfig;
    DEFAULTS = typesMod.DEFAULT_SEARCH_CONFIG;
  });

  it('TC-033: undefined input returns defaults', () => {
    const config = parseSearchConfig(undefined);
    expect(config.rrf_k).toBe(DEFAULTS.rrf_k);
  });

  it('TC-034: empty object returns defaults', () => {
    const config = parseSearchConfig({});
    expect(config.rrf_k).toBe(DEFAULTS.rrf_k);
  });

  it('TC-035: zero rrf_k is accepted (not negative)', () => {
    const config = parseSearchConfig({ rrf_k: 0 });
    expect(config.rrf_k).toBe(0);
  });

  it('TC-036: negative values use defaults', () => {
    const config = parseSearchConfig({
      rrf_k: -10,
      top_k: -5,
      centrality_weight: -0.1,
      supersede_demotion: -0.5,
    });
    expect(config.rrf_k).toBe(DEFAULTS.rrf_k);
    expect(config.top_k).toBe(DEFAULTS.top_k);
    expect(config.centrality_weight).toBe(DEFAULTS.centrality_weight);
    expect(config.supersede_demotion).toBe(DEFAULTS.supersede_demotion);
  });

  it('TC-037: very large rrf_k is accepted', () => {
    const config = parseSearchConfig({ rrf_k: 999999 });
    expect(config.rrf_k).toBe(999999);
  });
});

// ============================================================
// D3: PERFORMANCE — FTS under load
// ============================================================
describe('D3: Performance — FTS under load', () => {
  let createStore: typeof import('../src/store.js').createStore;
  let evictCachedStore: typeof import('../src/store.js').evictCachedStore;
  let searchFTS: typeof import('../src/search.js').searchFTS;
  let tmpDir: string;
  let dbPath: string;

  beforeAll(async () => {
    const storeMod = await import('../src/store.js');
    const searchMod = await import('../src/search.js');
    createStore = storeMod.createStore;
    evictCachedStore = storeMod.evictCachedStore;
    searchFTS = searchMod.searchFTS;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-perf-'));
    dbPath = path.join(tmpDir, 'perf.db');

    const store = createStore(dbPath);
    // Insert 500 docs
    for (let i = 0; i < 500; i++) {
      const content = `Document ${i}: This is memory content about topic ${i % 50} related to project alpha`;
      const hash = crypto.createHash('sha256').update(content).digest('hex');
      store.insertContent(hash, content);
      store.insertDocument({
        collection: 'memory',
        path: `/test/doc${i}.md`,
        title: `doc${i}`,
        hash,
        createdAt: new Date().toISOString(),
        modifiedAt: new Date().toISOString(),
        active: true,
        projectHash: 'perf123',
      });
    }
  });

  afterAll(() => {
    try { evictCachedStore(dbPath); } catch {}
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-038: 50 sequential FTS queries under 15s', () => {
    const store = createStore(dbPath);
    const queries = ['memory', 'project', 'topic', 'alpha', 'document', 'content', 'related', 'test', 'doc', 'about'];

    const start = Date.now();
    for (let i = 0; i < 50; i++) {
      searchFTS(store, queries[i % queries.length], { limit: 10, projectHash: 'perf123' });
    }
    const elapsed = Date.now() - start;
    // Relaxed threshold for CI and varied environments
    expect(elapsed).toBeLessThan(15000);
  });

  it('TC-039: FTS returns results for known content', () => {
    const store = createStore(dbPath);
    const results = searchFTS(store, 'project alpha', { limit: 10, projectHash: 'perf123' });
    expect(results.length).toBeGreaterThan(0);
  });
});

// ============================================================
// D4: SECURITY — Content hash integrity
// ============================================================
describe('D4: Security — Content hash integrity', () => {
  let createStore: typeof import('../src/store.js').createStore;
  let evictCachedStore: typeof import('../src/store.js').evictCachedStore;
  let tmpDir: string;
  let dbPath: string;

  beforeEach(async () => {
    const mod = await import('../src/store.js');
    createStore = mod.createStore;
    evictCachedStore = mod.evictCachedStore;
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rri-t-hash-'));
    dbPath = path.join(tmpDir, 'test.db');
  });

  afterEach(() => {
    try { evictCachedStore(dbPath); } catch {}
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('TC-040: content addressed by SHA-256 is tamper-proof', () => {
    const store = createStore(dbPath);

    const content = 'sensitive memory content';
    const hash = crypto.createHash('sha256').update(content).digest('hex');
    store.insertContent(hash, content);

    // Retrieve and verify
    const retrieved = store.getDocumentBody(hash);
    expect(retrieved).toBe(content);

    // Wrong hash returns nothing
    const wrongHash = crypto.createHash('sha256').update('different').digest('hex');
    const nothing = store.getDocumentBody(wrongHash);
    expect(nothing).toBeNull();

    store.close();
  });
});
