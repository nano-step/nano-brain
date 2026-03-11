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
    upsertQueryCluster: vi.fn(),
    clearClusterTransitions: () => {},
    upsertClusterTransition: vi.fn(),
    getQueryClusters: () => [],
    getTransitionsFrom: () => [],
    getTelemetryStats: () => ({ queryCount: 0, expandCount: 0 }),
    getGlobalLearning: () => [],
    clearGlobalTransitions: vi.fn(),
    upsertGlobalTransition: vi.fn(),
    getGlobalTransitions: () => [],
    getClusterTransitions: () => [],
    getWorkspaceProfile: () => null,
    getTelemetryTopKeywords: () => [],
    ...overrides,
  } as unknown as Store;
}

describe('GlobalOverlay', () => {
  describe('updateGlobalTransitions', () => {
    it('should aggregate transitions from multiple workspaces', async () => {
      const upsertGlobalTransition = vi.fn();
      const clearGlobalTransitions = vi.fn();
      
      const store = createMockStore({
        getGlobalLearning: () => [
          { parameter_name: 'ws:workspace1', value: 1, confidence: 1 },
          { parameter_name: 'ws:workspace2', value: 1, confidence: 1 },
        ],
        getClusterTransitions: (wsHash: string) => {
          if (wsHash === 'workspace1') {
            return [
              { from_cluster_id: 0, to_cluster_id: 1, frequency: 10, probability: 0.5 },
              { from_cluster_id: 1, to_cluster_id: 2, frequency: 5, probability: 0.3 },
            ];
          }
          if (wsHash === 'workspace2') {
            return [
              { from_cluster_id: 0, to_cluster_id: 1, frequency: 20, probability: 0.6 },
              { from_cluster_id: 0, to_cluster_id: 2, frequency: 8, probability: 0.4 },
            ];
          }
          return [];
        },
        clearGlobalTransitions,
        upsertGlobalTransition,
      });
      
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      await analyzer.updateGlobalTransitions();
      
      expect(clearGlobalTransitions).toHaveBeenCalled();
      expect(upsertGlobalTransition).toHaveBeenCalled();
      
      const calls = upsertGlobalTransition.mock.calls;
      const t01 = calls.find((c: number[]) => c[0] === 0 && c[1] === 1);
      expect(t01).toBeDefined();
      expect(t01![2]).toBe(30);
    });

    it('should handle empty workspace list', async () => {
      const upsertGlobalTransition = vi.fn();
      const clearGlobalTransitions = vi.fn();
      
      const store = createMockStore({
        getGlobalLearning: () => [],
        getTelemetryStats: () => ({ queryCount: 0, expandCount: 0 }),
        clearGlobalTransitions,
        upsertGlobalTransition,
      });
      
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      await analyzer.updateGlobalTransitions();
      
      expect(clearGlobalTransitions).toHaveBeenCalled();
      expect(upsertGlobalTransition).not.toHaveBeenCalled();
    });
  });

  describe('transferGlobalToWorkspace', () => {
    it('should copy global clusters to new workspace', async () => {
      const upsertQueryCluster = vi.fn();
      const upsertClusterTransition = vi.fn();
      
      const globalClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'query A', query_count: 100 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'query B', query_count: 80 },
      ];
      
      const globalTransitions = [
        { from_cluster_id: 0, to_cluster_id: 1, frequency: 50, probability: 0.7 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => wsHash === 'global' ? globalClusters : [],
        getGlobalTransitions: () => globalTransitions,
        upsertQueryCluster,
        upsertClusterTransition,
      });
      
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      await analyzer.transferGlobalToWorkspace('new-workspace');
      
      expect(upsertQueryCluster).toHaveBeenCalledTimes(2);
      expect(upsertClusterTransition).toHaveBeenCalledTimes(1);
      
      expect(upsertQueryCluster).toHaveBeenCalledWith(
        0, JSON.stringify([1, 0, 0]), 'query A', 0, 'new-workspace'
      );
    });

    it('should not transfer if workspace already has clusters', async () => {
      const upsertQueryCluster = vi.fn();
      
      const existingClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'existing', query_count: 5 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => wsHash === 'ws-hash' ? existingClusters : [],
        upsertQueryCluster,
      });
      
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      await analyzer.transferGlobalToWorkspace('ws-hash');
      
      expect(upsertQueryCluster).not.toHaveBeenCalled();
    });

    it('should not transfer if no global clusters exist', async () => {
      const upsertQueryCluster = vi.fn();
      
      const store = createMockStore({
        getQueryClusters: () => [],
        upsertQueryCluster,
      });
      
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      await analyzer.transferGlobalToWorkspace('new-workspace');
      
      expect(upsertQueryCluster).not.toHaveBeenCalled();
    });
  });

  describe('overlay blending', () => {
    it('should use workspace transitions when workspace has 50+ queries', async () => {
      const wsClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'ws query', query_count: 50 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'ws target', query_count: 30 },
      ];
      
      const wsTransitions = [
        { to_cluster_id: 1, frequency: 25, probability: 0.8 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => wsHash === 'ws-hash' ? wsClusters : [],
        getTransitionsFrom: () => wsTransitions,
        getTelemetryStats: () => ({ queryCount: 100, expandCount: 20 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [0.9, 0.1, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test', 'ws-hash');
      
      expect(result.length).toBeGreaterThan(0);
      expect(result[0].query).toBe('ws target');
    });

    it('should use global transitions when workspace has <50 queries', async () => {
      const globalClusters = [
        { cluster_id: 0, centroid_embedding: JSON.stringify([1, 0, 0]), representative_query: 'global query', query_count: 500 },
        { cluster_id: 1, centroid_embedding: JSON.stringify([0, 1, 0]), representative_query: 'global target', query_count: 300 },
      ];
      
      const globalTransitions = [
        { to_cluster_id: 1, frequency: 200, probability: 0.7 },
      ];
      
      const store = createMockStore({
        getQueryClusters: (wsHash: string) => wsHash === 'global' ? globalClusters : [],
        getTransitionsFrom: () => globalTransitions,
        getTelemetryStats: () => ({ queryCount: 10, expandCount: 2 }),
      });
      
      const embedder = async (texts: string[]) => texts.map(() => [0.9, 0.1, 0]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      const result = await analyzer.predictNext('test', 'new-workspace');
      
      expect(result.length).toBeGreaterThan(0);
      expect(result[0].query).toBe('global target');
    });
  });

  describe('getStore', () => {
    it('should return the store instance', () => {
      const store = createMockStore();
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      
      expect(analyzer.getStore()).toBe(store);
    });
  });
});
