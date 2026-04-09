import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createStore, evictCachedStore } from '../src/store.js';
import type { Store } from '../src/types.js';
import { generateBriefing, type BriefingResult } from '../src/wake-up.js';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import * as crypto from 'crypto';

function createTempStore(): { store: Store; dbPath: string; tmpDir: string; configPath: string } {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nb-wakeup-'));
  const dbPath = path.join(tmpDir, 'test.db');
  const configPath = path.join(tmpDir, 'config.yml');
  fs.writeFileSync(configPath, 'collections:\n  memory:\n    path: /tmp/memory\n    pattern: "**/*.md"\n');
  const store = createStore(dbPath);
  return { store, dbPath, tmpDir, configPath };
}

function insertTestDoc(
  store: Store,
  opts: { title: string; collection?: string; projectHash?: string; accessCount?: number; tags?: string[]; supersededBy?: number; active?: boolean; modifiedAt?: string }
): number {
  const body = `Test content for ${opts.title}`;
  const hash = crypto.createHash('sha256').update(body + Date.now() + Math.random()).digest('hex');
  store.insertContent(hash, body);
  const docId = store.insertDocument({
    collection: opts.collection ?? 'memory',
    path: `test/${opts.title.replace(/\s+/g, '-').toLowerCase()}.md`,
    title: opts.title,
    hash,
    createdAt: new Date().toISOString(),
    modifiedAt: opts.modifiedAt ?? new Date().toISOString(),
    active: opts.active !== false,
    projectHash: opts.projectHash ?? 'testhash1234',
  });

  if (opts.accessCount && opts.accessCount > 0) {
    const db = store.getDb();
    db.prepare('UPDATE documents SET access_count = ? WHERE id = ?').run(opts.accessCount, docId);
  }

  if (opts.tags && opts.tags.length > 0) {
    store.insertTags(docId, opts.tags);
  }

  if (opts.supersededBy !== undefined) {
    store.supersedeDocument(docId, opts.supersededBy);
  }

  return docId;
}

describe('wake-up: generateBriefing', () => {
  let store: Store;
  let dbPath: string;
  let tmpDir: string;
  let configPath: string;

  beforeEach(() => {
    ({ store, dbPath, tmpDir, configPath } = createTempStore());
  });

  afterEach(() => {
    evictCachedStore(dbPath);
    if (fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  describe('API-1: return structure', () => {
    it('returns BriefingResult with all required fields', () => {
      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result).toHaveProperty('workspace');
      expect(result).toHaveProperty('l0');
      expect(result).toHaveProperty('l1_memories');
      expect(result).toHaveProperty('l1_decisions');
      expect(result).toHaveProperty('formatted');
      expect(typeof result.formatted).toBe('string');
      expect(result.l0).toHaveProperty('label');
      expect(result.l0).toHaveProperty('items');
      expect(Array.isArray(result.l0.items)).toBe(true);
    });
  });

  describe('EDGE-1: empty store', () => {
    it('returns "no memories yet" for empty store', () => {
      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result.formatted).toContain('No memories yet');
      expect(result.formatted.length).toBeGreaterThan(0);
    });
  });

  describe('API-7: getTopAccessedDocuments ordering', () => {
    it('returns documents ordered by access_count DESC', () => {
      insertTestDoc(store, { title: 'Low Access', accessCount: 1 });
      insertTestDoc(store, { title: 'High Access', accessCount: 100 });
      insertTestDoc(store, { title: 'Mid Access', accessCount: 50 });

      const docs = store.getTopAccessedDocuments(10, 'testhash1234');
      expect(docs.length).toBe(3);
      expect(docs[0].title).toBe('High Access');
      expect(docs[1].title).toBe('Mid Access');
      expect(docs[2].title).toBe('Low Access');
    });

    it('respects limit parameter', () => {
      for (let i = 0; i < 15; i++) {
        insertTestDoc(store, { title: `Doc ${i}`, accessCount: i });
      }
      const docs = store.getTopAccessedDocuments(5, 'testhash1234');
      expect(docs.length).toBe(5);
    });
  });

  describe('DATA-2: superseded document exclusion', () => {
    it('excludes superseded documents from getTopAccessedDocuments', () => {
      insertTestDoc(store, { title: 'Active Doc', accessCount: 10 });
      insertTestDoc(store, { title: 'Superseded Doc', accessCount: 100, supersededBy: 999 });

      const docs = store.getTopAccessedDocuments(10, 'testhash1234');
      expect(docs.length).toBe(1);
      expect(docs[0].title).toBe('Active Doc');
    });

    it('excludes superseded documents from getRecentDocumentsByTags', () => {
      insertTestDoc(store, { title: 'Active Decision', tags: ['decision'] });
      insertTestDoc(store, { title: 'Old Decision', tags: ['decision'], supersededBy: 999 });

      const docs = store.getRecentDocumentsByTags(['decision'], 10, 'testhash1234');
      expect(docs.length).toBe(1);
      expect(docs[0].title).toBe('Active Decision');
    });
  });

  describe('DATA-1: inactive document exclusion', () => {
    it('excludes inactive documents from getTopAccessedDocuments', () => {
      insertTestDoc(store, { title: 'Active', accessCount: 5 });
      insertTestDoc(store, { title: 'Inactive', accessCount: 50, active: false });

      const docs = store.getTopAccessedDocuments(10, 'testhash1234');
      expect(docs.length).toBe(1);
      expect(docs[0].title).toBe('Active');
    });
  });

  describe('API-8: getRecentDocumentsByTags', () => {
    it('filters by tag correctly', () => {
      insertTestDoc(store, { title: 'Decision A', tags: ['decision'] });
      insertTestDoc(store, { title: 'Note B', tags: ['note'] });
      insertTestDoc(store, { title: 'Decision C', tags: ['decision', 'architecture'] });

      const docs = store.getRecentDocumentsByTags(['decision'], 10, 'testhash1234');
      expect(docs.length).toBe(2);
      const titles = docs.map(d => d.title);
      expect(titles).toContain('Decision A');
      expect(titles).toContain('Decision C');
    });

    it('returns empty array for empty tags', () => {
      const docs = store.getRecentDocumentsByTags([], 10, 'testhash1234');
      expect(docs).toEqual([]);
    });

    it('returns empty array when no docs match', () => {
      insertTestDoc(store, { title: 'Note', tags: ['note'] });
      const docs = store.getRecentDocumentsByTags(['decision'], 10, 'testhash1234');
      expect(docs).toEqual([]);
    });

    it('orders by modified_at DESC', () => {
      insertTestDoc(store, { title: 'Old', tags: ['decision'], modifiedAt: '2024-01-01T00:00:00Z' });
      insertTestDoc(store, { title: 'New', tags: ['decision'], modifiedAt: '2025-06-01T00:00:00Z' });

      const docs = store.getRecentDocumentsByTags(['decision'], 10, 'testhash1234');
      expect(docs[0].title).toBe('New');
      expect(docs[1].title).toBe('Old');
    });
  });

  describe('DATA-3: project_hash scoping', () => {
    it('only returns docs matching projectHash or global', () => {
      insertTestDoc(store, { title: 'My Project', accessCount: 10, projectHash: 'testhash1234' });
      insertTestDoc(store, { title: 'Global Doc', accessCount: 5, projectHash: 'global' });
      insertTestDoc(store, { title: 'Other Project', accessCount: 20, projectHash: 'otherhash5678' });

      const docs = store.getTopAccessedDocuments(10, 'testhash1234');
      const titles = docs.map(d => d.title);
      expect(titles).toContain('My Project');
      expect(titles).toContain('Global Doc');
      expect(titles).not.toContain('Other Project');
    });
  });

  describe('PERF-3 / EDGE-7: character cap enforcement', () => {
    it('truncates output exceeding 2000 chars', () => {
      for (let i = 0; i < 50; i++) {
        insertTestDoc(store, {
          title: `Very Long Document Title Number ${i} With Lots Of Text To Make It Long`,
          accessCount: 50 - i,
        });
      }

      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result.formatted.length).toBeLessThanOrEqual(2000);
    });

    it('respects custom maxChars option', () => {
      for (let i = 0; i < 50; i++) {
        insertTestDoc(store, { title: `Doc ${i}`, accessCount: i });
      }

      const result = generateBriefing(store, configPath, 'testhash1234', { maxChars: 500 });
      expect(result.formatted.length).toBeLessThanOrEqual(500);
    });
  });

  describe('API-6: limit parameter', () => {
    it('limits top docs to specified count', () => {
      for (let i = 0; i < 20; i++) {
        insertTestDoc(store, { title: `Doc ${i}`, accessCount: i });
      }

      const result = generateBriefing(store, configPath, 'testhash1234', { limit: 3 });
      expect(result.l1_memories.items.length).toBeLessThanOrEqual(3);
    });
  });

  describe('EDGE-8: no decision-tagged docs', () => {
    it('l1_decisions section is empty when no decisions', () => {
      insertTestDoc(store, { title: 'Regular Doc', accessCount: 5, tags: ['note'] });

      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result.l1_decisions.items.length).toBe(0);
    });
  });

  describe('populated store briefing', () => {
    it('includes key memories and decisions in formatted output', () => {
      insertTestDoc(store, { title: 'Auth System', accessCount: 50, tags: ['architecture'] });
      insertTestDoc(store, { title: 'Use Redis', accessCount: 30, tags: ['decision'] });
      insertTestDoc(store, { title: 'DB Schema', accessCount: 20 });

      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result.formatted).toContain('Key Memories');
      expect(result.formatted).toContain('Auth System');
      expect(result.formatted).toContain('Recent Decisions');
      expect(result.formatted).toContain('Use Redis');
    });

    it('formatted output starts with workspace header', () => {
      insertTestDoc(store, { title: 'Doc', accessCount: 1 });
      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result.formatted).toMatch(/^## Context Briefing/);
    });
  });

  describe('EDGE-10: missing modified_at', () => {
    it('DB enforces NOT NULL on modified_at — "unknown" fallback is unreachable dead code', () => {
      const docId = insertTestDoc(store, { title: 'Old Decision', tags: ['decision'] });
      expect(() => {
        store.getDb().prepare('UPDATE documents SET modified_at = NULL WHERE id = ?').run(docId);
      }).toThrow(/NOT NULL/);
    });

    it('shows date portion for empty-string modified_at', () => {
      const docId = insertTestDoc(store, { title: 'Old Decision', tags: ['decision'] });
      store.getDb().prepare('UPDATE documents SET modified_at = ? WHERE id = ?').run('', docId);
      const result = generateBriefing(store, configPath, 'testhash1234');
      expect(result.l1_decisions.items[0]).toContain('unknown');
    });
  });
});

describe('wake-up: truncateLine (via generateBriefing)', () => {
  let store: Store;
  let dbPath: string;
  let tmpDir: string;
  let configPath: string;

  beforeEach(() => {
    ({ store, dbPath, tmpDir, configPath } = createTempStore());
  });

  afterEach(() => {
    evictCachedStore(dbPath);
    if (fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('EDGE-2: empty title falls back to path, not (untitled) u2014 FINDING: truncateLine unreachable for empty title', () => {
    const docId = insertTestDoc(store, { title: '', accessCount: 5 });
    store.getDb().prepare('UPDATE documents SET title = ? WHERE id = ?').run('', docId);

    const result = generateBriefing(store, configPath, 'testhash1234');
    expect(result.l1_memories.items[0]).toContain('.md');
    expect(result.l1_memories.items[0]).not.toContain('(untitled)');
  });

  it('EDGE-4: title at max does not truncate', () => {
    const title80 = 'A'.repeat(80);
    insertTestDoc(store, { title: title80, accessCount: 5 });

    const result = generateBriefing(store, configPath, 'testhash1234');
    expect(result.l1_memories.items[0]).toContain(title80);
    expect(result.l1_memories.items[0]).not.toContain('...');
  });

  it('EDGE-5: title over max truncates with ...', () => {
    const title81 = 'B'.repeat(81);
    insertTestDoc(store, { title: title81, accessCount: 5 });

    const result = generateBriefing(store, configPath, 'testhash1234');
    expect(result.l1_memories.items[0]).toContain('...');
    expect(result.l1_memories.items[0]).not.toContain(title81);
  });
});

describe('wake-up: Store interface contract (INFRA-2)', () => {
  let store: Store;
  let dbPath: string;
  let tmpDir: string;

  beforeEach(() => {
    const tmp = createTempStore();
    store = tmp.store;
    dbPath = tmp.dbPath;
    tmpDir = tmp.tmpDir;
  });

  afterEach(() => {
    evictCachedStore(dbPath);
    if (fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('store has getTopAccessedDocuments method', () => {
    expect(typeof store.getTopAccessedDocuments).toBe('function');
  });

  it('store has getRecentDocumentsByTags method', () => {
    expect(typeof store.getRecentDocumentsByTags).toBe('function');
  });

  it('getTopAccessedDocuments returns empty array for no docs', () => {
    const result = store.getTopAccessedDocuments(10, 'nonexistent');
    expect(result).toEqual([]);
  });

  it('getRecentDocumentsByTags returns empty for no matching docs', () => {
    const result = store.getRecentDocumentsByTags(['decision'], 10, 'nonexistent');
    expect(result).toEqual([]);
  });
});

describe('wake-up: SEC-1 prepared statements', () => {
  let store: Store;
  let dbPath: string;
  let tmpDir: string;

  beforeEach(() => {
    const tmp = createTempStore();
    store = tmp.store;
    dbPath = tmp.dbPath;
    tmpDir = tmp.tmpDir;
  });

  afterEach(() => {
    evictCachedStore(dbPath);
    if (fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('SQL injection in projectHash does not crash or leak', () => {
    const malicious = "'; DROP TABLE documents; --";
    const docs = store.getTopAccessedDocuments(10, malicious);
    expect(Array.isArray(docs)).toBe(true);
    const health = store.getIndexHealth();
    expect(health).toBeDefined();
  });

  it('SQL injection in tags does not crash or leak', () => {
    const malicious = ["'; DROP TABLE documents; --"];
    const docs = store.getRecentDocumentsByTags(malicious, 10, 'testhash1234');
    expect(Array.isArray(docs)).toBe(true);
    const health = store.getIndexHealth();
    expect(health).toBeDefined();
  });
});
