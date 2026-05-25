import { describe, it, expect } from 'vitest';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs';
import { resolveConfiguredWorkspace } from '../src/server/bootstrap.js';
import { resolveWorkspaceDbPath } from '../src/store.js';

describe('resolveConfiguredWorkspace', () => {
  describe('empty configured list — no restriction', () => {
    it('returns the root unchanged when list is empty', () => {
      const result = resolveConfiguredWorkspace('/any/path', []);
      expect(result).toEqual({ resolved: '/any/path', fallback: false });
    });

    it('returns cwd-style paths unchanged', () => {
      const result = resolveConfiguredWorkspace('/home/user/project', []);
      expect(result).toEqual({ resolved: '/home/user/project', fallback: false });
    });
  });

  describe('exact match', () => {
    it('returns the root unchanged when it is in the configured list', () => {
      const result = resolveConfiguredWorkspace('/repo/app', ['/repo/app', '/repo/other']);
      expect(result).toEqual({ resolved: '/repo/app', fallback: false });
    });

    it('matches the first workspace in a single-element list', () => {
      const result = resolveConfiguredWorkspace('/configured', ['/configured']);
      expect(result).toEqual({ resolved: '/configured', fallback: false });
    });
  });

  describe('longest-prefix match', () => {
    it('falls back to longest prefix when root is a subdirectory of a configured workspace', () => {
      const result = resolveConfiguredWorkspace('/repo/sub-project', ['/repo', '/other']);
      expect(result).toEqual({ resolved: '/repo', fallback: true });
    });

    it('selects the longest prefix when multiple configured workspaces are prefixes', () => {
      const result = resolveConfiguredWorkspace(
        '/a/b/c/d',
        ['/a', '/a/b', '/a/b/c']
      );
      expect(result).toEqual({ resolved: '/a/b/c', fallback: true });
    });

    it('does not treat partial directory names as a prefix match', () => {
      // /repo-other should NOT match /repo as a prefix — falls to first-workspace fallback
      const result = resolveConfiguredWorkspace('/repo-other', ['/first-workspace', '/repo']);
      expect(result).toEqual({ resolved: '/first-workspace', fallback: true });
    });

    it('handles path separator boundary correctly', () => {
      // /foo/bar should match /foo if /foo is configured
      const result = resolveConfiguredWorkspace('/foo/bar', ['/foo']);
      expect(result).toEqual({ resolved: '/foo', fallback: true });
    });
  });

  describe('no-prefix fallback to first configured workspace', () => {
    it('falls back to first workspace when no prefix match exists', () => {
      const result = resolveConfiguredWorkspace('/unrelated/path', ['/configured/a', '/configured/b']);
      expect(result).toEqual({ resolved: '/configured/a', fallback: true });
    });

    it('falls back to first workspace in a single-element list when no match', () => {
      const result = resolveConfiguredWorkspace('/not-here', ['/configured']);
      expect(result).toEqual({ resolved: '/configured', fallback: true });
    });
  });

  describe('path separator handling', () => {
    it('treats paths with trailing sep as non-exact (no false exact match)', () => {
      // '/repo/' with trailing slash should not exact-match '/repo'
      const result = resolveConfiguredWorkspace('/repo/', ['/repo']);
      // /repo/ starts with /repo + / so it IS a prefix match
      expect(result.resolved).toBe('/repo');
    });
  });
});

describe('workspace guard → DB path integration', () => {
  it('3.2: unconfigured root resolves to fallback DB, not unconfigured DB', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nb-guard-test-'));
    try {
      const configuredWs = path.join(tmpDir, 'configured');
      const unconfiguredWs = path.join(tmpDir, 'unconfigured');

      const { resolved } = resolveConfiguredWorkspace(unconfiguredWs, [configuredWs]);
      expect(resolved).toBe(configuredWs);

      const dbPath = resolveWorkspaceDbPath(tmpDir, resolved);
      const wrongDbPath = resolveWorkspaceDbPath(tmpDir, unconfiguredWs);
      expect(dbPath).not.toBe(wrongDbPath);
      // The DB that would be created is for the configured workspace, not the unconfigured one
      expect(dbPath).toContain('configured');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('3.3: configured root is opened as-is (no fallback)', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nb-guard-test-'));
    try {
      const configuredWs = path.join(tmpDir, 'my-project');

      const { resolved, fallback } = resolveConfiguredWorkspace(configuredWs, [configuredWs]);
      expect(resolved).toBe(configuredWs);
      expect(fallback).toBe(false);

      const dbPath = resolveWorkspaceDbPath(tmpDir, resolved);
      expect(dbPath).toContain('my-project');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('3.4: empty config.workspaces — any root is used unchanged', () => {
    const anyRoot = '/arbitrary/path/not/in/config';
    const { resolved, fallback } = resolveConfiguredWorkspace(anyRoot, []);
    expect(resolved).toBe(anyRoot);
    expect(fallback).toBe(false);
  });
});
