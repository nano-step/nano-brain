import { log } from './logger.js';
import type { Store, ProactiveConfig } from './types.js';

export interface QueryCluster {
  clusterId: number;
  centroid: number[];
  representativeQuery: string;
  queryCount: number;
}

export interface ClusterTransition {
  fromClusterId: number;
  toClusterId: number;
  frequency: number;
  probability: number;
}

export interface EmbedderFn {
  (texts: string[]): Promise<number[][]>;
}

export class SequenceAnalyzer {
  private store: Store;
  private config: ProactiveConfig;
  private embedder: EmbedderFn | null;

  constructor(store: Store, config: ProactiveConfig, embedder?: EmbedderFn) {
    this.store = store;
    this.config = config;
    this.embedder = embedder ?? null;
  }

  async runAnalysisCycle(workspaceHash: string): Promise<void> {
    if (!this.embedder) {
      log('sequence', 'No embedder configured, skipping analysis');
      return;
    }
    
    const recentQueries = this.store.getRecentTelemetryQueries(workspaceHash, 1000);
    if (recentQueries.length < this.config.min_queries_for_prediction) {
      log('sequence', 'Not enough queries (' + recentQueries.length + '), need ' + this.config.min_queries_for_prediction);
      return;
    }
    
    const queryTexts = recentQueries.map(q => q.query_text);
    const embeddings = await this.embedder(queryTexts);
    
    const k = Math.min(this.config.cluster_count, Math.floor(recentQueries.length / 2));
    const { centroids, assignments } = this.kMeansClustering(embeddings, k);
    
    this.store.clearQueryClusters(workspaceHash);
    for (let i = 0; i < centroids.length; i++) {
      const members = recentQueries.filter((_, idx) => assignments[idx] === i);
      if (members.length === 0) continue;
      const representative = members[0].query_text;
      this.store.upsertQueryCluster(i, JSON.stringify(centroids[i]), representative, members.length, workspaceHash);
    }
    
    const chains = this.store.getChainsByWorkspace(workspaceHash, 1000);
    const chainMap = new Map<string, Array<{ query_id: string; position: number }>>();
    for (const row of chains) {
      if (!chainMap.has(row.chain_id)) chainMap.set(row.chain_id, []);
      chainMap.get(row.chain_id)!.push({ query_id: row.query_id, position: row.position });
    }
    
    const queryToCluster = new Map<string, number>();
    for (let i = 0; i < recentQueries.length; i++) {
      queryToCluster.set(recentQueries[i].query_id, assignments[i]);
    }
    
    const clusterChains: Array<Array<number>> = [];
    for (const [, members] of chainMap) {
      members.sort((a, b) => a.position - b.position);
      const clusterSeq = members
        .map(m => queryToCluster.get(m.query_id))
        .filter((c): c is number => c !== undefined);
      if (clusterSeq.length >= 2) clusterChains.push(clusterSeq);
    }
    
    const transitions = this.buildTransitions(clusterChains);
    
    this.store.clearClusterTransitions(workspaceHash);
    for (const t of transitions) {
      this.store.upsertClusterTransition(t.fromClusterId, t.toClusterId, t.frequency, t.probability, workspaceHash);
    }
    
    log('sequence', 'Analysis complete: ' + centroids.length + ' clusters, ' + transitions.length + ' transitions');
  }

  kMeansClustering(embeddings: number[][], k: number, maxIterations: number = 100): { centroids: number[][]; assignments: number[] } {
    const n = embeddings.length;
    if (n === 0 || k <= 0) return { centroids: [], assignments: [] };
    const actualK = Math.min(k, n);
    
    const centroids: number[][] = [];
    const dim = embeddings[0].length;
    
    centroids.push([...embeddings[Math.floor(Math.random() * n)]]);
    
    for (let c = 1; c < actualK; c++) {
      const distances = embeddings.map(emb => {
        let minDist = Infinity;
        for (const cent of centroids) {
          const dist = this.euclideanDistSq(emb, cent);
          if (dist < minDist) minDist = dist;
        }
        return minDist;
      });
      const totalDist = distances.reduce((a, b) => a + b, 0);
      let r = Math.random() * totalDist;
      for (let i = 0; i < n; i++) {
        r -= distances[i];
        if (r <= 0) { centroids.push([...embeddings[i]]); break; }
      }
      if (centroids.length <= c) centroids.push([...embeddings[Math.floor(Math.random() * n)]]);
    }
    
    let assignments = new Array(n).fill(0);
    for (let iter = 0; iter < maxIterations; iter++) {
      const newAssignments = embeddings.map(emb => {
        let bestIdx = 0, bestDist = Infinity;
        for (let c = 0; c < centroids.length; c++) {
          const dist = this.euclideanDistSq(emb, centroids[c]);
          if (dist < bestDist) { bestDist = dist; bestIdx = c; }
        }
        return bestIdx;
      });
      
      let changed = false;
      for (let i = 0; i < n; i++) {
        if (newAssignments[i] !== assignments[i]) { changed = true; break; }
      }
      assignments = newAssignments;
      if (!changed) break;
      
      for (let c = 0; c < centroids.length; c++) {
        const members = embeddings.filter((_, i) => assignments[i] === c);
        if (members.length === 0) {
          let maxDist = -1, farthestIdx = 0;
          for (let i = 0; i < n; i++) {
            const dist = this.euclideanDistSq(embeddings[i], centroids[assignments[i]]);
            if (dist > maxDist) { maxDist = dist; farthestIdx = i; }
          }
          centroids[c] = [...embeddings[farthestIdx]];
        } else {
          centroids[c] = new Array(dim).fill(0);
          for (const m of members) {
            for (let d = 0; d < dim; d++) centroids[c][d] += m[d];
          }
          for (let d = 0; d < dim; d++) centroids[c][d] /= members.length;
        }
      }
    }
    
    return { centroids, assignments };
  }

  buildTransitions(chains: Array<Array<number>>): ClusterTransition[] {
    const counts = new Map<string, number>();
    const fromCounts = new Map<number, number>();
    
    for (const chain of chains) {
      for (let i = 0; i < chain.length - 1; i++) {
        const from = chain[i];
        const to = chain[i + 1];
        const key = from + ':' + to;
        counts.set(key, (counts.get(key) ?? 0) + 1);
        fromCounts.set(from, (fromCounts.get(from) ?? 0) + 1);
      }
    }
    
    const transitions: ClusterTransition[] = [];
    for (const [key, freq] of counts) {
      const [fromStr, toStr] = key.split(':');
      const from = parseInt(fromStr);
      const to = parseInt(toStr);
      const total = fromCounts.get(from) ?? 1;
      transitions.push({
        fromClusterId: from,
        toClusterId: to,
        frequency: freq,
        probability: freq / total,
      });
    }
    
    return transitions;
  }

  cosineSimilarity(a: number[], b: number[]): number {
    let dot = 0, normA = 0, normB = 0;
    for (let i = 0; i < a.length; i++) {
      dot += a[i] * b[i];
      normA += a[i] * a[i];
      normB += b[i] * b[i];
    }
    return dot / (Math.sqrt(normA) * Math.sqrt(normB) || 1);
  }

  findNearestCluster(embedding: number[], centroids: number[][]): { clusterId: number; similarity: number } {
    let bestId = 0, bestSim = -1;
    for (let i = 0; i < centroids.length; i++) {
      const sim = this.cosineSimilarity(embedding, centroids[i]);
      if (sim > bestSim) { bestSim = sim; bestId = i; }
    }
    return { clusterId: bestId, similarity: bestSim };
  }

  private euclideanDistSq(a: number[], b: number[]): number {
    let sum = 0;
    for (let i = 0; i < a.length; i++) {
      const d = a[i] - b[i];
      sum += d * d;
    }
    return sum;
  }

  computeFrecencyWeight(transitionTimestamp: string, now: Date, halfLifeDays: number = 30): number {
    const ageMs = now.getTime() - new Date(transitionTimestamp).getTime();
    const ageDays = ageMs / (1000 * 60 * 60 * 24);
    return Math.exp(-0.693 * ageDays / halfLifeDays);
  }

  buildTransitionsWithFrecency(
    chains: Array<Array<{ clusterId: number; timestamp: string }>>,
    halfLifeDays: number = 30
  ): ClusterTransition[] {
    const now = new Date();
    const weightedCounts = new Map<string, number>();
    const fromWeightedCounts = new Map<number, number>();
    
    for (const chain of chains) {
      for (let i = 0; i < chain.length - 1; i++) {
        const from = chain[i].clusterId;
        const to = chain[i + 1].clusterId;
        const timestamp = chain[i + 1].timestamp;
        const weight = this.computeFrecencyWeight(timestamp, now, halfLifeDays);
        const key = from + ':' + to;
        weightedCounts.set(key, (weightedCounts.get(key) ?? 0) + weight);
        fromWeightedCounts.set(from, (fromWeightedCounts.get(from) ?? 0) + weight);
      }
    }
    
    const transitions: ClusterTransition[] = [];
    for (const [key, weightedFreq] of weightedCounts) {
      const [fromStr, toStr] = key.split(':');
      const from = parseInt(fromStr);
      const to = parseInt(toStr);
      const total = fromWeightedCounts.get(from) ?? 1;
      transitions.push({
        fromClusterId: from,
        toClusterId: to,
        frequency: Math.round(weightedFreq),
        probability: weightedFreq / total,
      });
    }
    
    return transitions;
  }

  async predictNext(queryText: string, workspaceHash: string, limit: number = 3): Promise<Array<{
    query: string;
    confidence: number;
    reasoning: string;
    relatedDocids: string[];
  }>> {
    if (!this.embedder) return [];
    
    const clusters = this.store.getQueryClusters(workspaceHash);
    if (clusters.length === 0) {
      const globalClusters = this.store.getQueryClusters('global');
      if (globalClusters.length === 0) return [];
      return this.predictFromClusters(queryText, globalClusters, 'global', limit, 1.0);
    }
    
    const telemetryStats = this.store.getTelemetryStats(workspaceHash);
    const queryCount = telemetryStats.queryCount;
    
    if (queryCount >= this.config.min_queries_for_prediction) {
      return this.predictFromClusters(queryText, clusters, workspaceHash, limit, 1.0);
    }
    
    const globalClusters = this.store.getQueryClusters('global');
    if (globalClusters.length === 0) {
      return this.predictFromClusters(queryText, clusters, workspaceHash, limit, 1.0);
    }
    
    const workspaceWeight = queryCount / this.config.min_queries_for_prediction;
    const globalWeight = 1 - workspaceWeight;
    
    const wsPredictions = await this.predictFromClusters(queryText, clusters, workspaceHash, limit, workspaceWeight);
    const globalPredictions = await this.predictFromClusters(queryText, globalClusters, 'global', limit, globalWeight);
    
    const merged = new Map<string, { query: string; confidence: number; reasoning: string; relatedDocids: string[] }>();
    for (const p of wsPredictions) {
      merged.set(p.query, p);
    }
    for (const p of globalPredictions) {
      const existing = merged.get(p.query);
      if (existing) {
        existing.confidence = Math.max(existing.confidence, p.confidence);
      } else {
        merged.set(p.query, p);
      }
    }
    
    return [...merged.values()]
      .sort((a, b) => b.confidence - a.confidence)
      .slice(0, limit);
  }

  private async predictFromClusters(
    queryText: string,
    clusters: Array<{ cluster_id: number; centroid_embedding: string; representative_query: string; query_count: number }>,
    workspaceHash: string,
    limit: number,
    weight: number
  ): Promise<Array<{ query: string; confidence: number; reasoning: string; relatedDocids: string[] }>> {
    if (!this.embedder || clusters.length === 0) return [];
    
    const [embedding] = await this.embedder([queryText]);
    
    const centroids = clusters.map(c => JSON.parse(c.centroid_embedding) as number[]);
    const { clusterId, similarity } = this.findNearestCluster(embedding, centroids);
    
    const transitions = this.store.getTransitionsFrom(clusterId, workspaceHash, limit * 2);
    
    const sourceCluster = clusters.find(c => c.cluster_id === clusterId);
    const sourceQuery = sourceCluster?.representative_query ?? queryText;
    
    return transitions
      .filter(t => t.probability >= this.config.confidence_threshold)
      .slice(0, limit)
      .map(t => {
        const targetCluster = clusters.find(c => c.cluster_id === t.to_cluster_id);
        const adjustedConfidence = t.probability * weight * similarity;
        return {
          query: targetCluster?.representative_query ?? 'unknown',
          confidence: adjustedConfidence,
          reasoning: 'Users who asked about "' + sourceQuery + '" often ask about this next (' + Math.round(t.probability * 100) + '% of the time)',
          relatedDocids: this.getRelatedDocids(t.to_cluster_id, workspaceHash),
        };
      });
  }

  getRelatedDocids(clusterId: number, workspaceHash: string): string[] {
    return [];
  }

  async updateGlobalTransitions(): Promise<void> {
    const allWorkspaceTransitions = new Map<string, { frequency: number; fromTotal: number }>();
    const fromTotals = new Map<number, number>();
    
    const workspaceProfiles = this.store.getGlobalLearning();
    const workspaceHashes = new Set<string>();
    for (const p of workspaceProfiles) {
      if (p.parameter_name.startsWith('ws:')) {
        workspaceHashes.add(p.parameter_name.slice(3));
      }
    }
    
    if (workspaceHashes.size === 0) {
      const telemetryStats = this.store.getTelemetryStats('global');
      if (telemetryStats.queryCount > 0) {
        workspaceHashes.add('global');
      }
    }
    
    for (const wsHash of workspaceHashes) {
      const transitions = this.store.getClusterTransitions(wsHash);
      for (const t of transitions) {
        const key = t.from_cluster_id + ':' + t.to_cluster_id;
        const existing = allWorkspaceTransitions.get(key);
        if (existing) {
          existing.frequency += t.frequency;
        } else {
          allWorkspaceTransitions.set(key, { frequency: t.frequency, fromTotal: 0 });
        }
        fromTotals.set(t.from_cluster_id, (fromTotals.get(t.from_cluster_id) ?? 0) + t.frequency);
      }
    }
    
    for (const [key, data] of allWorkspaceTransitions) {
      const [fromStr] = key.split(':');
      const fromId = parseInt(fromStr);
      data.fromTotal = fromTotals.get(fromId) ?? 1;
    }
    
    this.store.clearGlobalTransitions();
    for (const [key, data] of allWorkspaceTransitions) {
      const [fromStr, toStr] = key.split(':');
      const fromId = parseInt(fromStr);
      const toId = parseInt(toStr);
      const probability = data.frequency / data.fromTotal;
      this.store.upsertGlobalTransition(fromId, toId, data.frequency, probability);
    }
    
    log('sequence', 'Global transitions updated: ' + allWorkspaceTransitions.size + ' transitions');
  }

  async transferGlobalToWorkspace(workspaceHash: string): Promise<void> {
    const existingClusters = this.store.getQueryClusters(workspaceHash);
    if (existingClusters.length > 0) return;
    
    const globalClusters = this.store.getQueryClusters('global');
    if (globalClusters.length === 0) return;
    
    for (const c of globalClusters) {
      this.store.upsertQueryCluster(c.cluster_id, c.centroid_embedding, c.representative_query, 0, workspaceHash);
    }
    
    const globalTransitions = this.store.getGlobalTransitions();
    for (const t of globalTransitions) {
      this.store.upsertClusterTransition(t.from_cluster_id, t.to_cluster_id, 0, t.probability, workspaceHash);
    }
    
    log('sequence', 'Transferred global patterns to workspace ' + workspaceHash + ': ' + globalClusters.length + ' clusters, ' + globalTransitions.length + ' transitions');
  }

  getStore(): Store {
    return this.store;
  }
}
