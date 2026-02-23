import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { parseSize, parseDuration, parseStorageConfig, checkDiskSpace, evictExpiredSessions, evictBySize } from '../src/storage.js';
import { createStore, computeHash } from '../src/store.js';
import type { Store } from '../src/types.js';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

describe('Storage Limits', () => {
  describe('parseSize', () => {
    it('should parse MB', () => {
      expect(parseSize('500MB')).toBe(524288000);
    });

    it('should parse GB', () => {
      expect(parseSize('2GB')).toBe(2147483648);
    });

    it('should parse TB', () => {
      expect(parseSize('1TB')).toBe(1099511627776);
    });

    it('should parse KB', () => {
      expect(parseSize('100KB')).toBe(102400);
    });

    it('should be case insensitive', () => {
      expect(parseSize('2gb')).toBe(2147483648);
    });

    it('should return -1 for invalid input', () => {
      expect(parseSize('banana')).toBe(-1);
    });

    it('should return -1 for empty string', () => {
      expect(parseSize('')).toBe(-1);
    });

    it('should handle decimal values', () => {
      expect(parseSize('1.5GB')).toBe(Math.floor(1.5 * 1024 * 1024 * 1024));
    });
  });

  describe('parseDuration', () => {
    it('should parse days', () => {
      expect(parseDuration('30d')).toBe(2592000000);
    });

    it('should parse weeks', () => {
      expect(parseDuration('2w')).toBe(1209600000);
    });

    it('should parse months', () => {
      expect(parseDuration('3m')).toBe(7776000000);
    });

    it('should parse years', () => {
      expect(parseDuration('1y')).toBe(31536000000);
    });

    it('should return -1 for invalid input', () => {
      expect(parseDuration('banana')).toBe(-1);
    });

    it('should return -1 for empty string', () => {
      expect(parseDuration('')).toBe(-1);
    });
  });

  describe('parseStorageConfig', () => {
    it('should use defaults when no config provided', () => {
      const config = parseStorageConfig();
      expect(config).toEqual({
        maxSize: 2147483648,
        retention: 7776000000,
        minFreeDisk: 104857600,
      });
    });

    it('should use defaults when undefined provided', () => {
      const config = parseStorageConfig(undefined);
      expect(config).toEqual({
        maxSize: 2147483648,
        retention: 7776000000,
        minFreeDisk: 104857600,
      });
    });

    it('should parse all fields', () => {
      const config = parseStorageConfig({
        maxSize: '1GB',
        retention: '30d',
        minFreeDisk: '200MB',
      });
      expect(config).toEqual({
        maxSize: 1073741824,
        retention: 2592000000,
        minFreeDisk: 209715200,
      });
    });

    it('should use defaults for invalid values', () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
      
      const config = parseStorageConfig({ maxSize: 'banana' });
      
      expect(config.maxSize).toBe(2147483648);
      expect(warnSpy).toHaveBeenCalled();
      
      warnSpy.mockRestore();
    });

    it('should handle partial config', () => {
      const config = parseStorageConfig({ maxSize: '500MB' });
      expect(config.maxSize).toBe(524288000);
      expect(config.retention).toBe(7776000000);
      expect(config.minFreeDisk).toBe(104857600);
    });
  });

  describe('checkDiskSpace', () => {
    it('should return ok when enough space', () => {
      const result = checkDiskSpace(os.tmpdir(), 1);
      expect(result.ok).toBe(true);
      expect(result.freeBytes).toBeGreaterThan(0);
    });

    it('should return not ok when minFreeDisk is huge', () => {
      const result = checkDiskSpace(os.tmpdir(), Number.MAX_SAFE_INTEGER);
      expect(result.ok).toBe(false);
    });
  });

  describe('eviction', () => {
    let tmpDir: string;
    let sessionsDir: string;
    let dbPath: string;
    let store: Store;

    beforeEach(() => {
      tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'opencode-memory-storage-test-'));
      sessionsDir = path.join(tmpDir, 'sessions');
      dbPath = path.join(tmpDir, 'test.db');
      fs.mkdirSync(sessionsDir, { recursive: true });
      store = createStore(dbPath);
    });

    afterEach(() => {
      store.close();
      if (fs.existsSync(tmpDir)) {
        fs.rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    function createSessionFile(hashDir: string, filename: string, content: string, mtime?: Date): string {
      const dirPath = path.join(sessionsDir, hashDir);
      fs.mkdirSync(dirPath, { recursive: true });
      const filePath = path.join(dirPath, filename);
      fs.writeFileSync(filePath, content);
      
      if (mtime) {
        fs.utimesSync(filePath, mtime, mtime);
      }
      
      return filePath;
    }

    it('should evict expired sessions', () => {
      const oldDate = new Date(Date.now() - 100 * 24 * 60 * 60 * 1000);
      const filePath = createSessionFile('hash123456ab', 'old.md', 'old content', oldDate);
      
      expect(fs.existsSync(filePath)).toBe(true);
      
      const evicted = evictExpiredSessions(sessionsDir, 7776000000, store);
      
      expect(evicted).toBe(1);
      expect(fs.existsSync(filePath)).toBe(false);
    });

    it('should not evict recent sessions', () => {
      const filePath = createSessionFile('hash123456ab', 'recent.md', 'recent content');
      
      expect(fs.existsSync(filePath)).toBe(true);
      
      const evicted = evictExpiredSessions(sessionsDir, 7776000000, store);
      
      expect(evicted).toBe(0);
      expect(fs.existsSync(filePath)).toBe(true);
    });

    it('should evict by size oldest first', () => {
      const oldDate = new Date(Date.now() - 10 * 24 * 60 * 60 * 1000);
      const newerDate = new Date(Date.now() - 5 * 24 * 60 * 60 * 1000);
      
      const oldFile = createSessionFile('hash111111ab', 'old.md', 'A'.repeat(1000), oldDate);
      const newFile = createSessionFile('hash222222ab', 'new.md', 'B'.repeat(1000), newerDate);
      
      const evicted = evictBySize(sessionsDir, dbPath, 100, store);
      
      expect(evicted).toBeGreaterThan(0);
      expect(fs.existsSync(oldFile)).toBe(false);
    });

    it('should not evict when under maxSize', () => {
      const file1 = createSessionFile('hash111111ab', 'file1.md', 'small content');
      const file2 = createSessionFile('hash222222ab', 'file2.md', 'small content');
      
      const evicted = evictBySize(sessionsDir, dbPath, Number.MAX_SAFE_INTEGER, store);
      
      expect(evicted).toBe(0);
      expect(fs.existsSync(file1)).toBe(true);
      expect(fs.existsSync(file2)).toBe(true);
    });
  });

  describe('deleteDocumentsByPath', () => {
    let tmpDir: string;
    let dbPath: string;
    let store: Store;

    beforeEach(() => {
      tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'opencode-memory-delete-test-'));
      dbPath = path.join(tmpDir, 'test.db');
      store = createStore(dbPath);
    });

    afterEach(() => {
      store.close();
      if (fs.existsSync(tmpDir)) {
        fs.rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    it('should delete document by path', () => {
      const body = '# Delete Me\n\nContent.';
      const hash = computeHash(body);
      
      store.insertContent(hash, body);
      store.insertDocument({
        collection: 'test',
        path: 'delete/me.md',
        title: 'Delete Me',
        hash,
        createdAt: new Date().toISOString(),
        modifiedAt: new Date().toISOString(),
        active: true,
      });
      
      const docBefore = store.findDocument('delete/me.md');
      expect(docBefore).not.toBeNull();
      
      const deleted = store.deleteDocumentsByPath('delete/me.md');
      expect(deleted).toBe(1);
      
      const docAfter = store.findDocument('delete/me.md');
      expect(docAfter).toBeNull();
    });
  });

  describe('cleanOrphanedEmbeddings', () => {
    let tmpDir: string;
    let dbPath: string;
    let store: Store;

    beforeEach(() => {
      tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'opencode-memory-orphan-test-'));
      dbPath = path.join(tmpDir, 'test.db');
      store = createStore(dbPath);
    });

    afterEach(() => {
      store.close();
      if (fs.existsSync(tmpDir)) {
        fs.rmSync(tmpDir, { recursive: true, force: true });
      }
    });

    it('should clean orphaned embeddings', () => {
      const body = '# Orphan Test\n\nContent.';
      const hash = computeHash(body);
      
      store.insertContent(hash, body);
      store.insertDocument({
        collection: 'test',
        path: 'orphan/test.md',
        title: 'Orphan Test',
        hash,
        createdAt: new Date().toISOString(),
        modifiedAt: new Date().toISOString(),
        active: true,
      });
      
      store.insertEmbedding(hash, 0, 0, new Array(384).fill(0.1), 'test-model');
      
      store.deactivateDocument('test', 'orphan/test.md');
      
      const cleaned = store.cleanOrphanedEmbeddings();
      
      expect(cleaned).toBeGreaterThanOrEqual(1);
    });
  });
});
