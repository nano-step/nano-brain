import { describe, it, expect } from 'vitest';
import { SequenceAnalyzer } from '../src/sequence-analyzer.js';
import type { Store, ProactiveConfig } from '../src/types.js';

const mockConfig: ProactiveConfig = {
  enabled: true,
  chain_timeout_ms: 300000,
  min_queries_for_prediction: 10,
  max_suggestions: 5,
  confidence_threshold: 0.3,
  cluster_count: 5,
  analysis_interval_ms: 1800000,
};

function createMockStore(): Partial<Store> {
  return {
    getRecentTelemetryQueries: () => [],
    getChainsByWorkspace: () => [],
    clearQueryClusters: () => {},
    upsertQueryCluster: () => {},
    clearClusterTransitions: () => {},
    upsertClusterTransition: () => {},
  };
}

describe('SequenceAnalyzer', () => {
  describe('cosineSimilarity', () => {
    it('should return 1.0 for identical vectors', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const v = [1, 2, 3, 4, 5];
      expect(analyzer.cosineSimilarity(v, v)).toBeCloseTo(1.0, 5);
    });

    it('should return 0.0 for orthogonal vectors', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const a = [1, 0, 0];
      const b = [0, 1, 0];
      expect(analyzer.cosineSimilarity(a, b)).toBeCloseTo(0.0, 5);
    });

    it('should return -1.0 for opposite vectors', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const a = [1, 2, 3];
      const b = [-1, -2, -3];
      expect(analyzer.cosineSimilarity(a, b)).toBeCloseTo(-1.0, 5);
    });

    it('should handle zero vectors gracefully', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const a = [0, 0, 0];
      const b = [1, 2, 3];
      const result = analyzer.cosineSimilarity(a, b);
      expect(Number.isFinite(result)).toBe(true);
    });
  });

  describe('kMeansClustering', () => {
    it('should return empty result for empty input', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const result = analyzer.kMeansClustering([], 3);
      expect(result.centroids).toHaveLength(0);
      expect(result.assignments).toHaveLength(0);
    });

    it('should return empty result for k <= 0', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const embeddings = [[1, 0], [0, 1]];
      const result = analyzer.kMeansClustering(embeddings, 0);
      expect(result.centroids).toHaveLength(0);
      expect(result.assignments).toHaveLength(0);
    });

    it('should cap k to n when k > n', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const embeddings = [[1, 0], [0, 1], [1, 1]];
      const result = analyzer.kMeansClustering(embeddings, 10);
      expect(result.centroids.length).toBeLessThanOrEqual(3);
      expect(result.assignments).toHaveLength(3);
    });

    it('should correctly separate two obvious clusters', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const cluster1 = [[0, 0], [0.1, 0.1], [0.05, 0.05]];
      const cluster2 = [[10, 10], [10.1, 10.1], [9.9, 9.9]];
      const embeddings = [...cluster1, ...cluster2];
      
      const result = analyzer.kMeansClustering(embeddings, 2, 50);
      
      const cluster1Assignments = result.assignments.slice(0, 3);
      const cluster2Assignments = result.assignments.slice(3, 6);
      
      const allSameInCluster1 = cluster1Assignments.every(a => a === cluster1Assignments[0]);
      const allSameInCluster2 = cluster2Assignments.every(a => a === cluster2Assignments[0]);
      const differentClusters = cluster1Assignments[0] !== cluster2Assignments[0];
      
      expect(allSameInCluster1).toBe(true);
      expect(allSameInCluster2).toBe(true);
      expect(differentClusters).toBe(true);
    });

    it('should converge within maxIterations', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const embeddings = Array.from({ length: 20 }, (_, i) => [i % 5, Math.floor(i / 5)]);
      const result = analyzer.kMeansClustering(embeddings, 3, 100);
      expect(result.centroids).toHaveLength(3);
      expect(result.assignments).toHaveLength(20);
    });
  });

  describe('buildTransitions', () => {
    it('should return empty array for empty chains', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const result = analyzer.buildTransitions([]);
      expect(result).toHaveLength(0);
    });

    it('should return empty array for single-element chains', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const result = analyzer.buildTransitions([[0], [1], [2]]);
      expect(result).toHaveLength(0);
    });

    it('should calculate correct probabilities for simple chain', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const chains = [[0, 1, 2]];
      const result = analyzer.buildTransitions(chains);
      
      expect(result).toHaveLength(2);
      
      const t01 = result.find(t => t.fromClusterId === 0 && t.toClusterId === 1);
      const t12 = result.find(t => t.fromClusterId === 1 && t.toClusterId === 2);
      
      expect(t01).toBeDefined();
      expect(t01!.probability).toBeCloseTo(1.0, 5);
      expect(t01!.frequency).toBe(1);
      
      expect(t12).toBeDefined();
      expect(t12!.probability).toBeCloseTo(1.0, 5);
      expect(t12!.frequency).toBe(1);
    });

    it('should calculate correct probabilities for chain with repeated transitions', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const chains = [[0, 1, 0, 2]];
      const result = analyzer.buildTransitions(chains);
      
      const t01 = result.find(t => t.fromClusterId === 0 && t.toClusterId === 1);
      const t02 = result.find(t => t.fromClusterId === 0 && t.toClusterId === 2);
      const t10 = result.find(t => t.fromClusterId === 1 && t.toClusterId === 0);
      
      expect(t01).toBeDefined();
      expect(t01!.probability).toBeCloseTo(0.5, 5);
      expect(t01!.frequency).toBe(1);
      
      expect(t02).toBeDefined();
      expect(t02!.probability).toBeCloseTo(0.5, 5);
      expect(t02!.frequency).toBe(1);
      
      expect(t10).toBeDefined();
      expect(t10!.probability).toBeCloseTo(1.0, 5);
      expect(t10!.frequency).toBe(1);
    });

    it('should aggregate frequencies across multiple chains', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const chains = [[0, 1], [0, 1], [0, 2]];
      const result = analyzer.buildTransitions(chains);
      
      const t01 = result.find(t => t.fromClusterId === 0 && t.toClusterId === 1);
      const t02 = result.find(t => t.fromClusterId === 0 && t.toClusterId === 2);
      
      expect(t01).toBeDefined();
      expect(t01!.frequency).toBe(2);
      expect(t01!.probability).toBeCloseTo(2/3, 5);
      
      expect(t02).toBeDefined();
      expect(t02!.frequency).toBe(1);
      expect(t02!.probability).toBeCloseTo(1/3, 5);
    });
  });

  describe('findNearestCluster', () => {
    it('should find the nearest cluster by cosine similarity', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const centroids = [[1, 0, 0], [0, 1, 0], [0, 0, 1]];
      
      const result1 = analyzer.findNearestCluster([0.9, 0.1, 0], centroids);
      expect(result1.clusterId).toBe(0);
      
      const result2 = analyzer.findNearestCluster([0.1, 0.9, 0], centroids);
      expect(result2.clusterId).toBe(1);
      
      const result3 = analyzer.findNearestCluster([0, 0.1, 0.9], centroids);
      expect(result3.clusterId).toBe(2);
    });

    it('should return similarity score', () => {
      const analyzer = new SequenceAnalyzer(createMockStore() as Store, mockConfig);
      const centroids = [[1, 0]];
      
      const result = analyzer.findNearestCluster([1, 0], centroids);
      expect(result.similarity).toBeCloseTo(1.0, 5);
    });
  });

  describe('runAnalysisCycle', () => {
    it('should skip if no embedder configured', async () => {
      const store = createMockStore() as Store;
      const analyzer = new SequenceAnalyzer(store, mockConfig);
      await analyzer.runAnalysisCycle('test-workspace');
    });

    it('should skip if not enough queries', async () => {
      const store = createMockStore() as Store;
      store.getRecentTelemetryQueries = () => [
        { id: 1, query_id: 'q1', query_text: 'test', timestamp: '2024-01-01', session_id: 's1' },
      ];
      
      const embedder = async (texts: string[]) => texts.map(() => [0, 1, 2]);
      const analyzer = new SequenceAnalyzer(store, mockConfig, embedder);
      
      await analyzer.runAnalysisCycle('test-workspace');
    });
  });
});
