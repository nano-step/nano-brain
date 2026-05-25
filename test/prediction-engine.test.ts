import { describe, it, expect, vi } from 'vitest';
import { SequenceAnalyzer } from '../src/sequence-analyzer.js';
import type { Store, ProactiveConfig } from '../src/types.js';

const mockConfig: ProactiveConfig = {
  enabled: true,
  chain_timeout_ms: 300000,
  min_queries_for_prediction: 50,
  max_suggestions: 5,
  confidence_threshold: 0.3,
  cluster_count: 5,
  analysis_interval_ms: 1800000,
};

function createMockStore(overrides: Partial<Store> = {}): Store {
  return {
    getRecentTelemetryQueries: () => [],
    getChainsByWorkspace: () => [],
    clearQueryClusters: () => {},
    upsertQueryCluster: () => {},
    clearClusterTransitions: () => {},
    upsertClusterTransition: () => {},
    getQueryClusters: () => [],
    getTransitionsFrom: () => [],
    getTelemetryStats: () => ({ queryCount: 0, expandCount: 0 }),
    getGlobalLearning: () => [],
    clearGlobalTransitions: () => {},
    upsertGlobalTransition: () => {},
    getGlobalTransitions: () => [],
    getClusterTransitions: () => [],
    getWorkspaceProfile: () => null,
    getTelemetryTopKeywords: () => [],
    ...overrides,
  } as unknown as Store;
}

describe('PredictionEngine', () => {
  describe('predictNext', () => {
    it('should return empty array when no embedder configured', async () => {
      const store = createMockStore();
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      
      const result = await analyzer.predictNext('test query', 'ws-hash');
      expect(result).toHaveLength(0);
    });

    it('should return empty array when no clusters exist', async () => {
      const store = createMockStore({
        getQueryClusters: () => [],
      });
      const embedder = async (texts: string[]) => texts.map(() => [0.1, 0.2, 0.3]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test query', 'ws-hash');
      expect(result).toHaveLength(0);
    });

    it('should return predictions from workspace clusters when enough queries', async () => {
      const clusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'auth login', query_count: 10 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'user profile', query_count: 8 },
        { cluster_id: 2, centroid_embedding: JSON.stringify([0, 0, 1]), representative_query: 'settings page', query_count: 5 },
      ];
      
      const transitions = [
        { to_cluster_id: 1, frequency: 5, probability: 0.6 },
        { to_cluster_id: 2, frequency: 3, probability: 0.4 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => wsHash === 'ws-hash' ? clusters : [],
        getTransitionsFrom: () => transitions,
        getTelemetryStats: () => ({ queryCount: 100, expandCount: 20 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [0.9, 0.1, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('login help', 'ws-hash', 3);
      
      expect(result.length).toBeGreaterThan(0);
      expect(result[0].query).toBe('user profile');
      expect(result[0].confidence).toBeGreaterThan(0);
      expect(result[0].reasoning).toContain('often ask about this next');
    });

    it('should filter predictions below confidence threshold', async () => {
      const clusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'query A', query_count: 10 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'query B', query_count: 8 },
      ];
      
      const transitions = [
        { to_cluster_id: 1, frequency: 1, probability: 0.1 },
      ];
      
      const store = createMockStore({
        getQueryClusters: () => clusters,
        getTransitionsFrom: () => transitions,
        getTelemetryStats: () => ({ queryCount: 100, expandCount: 20 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [1, 0, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test', 'ws-hash');
      expect(result).toHaveLength(0);
    });

    it('should respect limit parameter', async () => {
      const clusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'query A', query_count: 10 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'query B', query_count: 8 },
        { cluster_id: 2, centroid_embedding: JSON.stringify([0, 0, 1]), representative_query: 'query C', query_count: 5 },
        { cluster_id: 3, centroid_embedding: JSON.stringify([1, 1, 0]), representative_query: 'query D', query_count: 3 },
      ];
      
      const transitions = [
        { to_cluster_id: 1, frequency: 10, probability: 0.5 },
        { to_cluster_id: 2, frequency: 8, probability: 0.4 },
        { to_cluster_id: 3, frequency: 6, probability: 0.35 },
      ];
      
      const store = createMockStore({
        getQueryClusters: () => clusters,
        getTransitionsFrom: () => transitions,
        getTelemetryStats: () => ({ queryCount: 100, expandCount: 20 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [1, 0, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test', 'ws-hash', 2);
      expect(result.length).toBeLessThanOrEqual(2);
    });
  });

  describe('cold start handling', () => {
    it('should use global clusters when workspace has no clusters', async () => {
      const globalClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'global query A', query_count: 100 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'global query B', query_count: 80 },
      ];
      
      const transitions = [
        { to_cluster_id: 1, frequency: 50, probability: 0.7 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => wsHash === 'global' ? globalClusters : [],
        getTransitionsFrom: () => transitions,
        getTelemetryStats: () => ({ queryCount: 0, expandCount: 0 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [0.9, 0.1, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test query', 'new-workspace');
      
      expect(result.length).toBeGreaterThan(0);
      expect(result[0].query).toBe('global query B');
    });

    it('should blend workspace and global when workspace has few queries', async () => {
      const wsClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'ws query A', query_count: 5 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'ws query B', query_count: 3 },
      ];
      
      const globalClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'global query A', query_count: 100 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'global query B', query_count: 80 },
      ];
      
      const transitions = [
        { to_cluster_id: 1, frequency: 10, probability: 0.6 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => {
          if (wsHash === 'global') return globalClusters;
          if (wsHash === 'ws-hash') return wsClusters;
          return [];
        },
        getTransitionsFrom: () => transitions,
        getTelemetryStats: () => ({ queryCount: 25, expandCount: 5 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [0.9, 0.1, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test query', 'ws-hash');
      
      expect(result.length).toBeGreaterThan(0);
    });
  });

  describe('getRelatedDocids', () => {
    it('should return empty array (placeholder implementation)', () => {
      const store = createMockStore();
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      
      const result = analyzer.getRelatedDocids(0, 'ws-hash');
      expect(result).toEqual([]);
    });
  });
});
