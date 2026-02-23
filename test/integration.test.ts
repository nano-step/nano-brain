import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import * as crypto from 'crypto';
import { createStore, sanitizeFTS5Query } from '../src/store.js';
import type { Store } from '../src/types.js';

describe('FTS5 Query Sanitization', () => {
  it('wraps normal query in quotes', () => {
    expect(sanitizeFTS5Query('hello world')).toBe('"hello world"');
  });

  it('preserves hyphenated words', () => {
    expect(sanitizeFTS5Query('nano-brain')).toBe('"nano-brain"');
  });

  it('escapes internal double quotes', () => {
    expect(sanitizeFTS5Query('hello "world"')).toBe('"hello ""world"""');
  });

  it('neutralizes FTS5 operators', () => {
    expect(sanitizeFTS5Query('AND OR NOT')).toBe('"AND OR NOT"');
  });

  it('neutralizes FTS5 column names', () => {
    expect(sanitizeFTS5Query('filepath: test')).toBe('"filepath: test"');
  });

  it('returns empty string for empty input', () => {
    expect(sanitizeFTS5Query('')).toBe('');
  });

  it('returns empty string for whitespace-only input', () => {
    expect(sanitizeFTS5Query('   ')).toBe('');
  });
});

describe('Real Database Integration', () => {
  let tempDir: string;
  let dbPath: string;
  let store: Store;

  beforeAll(() => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nano-brain-test-'));
    dbPath = path.join(tempDir, 'test.db');
    store = createStore(dbPath);

    const doc1Content = '# Nano Brain\n\nThis is a test document about nano-brain architecture.';
    const doc1Hash = crypto.createHash('sha256').update(doc1Content).digest('hex');
    store.insertContent(doc1Hash, doc1Content);
    store.insertDocument({
      collection: 'test-collection',
      path: '/test/doc1.md',
      title: 'Nano Brain',
      hash: doc1Hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
    });

    const doc2Content = '# Search Guide\n\nHow to use AND OR NOT operators in search queries.';
    const doc2Hash = crypto.createHash('sha256').update(doc2Content).digest('hex');
    store.insertContent(doc2Hash, doc2Content);
    store.insertDocument({
      collection: 'test-collection',
      path: '/test/doc2.md',
      title: 'Search Guide',
      hash: doc2Hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
    });

    const doc3Content = '# Daily Log\n\nToday I worked on the filepath indexing feature.';
    const doc3Hash = crypto.createHash('sha256').update(doc3Content).digest('hex');
    store.insertContent(doc3Hash, doc3Content);
    store.insertDocument({
      collection: 'daily',
      path: '/daily/2024-01-01.md',
      title: 'Daily Log',
      hash: doc3Hash,
      createdAt: new Date().toISOString(),
      modifiedAt: new Date().toISOString(),
      active: true,
    });
  });

  afterAll(() => {
    store.close();
    fs.rmSync(tempDir, { recursive: true, force: true });
    expect(fs.existsSync(tempDir)).toBe(false);
  });

  it('search finds indexed documents', () => {
    const results = store.searchFTS('memory', 10);
    expect(results.length).toBeGreaterThan(0);
    expect(results.some(r => r.title === 'Nano Brain')).toBe(true);
  });

  it('hyphenated query works without error', () => {
    expect(() => {
      const results = store.searchFTS('nano-brain', 10);
      expect(Array.isArray(results)).toBe(true);
    }).not.toThrow();
  });

  it('FTS5 operator words work without error', () => {
    expect(() => {
      const results = store.searchFTS('AND OR NOT', 10);
      expect(Array.isArray(results)).toBe(true);
    }).not.toThrow();
  });

  it('FTS5 column name words work without error', () => {
    expect(() => {
      const results = store.searchFTS('filepath', 10);
      expect(Array.isArray(results)).toBe(true);
    }).not.toThrow();
  });

  it('empty query returns empty array', () => {
    const results = store.searchFTS('', 10);
    expect(results).toEqual([]);
  });

  it('collection filter works', () => {
    const results = store.searchFTS('log', 10, 'daily');
    expect(results.length).toBeGreaterThan(0);
    expect(results.every(r => r.collection === 'daily')).toBe(true);
  });

  it('getHealth() returns correct document count', () => {
    const health = store.getIndexHealth();
    expect(health.documentCount).toBe(3);
  });

  it('getHealth() returns correct collection stats', () => {
    const health = store.getIndexHealth();
    expect(health.collections.length).toBe(2);
    
    const testCollection = health.collections.find(c => c.name === 'test-collection');
    expect(testCollection).toBeDefined();
    expect(testCollection?.documentCount).toBe(2);
    
    const dailyCollection = health.collections.find(c => c.name === 'daily');
    expect(dailyCollection).toBeDefined();
    expect(dailyCollection?.documentCount).toBe(1);
  });
});
