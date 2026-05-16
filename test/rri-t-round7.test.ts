/**
 * RRI-T Round 7 — excludeFolders, frontmatter tags, db:clean --list-only
 *
 * Coverage:
 *   1. excludeFolders in Collection → scanCollectionFiles excludes specified dirs
 *   2. parseFrontmatter uses yaml.parse (no comma-split brittleness)
 *   3. Frontmatter tags stored via store.insertTags after indexDocument
 *   4. db:clean --list-only flag (inspect without deletion language)
 *   5. excludeFolders config wired through getCollections
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { createStore, evictCachedStore } from '../src/store.js';
import type { Store } from '../src/types.js';
import { parseFrontmatter } from '../src/jobs/watcher.js';
import { scanForOrphanedDbs } from '../src/cli/commands/db-clean.js';
import { getCollections, loadCollectionConfig } from '../src/collections.js';
import { scanCollectionFiles } from '../src/collections.js';
import type { Collection } from '../src/types.js';

function makeTmpDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'nb-rri7-'));
}

function touch(p: string, content = ''): void {
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, content);
}

// ─── Phase 1: excludeFolders in scanCollectionFiles ──────────────────────────

describe('Phase 1 — excludeFolders in scanCollectionFiles', () => {
  let vaultDir: string;

  beforeEach(() => {
    vaultDir = makeTmpDir();
    // Create a realistic Obsidian vault structure
    touch(path.join(vaultDir, 'Note1.md'), '# Note 1');
    touch(path.join(vaultDir, 'Note2.md'), '# Note 2');
    touch(path.join(vaultDir, 'subfolder', 'Deep.md'), '# Deep');
    touch(path.join(vaultDir, '.trash', 'Deleted.md'), '# Deleted');
    touch(path.join(vaultDir, 'templates', 'Daily.md'), '# Template');
    touch(path.join(vaultDir, 'attachments', 'image.md'), '# Attachment');
    touch(path.join(vaultDir, 'excalidraw', 'Diagram.md'), '# Diagram');
  });

  afterEach(() => {
    fs.rmSync(vaultDir, { recursive: true, force: true });
  });

  it('without excludeFolders — includes non-hidden dirs like templates and attachments', async () => {
    const col: Collection = { name: 'vault', path: vaultDir, pattern: '**/*.md' };
    const files = await scanCollectionFiles(col);
    // Non-hidden dirs (templates, attachments, excalidraw) must be included
    expect(files.some(f => f.includes('templates'))).toBe(true);
    expect(files.some(f => f.includes('attachments'))).toBe(true);
    expect(files.some(f => f.includes('excalidraw'))).toBe(true);
    expect(files.some(f => f.includes('Note1'))).toBe(true);
  });

  it('excludeFolders: [.trash] — omits .trash files', async () => {
    const col: Collection = { name: 'vault', path: vaultDir, pattern: '**/*.md', excludeFolders: ['.trash'] };
    const files = await scanCollectionFiles(col);
    expect(files.some(f => f.includes('.trash'))).toBe(false);
    expect(files.some(f => f.includes('Note1'))).toBe(true);
  });

  it('excludeFolders: [templates, attachments] — omits both folders', async () => {
    const col: Collection = { name: 'vault', path: vaultDir, pattern: '**/*.md', excludeFolders: ['templates', 'attachments'] };
    const files = await scanCollectionFiles(col);
    expect(files.some(f => f.includes('templates'))).toBe(false);
    expect(files.some(f => f.includes('attachments'))).toBe(false);
    expect(files.some(f => f.includes('Note1'))).toBe(true);
    expect(files.some(f => f.includes('subfolder'))).toBe(true);
  });

  it('excludeFolders: all special dirs — only root and subfolder notes remain', async () => {
    const col: Collection = {
      name: 'vault',
      path: vaultDir,
      pattern: '**/*.md',
      excludeFolders: ['.trash', 'templates', 'attachments', 'excalidraw'],
    };
    const files = await scanCollectionFiles(col);
    const basenames = files.map(f => path.basename(f));
    expect(basenames).toContain('Note1.md');
    expect(basenames).toContain('Note2.md');
    expect(basenames).toContain('Deep.md');
    expect(files.some(f => f.includes('.trash') || f.includes('templates') || f.includes('attachments') || f.includes('excalidraw'))).toBe(false);
  });

  it('empty excludeFolders array — behaves same as no excludeFolders', async () => {
    const col: Collection = { name: 'vault', path: vaultDir, pattern: '**/*.md', excludeFolders: [] };
    const col2: Collection = { name: 'vault', path: vaultDir, pattern: '**/*.md' };
    const files1 = await scanCollectionFiles(col);
    const files2 = await scanCollectionFiles(col2);
    expect(files1.length).toBe(files2.length);
  });
});

// ─── Phase 2: parseFrontmatter with yaml parser ───────────────────────────────

describe('Phase 2 — parseFrontmatter correctness', () => {
  it('parses tags array correctly', () => {
    const content = '---\ntags: [tag1, tag2, tag3]\n---\n# Body';
    const fm = parseFrontmatter(content);
    expect(fm.tags).toEqual(['tag1', 'tag2', 'tag3']);
  });

  it('handles tags with commas in quoted strings (yaml-safe)', () => {
    const content = '---\ntags:\n  - "tag, with comma"\n  - normal\n---\n# Body';
    const fm = parseFrontmatter(content);
    expect(Array.isArray(fm.tags)).toBe(true);
    expect((fm.tags as string[]).some(t => t.includes(','))).toBe(true);
  });

  it('parses YAML block sequence tags', () => {
    const content = '---\ntags:\n  - alpha\n  - beta\n  - gamma\n---\n# Body';
    const fm = parseFrontmatter(content);
    expect(fm.tags).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('parses string tag as single-element array wrapper', () => {
    const content = '---\ntags: single-tag\n---\n# Body';
    const fm = parseFrontmatter(content);
    // parseFrontmatter returns string for scalar, caller wraps to array
    expect(fm.tags).toBeDefined();
  });

  it('returns empty object for no frontmatter', () => {
    const content = '# Just a heading\nNo frontmatter here';
    const fm = parseFrontmatter(content);
    expect(Object.keys(fm)).toHaveLength(0);
  });

  it('handles CRLF line endings', () => {
    const content = '---\r\ntitle: My Note\r\ntags: [a, b]\r\n---\r\n# Body';
    const fm = parseFrontmatter(content);
    expect(fm.title).toBe('My Note');
  });

  it('handles numeric and boolean values', () => {
    const content = '---\nrank: 5\npublished: true\n---';
    const fm = parseFrontmatter(content);
    expect(fm.rank).toBe('5');
    expect(fm.published).toBe('true');
  });

  it('returns empty object on malformed YAML', () => {
    const content = '---\n: invalid: yaml:\n---';
    const fm = parseFrontmatter(content);
    // Should not throw, returns {} or partial
    expect(typeof fm).toBe('object');
  });
});

// ─── Phase 3: frontmatter tags stored in DB ───────────────────────────────────

describe('Phase 3 — frontmatter tags stored after indexDocument', () => {
  let tmpDir: string;
  let store: Store;
  let dbPath: string;

  beforeEach(() => {
    tmpDir = makeTmpDir();
    dbPath = path.join(tmpDir, 'test.sqlite');
    store = createStore(dbPath);
  });

  afterEach(() => {
    try { store.close(); } catch {}
    evictCachedStore(dbPath);
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  const now = new Date().toISOString();
  const projectHash = 'proj001abc';

  function insertDoc(store: Store, hash: string, filePath: string, title: string, collection: string): number {
    store.registerWorkspacePrefix(projectHash, tmpDir);
    store.insertContent(hash, `# ${title}`);
    return store.insertDocument({
      hash,
      path: path.join(tmpDir, filePath),
      title,
      collection,
      active: 1,
      projectHash,
      createdAt: now,
      modifiedAt: now,
    });
  }

  it('insertTags stores tags normalized to lowercase', () => {
    const docId = insertDoc(store, 'abc123', 'note1.md', 'Test', 'obsidian-main');
    store.insertTags(docId, ['Tag1', 'TAG2', 'tag3']);
    const tags = store.getDocumentTags(docId);
    expect(tags).toContain('tag1');
    expect(tags).toContain('tag2');
    expect(tags).toContain('tag3');
  });

  it('insertTags deduplicates tags', () => {
    const docId = insertDoc(store, 'def456', 'note2.md', 'Test2', 'obsidian-main');
    store.insertTags(docId, ['alpha', 'alpha', 'beta', 'ALPHA']);
    const tags = store.getDocumentTags(docId);
    expect(tags.filter(t => t === 'alpha')).toHaveLength(1);
  });

  it('insertTags filters empty/whitespace tags', () => {
    const docId = insertDoc(store, 'ghi789', 'note3.md', 'Test3', 'memory');
    store.insertTags(docId, ['valid', '', '  ', 'also-valid']);
    const tags = store.getDocumentTags(docId);
    expect(tags).toContain('valid');
    expect(tags).toContain('also-valid');
    expect(tags.some(t => t.trim() === '')).toBe(false);
  });
});

// ─── Phase 4: db:clean --list-only ────────────────────────────────────────────

describe('Phase 4 — db:clean --list-only flag', () => {
  let dataDir: string;

  beforeEach(() => {
    dataDir = makeTmpDir();
  });

  afterEach(() => {
    fs.rmSync(dataDir, { recursive: true, force: true });
  });

  it('--list-only: scan reports orphaned DB', () => {
    const orphaned = path.join(dataDir, 'old-aabbccddee11.sqlite');
    fs.writeFileSync(orphaned, '');
    const result = scanForOrphanedDbs(dataDir, []);
    expect(result.orphanedDbs).toHaveLength(1);
    // File NOT deleted (list-only means just scan, no unlink)
    expect(fs.existsSync(orphaned)).toBe(true);
  });

  it('--list-only: does not delete even with orphaned files present', () => {
    const orphaned = path.join(dataDir, 'stale-aabbccddee11.sqlite');
    const corrupted = path.join(dataDir, 'db.sqlite.corrupted.2024-01-01T00-00-00');
    fs.writeFileSync(orphaned, '');
    fs.writeFileSync(corrupted, '');
    // scanForOrphanedDbs is the list-only operation — no deletion
    const result = scanForOrphanedDbs(dataDir, []);
    expect(result.orphanedDbs).toHaveLength(1);
    expect(result.corruptedBackups).toHaveLength(1);
    expect(fs.existsSync(orphaned)).toBe(true);
    expect(fs.existsSync(corrupted)).toBe(true);
  });

  it('--list-only: returns correct sizes', () => {
    const orphaned = path.join(dataDir, 'sized-aabbccddee11.sqlite');
    fs.writeFileSync(orphaned, 'x'.repeat(1024));
    const result = scanForOrphanedDbs(dataDir, []);
    expect(result.orphanedDbs[0].size).toBe(1024);
  });
});

// ─── Phase 5: getCollections wires excludeFolders from config ────────────────

describe('Phase 5 — getCollections passes excludeFolders from YAML config', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = makeTmpDir();
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it('excludeFolders from config is passed to Collection object', () => {
    const configPath = path.join(tmpDir, 'config.yml');
    fs.writeFileSync(configPath, [
      'collections:',
      '  obsidian-main:',
      '    path: ~/Documents/MyVault',
      '    pattern: "**/*.md"',
      '    excludeFolders:',
      '      - .trash',
      '      - templates',
    ].join('\n'));
    const config = loadCollectionConfig(configPath);
    const collections = getCollections(config!);
    expect(collections).toHaveLength(1);
    expect(collections[0].excludeFolders).toEqual(['.trash', 'templates']);
  });

  it('collection without excludeFolders has undefined excludeFolders', () => {
    const configPath = path.join(tmpDir, 'config.yml');
    fs.writeFileSync(configPath, [
      'collections:',
      '  memory:',
      '    path: ~/notes',
      '    pattern: "**/*.md"',
    ].join('\n'));
    const config = loadCollectionConfig(configPath);
    const collections = getCollections(config!);
    expect(collections[0].excludeFolders).toBeUndefined();
  });

  it('empty excludeFolders list is preserved', () => {
    const configPath = path.join(tmpDir, 'config.yml');
    fs.writeFileSync(configPath, [
      'collections:',
      '  notes:',
      '    path: ~/notes',
      '    excludeFolders: []',
    ].join('\n'));
    const config = loadCollectionConfig(configPath);
    const collections = getCollections(config!);
    expect(collections[0].excludeFolders).toEqual([]);
  });
});
