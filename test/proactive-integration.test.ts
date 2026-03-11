import { describe, it, expect } from 'vitest';
import { SequenceAnalyzer } from '../src/sequence-analyzer.js';
import { detectQueryChains } from '../src/telemetry.js';
import { DEFAULT_PROACTIVE_CONFIG } from '../src/types.js';

describe('Proactive Intelligence Integration', () => {
  it('end-to-end: queries → chains → clusters → transitions → prediction', () => {
    const queries = [
      { id: 1, query_id: 'q1', query_text: 'auth handler', timestamp: '2026-01-01T10:00:00Z', session_id: 's1' },
      { id: 2, query_id: 'q2', query_text: 'token refresh', timestamp: '2026-01-01T10:02:00Z', session_id: 's1' },
      { id: 3, query_id: 'q3', query_text: 'middleware order', timestamp: '2026-01-01T10:04:00Z', session_id: 's1' },
      { id: 4, query_id: 'q4', query_text: 'authentication', timestamp: '2026-01-01T11:00:00Z', session_id: 's2' },
      { id: 5, query_id: 'q5', query_text: 'refresh token', timestamp: '2026-01-01T11:02:00Z', session_id: 's2' },
    ];
    
    const chains = detectQueryChains(queries, 300000);
    expect(chains.length).toBe(2);
    expect(chains[0].queries.length).toBe(3);
    expect(chains[1].queries.length).toBe(2);
  });

  it('k-means produces correct cluster count', () => {
    const analyzer = new SequenceAnalyzer(null as any, DEFAULT_PROACTIVE_CONFIG);
    
    const embeddings = [
      [1, 0, 0], [1.1, 0.1, 0], [0.9, -0.1, 0],
      [0, 1, 0], [0.1, 1.1, 0], [-0.1, 0.9, 0],
    ];
    
    const { centroids, assignments } = analyzer.kMeansClustering(embeddings, 2);
    expect(centroids.length).toBe(2);
    expect(assignments[0]).toBe(assignments[1]);
    expect(assignments[0]).toBe(assignments[2]);
    expect(assignments[3]).toBe(assignments[4]);
    expect(assignments[3]).toBe(assignments[5]);
    expect(assignments[0]).not.toBe(assignments[3]);
  });

  it('transition probabilities are correct', () => {
    const analyzer = new SequenceAnalyzer(null as any, DEFAULT_PROACTIVE_CONFIG);
    
    const transitions = analyzer.buildTransitions([[0, 1, 2], [0, 1, 3]]);
    
    const t01 = transitions.find(t => t.fromClusterId === 0 && t.toClusterId === 1);
    expect(t01?.probability).toBe(1.0);
    expect(t01?.frequency).toBe(2);
    
    const t12 = transitions.find(t => t.fromClusterId === 1 && t.toClusterId === 2);
    expect(t12?.probability).toBe(0.5);
    
    const t13 = transitions.find(t => t.fromClusterId === 1 && t.toClusterId === 3);
    expect(t13?.probability).toBe(0.5);
  });

  it('frecency weight decays over time', () => {
    const analyzer = new SequenceAnalyzer(null as any, DEFAULT_PROACTIVE_CONFIG);
    const now = new Date('2026-03-11T00:00:00Z');
    
    const recentWeight = analyzer.computeFrecencyWeight('2026-03-10T00:00:00Z', now, 30);
    const oldWeight = analyzer.computeFrecencyWeight('2026-02-09T00:00:00Z', now, 30);
    
    expect(recentWeight).toBeGreaterThan(0.9);
    expect(oldWeight).toBeCloseTo(0.5, 1);
    expect(recentWeight).toBeGreaterThan(oldWeight);
  });

  it('buildTransitionsWithFrecency weights recent transitions higher', () => {
    const analyzer = new SequenceAnalyzer(null as any, DEFAULT_PROACTIVE_CONFIG);
    
    const chains = [
      [
        { clusterId: 0, timestamp: '2026-03-10T10:00:00Z' },
        { clusterId: 1, timestamp: '2026-03-10T10:01:00Z' },
      ],
      [
        { clusterId: 0, timestamp: '2026-01-01T10:00:00Z' },
        { clusterId: 2, timestamp: '2026-01-01T10:01:00Z' },
      ],
    ];
    
    const transitions = analyzer.buildTransitionsWithFrecency(chains, 30);
    
    const t01 = transitions.find(t => t.fromClusterId === 0 && t.toClusterId === 1);
    const t02 = transitions.find(t => t.fromClusterId === 0 && t.toClusterId === 2);
    
    expect(t01).toBeDefined();
    expect(t02).toBeDefined();
    expect(t01!.probability).toBeGreaterThan(t02!.probability);
  });
});
