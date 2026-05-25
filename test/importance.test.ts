import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ImportanceScorer, type ImportanceParams } from '../src/importance.js';
import { createStore, evictCachedStore, computeHash } from '../src/store.js';
import type { Store } from '../src/types.js';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

const createMockStore = (): Store => ({
  close: vi.fn(),
  getActiveDocumentsWithAccess: vi.fn().mockReturnValue([]),
  getConnectionCount: vi.fn().mockReturnValue(0),
  getTagCountForDocument: vi.fn().mockReturnValue(0),
} as unknown as Store);

describe('ImportanceScorer', () => {
  let store: Store;

  beforeEach(() => {
    store = createMockStore();
  });

  describe('constructor', () => {
    it('should initialize with default config', () => {
      const scorer = new ImportanceScorer(store);
      expect(scorer).toBeDefined();
      expect(scorer.getConfig().enabled).toBe(false);
    });

    it('should accept custom config', () => {
      const scorer = new ImportanceScorer(store, { enabled: true, weight: 0.2 });
      expect(scorer.getConfig().enabled).toBe(true);
      expect(scorer.getConfig().weight).toBe(0.2);
    });
  });

  describe('calculateScore', () => {
    it('should return correct value for known inputs', () => {
      const scorer = new ImportanceScorer(store);
      const score = scorer.calculateScore({
        usageCount: 10,
        entityDensity: 0.5,
        daysSinceAccess: 0,
        connectionCount: 5,
        maxUsage: 10,
        maxConnections: 10,
      });
      expect(score).toBeGreaterThan(0);
      expect(score).toBeLessThanOrEqual(1);
    });

    it('should return 0 when all inputs are 0', () => {
      const scorer = new ImportanceScorer(store);
      const score = scorer.calculateScore({
        usageCount: 0,
        entityDensity: 0,
        daysSinceAccess: 0,
        connectionCount: 0,
        maxUsage: 0,
        maxConnections: 0,
      });
      expect(score).toBeGreaterThanOrEqual(0);
    });

    it('should weight usage correctly', () => {
      const scorer = new ImportanceScorer(store);
      const params: ImportanceParams = {
        usageCount: 10,
        entityDensity: 0,
        daysSinceAccess: 0,
        connectionCount: 0,
        maxUsage: 10,
        maxConnections: 0,
      };
      const score = scorer.calculateScore(params);
      expect(score).toBeGreaterThan(0);
    });

    it('should apply recency decay', () => {
      const scorer = new ImportanceScorer(store);
      const recentScore = scorer.calculateScore({
        usageCount: 0,
        entityDensity: 0,
        daysSinceAccess: 0,
        connectionCount: 0,
        maxUsage: 0,
        maxConnections: 0,
      });
      const oldScore = scorer.calculateScore({
        usageCount: 0,
        entityDensity: 0,
        daysSinceAccess: 60,
        connectionCount: 0,
        maxUsage: 0,
        maxConnections: 0,
      });
      expect(recentScore).toBeGreaterThan(oldScore);
    });
  });

  describe('applyBoost', () => {
    it('should boost search score by importance', () => {
      const scorer = new ImportanceScorer(store, { weight: 0.1 });
      const boosted = scorer.applyBoost(1.0, 0.5);
      expect(boosted).toBe(1.05);
    });

    it('should not change score when importance is 0', () => {
      const scorer = new ImportanceScorer(store, { weight: 0.1 });
      const boosted = scorer.applyBoost(1.0, 0);
      expect(boosted).toBe(1.0);
    });

    it('should modify search score correctly', () => {
      const scorer = new ImportanceScorer(store, { weight: 0.15 });
      const boosted = scorer.applyBoost(0.5, 0.8);
      expect(boosted).toBeCloseTo(0.56, 2);
    });
  });

  describe('getScore', () => {
    it('should return 0 for unknown docid', () => {
      const scorer = new ImportanceScorer(store);
      expect(scorer.getScore('unknown')).toBe(0);
    });
  });

  describe('recalculateAll with mock store', () => {
    it('should return 0 when no documents', async () => {
      const scorer = new ImportanceScorer(store);
      const count = await scorer.recalculateAll();
      expect(count).toBe(0);
    });
  });
});

describe('ImportanceScorer with real store', () => {
  let store: Store;
  let dbPath: string;
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nano-brain-importance-test-'));
    dbPath = path.join(tmpDir, 'test.db');
    store = createStore(dbPath);
  });

  afterEach(() => {
    evictCachedStore(dbPath);
    if (fs.existsSync(tmpDir)) {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  const insertTestDocument = (content: string, accessCount: number = 0, lastAccessedAt?: string) => {
    const hash = computeHash(content);
    store.insertContent(hash, content);
    const now = new Date().toISOString();
    const docId = store.insertDocument({
      collection: 'test',
      path: `test-${hash.substring(0, 8)}.md`,
      title: 'Test Document',
      hash,
      createdAt: now,
      modifiedAt: now,
      active: true,
      projectHash: 'test-project',
    });
    if (accessCount > 0 || lastAccessedAt) {
      const db = store.getDb();
      db.prepare('UPDATE documents SET access_count = ?, last_accessed_at = ? WHERE id = ?')
        .run(accessCount, lastAccessedAt ?? now, docId);
    }
    return { docId, hash };
  };

  describe('recalculateAll', () => {
    it('should populate scoreCache with entries', async () => {
      insertTestDocument('# Test Document 1\nSome content here', 10);
      insertTestDocument('# Test Document 2\nMore content', 5);

      const scorer = new ImportanceScorer(store);
      const count = await scorer.recalculateAll();

      expect(count).toBe(2);
    });

    it('should return cached score after recalculation', async () => {
      const { hash } = insertTestDocument('# Test Document\nContent', 10);
      const docid = hash.substring(0, 6);

      const scorer = new ImportanceScorer(store);
      await scorer.recalculateAll();

      const score = scorer.getScore(docid);
      expect(score).toBeGreaterThan(0);
    });

    it('should give higher score to documents with more access', async () => {
      const { hash: hashA } = insertTestDocument('# Doc A\nHigh access', 100);
      const { hash: hashB } = insertTestDocument('# Doc B\nLow access', 1);

      const scorer = new ImportanceScorer(store);
      await scorer.recalculateAll();

      const scoreA = scorer.getScore(hashA.substring(0, 6));
      const scoreB = scorer.getScore(hashB.substring(0, 6));

      expect(scoreA).toBeGreaterThan(scoreB);
    });

    it('should give higher score to recently accessed documents', async () => {
      const now = new Date();
      const recentDate = now.toISOString();
      const oldDate = new Date(now.getTime() - 60 * 24 * 60 * 60 * 1000).toISOString();

      const { hash: hashA } = insertTestDocument('# Doc A\nRecent', 10, recentDate);
      const { hash: hashB } = insertTestDocument('# Doc B\nOld', 10, oldDate);

      const scorer = new ImportanceScorer(store);
      await scorer.recalculateAll();

      const scoreA = scorer.getScore(hashA.substring(0, 6));
      const scoreB = scorer.getScore(hashB.substring(0, 6));

      expect(scoreA).toBeGreaterThan(scoreB);
    });

    it('should give higher score to documents with more connections', async () => {
      const { docId: docIdA, hash: hashA } = insertTestDocument('# Doc A\nWith connections', 10);
      const { docId: docIdB, hash: hashB } = insertTestDocument('# Doc B\nNo connections', 10);
      const { docId: docIdC } = insertTestDocument('# Doc C\nTarget', 10);

      for (let i = 0; i < 5; i++) {
        store.insertConnection({
          fromDocId: docIdA,
          toDocId: docIdC,
          relationshipType: 'related',
          description: null,
          strength: 1.0,
          createdBy: 'user',
          projectHash: 'test-project',
        });
      }

      const scorer = new ImportanceScorer(store);
      await scorer.recalculateAll();

      const scoreA = scorer.getScore(hashA.substring(0, 6));
      const scoreB = scorer.getScore(hashB.substring(0, 6));

      expect(scoreA).toBeGreaterThan(scoreB);
    });

    it('should give higher score to documents with more tags', async () => {
      const { docId: docIdA, hash: hashA } = insertTestDocument('# Doc A\nWith tags', 10);
      const { hash: hashB } = insertTestDocument('# Doc B\nNo tags', 10);

      store.insertTags(docIdA, ['tag1', 'tag2', 'tag3', 'tag4', 'tag5']);

      const scorer = new ImportanceScorer(store);
      await scorer.recalculateAll();

      const scoreA = scorer.getScore(hashA.substring(0, 6));
      const scoreB = scorer.getScore(hashB.substring(0, 6));

      expect(scoreA).toBeGreaterThan(scoreB);
    });
  });
});
