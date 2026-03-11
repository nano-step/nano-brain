import { describe, it, expect } from 'vitest';
import { detectQueryChains } from '../src/telemetry.js';

describe('proactive-foundation', () => {
  describe('detectQueryChains', () => {
    it('groups queries within 5-min window into single chain', () => {
      const queries = [
        { id: 1, query_id: 'q1', query_text: 'first query', timestamp: '2024-01-01T10:00:00Z', session_id: 's1' },
        { id: 2, query_id: 'q2', query_text: 'second query', timestamp: '2024-01-01T10:02:00Z', session_id: 's1' },
        { id: 3, query_id: 'q3', query_text: 'third query', timestamp: '2024-01-01T10:04:00Z', session_id: 's1' },
      ];
      const chains = detectQueryChains(queries, 300000);
      expect(chains).toHaveLength(1);
      expect(chains[0].queries).toHaveLength(3);
      expect(chains[0].queries[0].queryId).toBe('q1');
      expect(chains[0].queries[1].queryId).toBe('q2');
      expect(chains[0].queries[2].queryId).toBe('q3');
    });

    it('separates queries across 5-min gap into different chains', () => {
      const queries = [
        { id: 1, query_id: 'q1', query_text: 'first query', timestamp: '2024-01-01T10:00:00Z', session_id: 's1' },
        { id: 2, query_id: 'q2', query_text: 'second query', timestamp: '2024-01-01T10:02:00Z', session_id: 's1' },
        { id: 3, query_id: 'q3', query_text: 'third query', timestamp: '2024-01-01T10:10:00Z', session_id: 's1' },
      ];
      const chains = detectQueryChains(queries, 300000);
      expect(chains).toHaveLength(2);
      expect(chains[0].queries).toHaveLength(2);
      expect(chains[1].queries).toHaveLength(1);
      expect(chains[0].queries[0].queryId).toBe('q1');
      expect(chains[0].queries[1].queryId).toBe('q2');
      expect(chains[1].queries[0].queryId).toBe('q3');
    });

    it('creates chain of 1 for single query', () => {
      const queries = [
        { id: 1, query_id: 'q1', query_text: 'only query', timestamp: '2024-01-01T10:00:00Z', session_id: 's1' },
      ];
      const chains = detectQueryChains(queries, 300000);
      expect(chains).toHaveLength(1);
      expect(chains[0].queries).toHaveLength(1);
      expect(chains[0].queries[0].queryId).toBe('q1');
    });

    it('separates queries with different session_ids into different chains', () => {
      const queries = [
        { id: 1, query_id: 'q1', query_text: 'first query', timestamp: '2024-01-01T10:00:00Z', session_id: 's1' },
        { id: 2, query_id: 'q2', query_text: 'second query', timestamp: '2024-01-01T10:01:00Z', session_id: 's2' },
        { id: 3, query_id: 'q3', query_text: 'third query', timestamp: '2024-01-01T10:02:00Z', session_id: 's2' },
      ];
      const chains = detectQueryChains(queries, 300000);
      expect(chains).toHaveLength(2);
      expect(chains[0].queries).toHaveLength(1);
      expect(chains[0].queries[0].queryId).toBe('q1');
      expect(chains[1].queries).toHaveLength(2);
      expect(chains[1].queries[0].queryId).toBe('q2');
      expect(chains[1].queries[1].queryId).toBe('q3');
    });

    it('returns empty array for empty input', () => {
      const chains = detectQueryChains([], 300000);
      expect(chains).toHaveLength(0);
    });

    it('sorts queries by timestamp before grouping', () => {
      const queries = [
        { id: 3, query_id: 'q3', query_text: 'third', timestamp: '2024-01-01T10:04:00Z', session_id: 's1' },
        { id: 1, query_id: 'q1', query_text: 'first', timestamp: '2024-01-01T10:00:00Z', session_id: 's1' },
        { id: 2, query_id: 'q2', query_text: 'second', timestamp: '2024-01-01T10:02:00Z', session_id: 's1' },
      ];
      const chains = detectQueryChains(queries, 300000);
      expect(chains).toHaveLength(1);
      expect(chains[0].queries[0].queryId).toBe('q1');
      expect(chains[0].queries[1].queryId).toBe('q2');
      expect(chains[0].queries[2].queryId).toBe('q3');
    });

    it('generates unique chainId for each chain', () => {
      const queries = [
        { id: 1, query_id: 'q1', query_text: 'first', timestamp: '2024-01-01T10:00:00Z', session_id: 's1' },
        { id: 2, query_id: 'q2', query_text: 'second', timestamp: '2024-01-01T10:10:00Z', session_id: 's1' },
      ];
      const chains = detectQueryChains(queries, 300000);
      expect(chains).toHaveLength(2);
      expect(chains[0].chainId).toBeDefined();
      expect(chains[1].chainId).toBeDefined();
      expect(chains[0].chainId).not.toBe(chains[1].chainId);
    });
  });

  describe('keyword extraction', () => {
    it('filters stopwords from query text', () => {
      const stopwords = new Set([
        'a', 'an', 'the', 'is', 'are', 'was', 'were', 'be', 'been', 'being',
        'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would', 'could', 'should',
        'to', 'of', 'in', 'for', 'on', 'with', 'at', 'by', 'from', 'as', 'into',
        'and', 'but', 'or', 'nor', 'so', 'yet', 'both', 'either', 'neither',
        'not', 'only', 'own', 'same', 'than', 'too', 'very', 'just',
        'i', 'me', 'my', 'we', 'our', 'you', 'your', 'he', 'him', 'his', 'she', 'her',
        'it', 'its', 'they', 'them', 'their', 'what', 'which', 'who', 'this', 'that',
        'am', 'if', 'then', 'else', 'when', 'where', 'why', 'how', 'all', 'any',
      ]);
      const queryText = 'how to implement authentication in the application';
      const tokens = queryText.toLowerCase().split(/\s+/).filter(t => t.length > 2 && !stopwords.has(t));
      expect(tokens).toContain('implement');
      expect(tokens).toContain('authentication');
      expect(tokens).toContain('application');
      expect(tokens).not.toContain('how');
      expect(tokens).not.toContain('the');
    });

    it('filters short tokens (length <= 2)', () => {
      const queryText = 'a to be or not to be';
      const tokens = queryText.toLowerCase().split(/\s+/).filter(t => t.length > 2);
      expect(tokens).toEqual(['not']);
    });
  });
});
