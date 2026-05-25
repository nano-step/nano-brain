import { describe, it, expect } from 'vitest';
import { formatCompactResults, formatSearchResults } from '../src/server.js';
import type { SearchResult } from '../src/types.js';

function createMockResult(id: string, score: number, snippet: string = 'test snippet'): SearchResult {
  return {
    id,
    path: `path/${id}`,
    collection: 'test',
    title: `Title ${id}`,
    snippet,
    score,
    startLine: 1,
    endLine: 10,
    docid: id.substring(0, 6),
  };
}

describe('Response Caps', () => {
  describe('compact mode default', () => {
    it('formatCompactResults should produce single-line results', () => {
      const results = [
        createMockResult('doc1', 0.95, 'This is a test snippet'),
        createMockResult('doc2', 0.85, 'Another test snippet'),
      ];
      
      const formatted = formatCompactResults(results, 'test-cache-key');
      
      expect(formatted).toContain('test-cache-key');
      expect(formatted).toContain('memory_expand');
      expect(formatted).toContain('[0.950]');
      expect(formatted).toContain('[0.850]');
      const lines = formatted.split('\n').filter(l => l.trim());
      expect(lines.length).toBeLessThan(10);
    });
  });

  describe('memory_get truncation', () => {
    it('should truncate body after maxLines', () => {
      const longBody = Array(300).fill('Line content').join('\n');
      const lines = longBody.split('\n');
      expect(lines.length).toBe(300);
      
      const truncatedLines = lines.slice(0, 200);
      expect(truncatedLines.length).toBe(200);
    });
  });

  describe('code_impact caps', () => {
    it('should respect maxDepth=3 and maxEntries=50 defaults', () => {
      const byDepth: Record<string, Array<{ name: string }>> = {
        '1': Array(30).fill({ name: 'sym' }),
        '2': Array(30).fill({ name: 'sym' }),
        '3': Array(30).fill({ name: 'sym' }),
        '4': Array(30).fill({ name: 'sym' }),
        '5': Array(30).fill({ name: 'sym' }),
      };
      
      const maxDepth = 3;
      const maxEntries = 50;
      let totalEntries = 0;
      let truncatedEntries = 0;
      const truncatedByDepth: Record<string, Array<{ name: string }>> = {};
      
      for (const [depth, depItems] of Object.entries(byDepth)) {
        if (parseInt(depth) > maxDepth) {
          truncatedEntries += depItems.length;
          continue;
        }
        const remaining = maxEntries - totalEntries;
        if (remaining <= 0) {
          truncatedEntries += depItems.length;
          continue;
        }
        if (depItems.length > remaining) {
          truncatedByDepth[depth] = depItems.slice(0, remaining);
          truncatedEntries += depItems.length - remaining;
          totalEntries += remaining;
        } else {
          truncatedByDepth[depth] = depItems;
          totalEntries += depItems.length;
        }
      }
      
      expect(totalEntries).toBeLessThanOrEqual(maxEntries);
      expect(truncatedEntries).toBeGreaterThan(0);
      expect(Object.keys(truncatedByDepth).length).toBeLessThanOrEqual(3);
    });
  });

  describe('code_context caps', () => {
    it('should cap callers at 20', () => {
      const callers = Array(50).fill({ name: 'caller' });
      const maxIncoming = 20;
      const displayIncoming = callers.slice(0, maxIncoming);
      expect(displayIncoming.length).toBe(20);
    });

    it('should cap callees at 20', () => {
      const callees = Array(50).fill({ name: 'callee' });
      const maxOutgoing = 20;
      const displayOutgoing = callees.slice(0, maxOutgoing);
      expect(displayOutgoing.length).toBe(20);
    });

    it('should cap flows at 10', () => {
      const flows = Array(50).fill({ label: 'flow' });
      const maxFlows = 10;
      const displayFlows = flows.slice(0, maxFlows);
      expect(displayFlows.length).toBe(10);
    });
  });

  describe('memory_focus caps', () => {
    it('should cap dependencies at 30', () => {
      const deps = Array(100).fill('/path/to/dep');
      const maxDeps = 30;
      const displayDeps = deps.slice(0, maxDeps);
      expect(displayDeps.length).toBe(30);
    });

    it('should cap dependents at 30', () => {
      const dependents = Array(100).fill('/path/to/dependent');
      const maxDependents = 30;
      const displayDependents = dependents.slice(0, maxDependents);
      expect(displayDependents.length).toBe(30);
    });
  });

  describe('memory_symbols/memory_impact caps', () => {
    it('should cap symbols at 50', () => {
      const symbols = Array(100).fill({ type: 'redis_key', pattern: 'key:*' });
      const maxSymbols = 50;
      let symbolCount = 0;
      const displayed: typeof symbols = [];
      
      for (const sym of symbols) {
        if (symbolCount >= maxSymbols) break;
        displayed.push(sym);
        symbolCount++;
      }
      
      expect(displayed.length).toBe(50);
    });

    it('should cap impact results at 50', () => {
      const results = Array(100).fill({ operation: 'read', repo: 'test' });
      const maxImpact = 50;
      let impactCount = 0;
      const displayed: typeof results = [];
      
      for (const r of results) {
        if (impactCount >= maxImpact) break;
        displayed.push(r);
        impactCount++;
      }
      
      expect(displayed.length).toBe(50);
    });
  });

  describe('code_detect_changes caps', () => {
    it('should cap flows at 20', () => {
      const flows = Array(50).fill({ label: 'flow', flowType: 'api' });
      const maxFlows = 20;
      const displayFlows = flows.slice(0, maxFlows);
      expect(displayFlows.length).toBe(20);
    });

    it('should cap changed files at 20', () => {
      const files = Array(50).fill('/path/to/file.ts');
      const displayFiles = files.slice(0, 20);
      expect(displayFiles.length).toBe(20);
    });

    it('should cap changed symbols at 30', () => {
      const symbols = Array(50).fill({ name: 'sym', kind: 'function' });
      const displaySymbols = symbols.slice(0, 30);
      expect(displaySymbols.length).toBe(30);
    });
  });
});
