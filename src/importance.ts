import { log } from './logger.js';
import type { Store, ImportanceConfig } from './types.js';
import { DEFAULT_IMPORTANCE_CONFIG } from './types.js';

export interface ImportanceParams {
  usageCount: number;
  entityDensity: number;
  daysSinceAccess: number;
  connectionCount: number;
  maxUsage: number;
  maxConnections: number;
}

export class ImportanceScorer {
  private store: Store;
  private config: ImportanceConfig;
  private scoreCache: Map<string, number> = new Map();

  constructor(store: Store, config?: Partial<ImportanceConfig>) {
    this.store = store;
    this.config = { ...DEFAULT_IMPORTANCE_CONFIG, ...config };
  }

  calculateScore(params: ImportanceParams): number {
    const w = this.config.formula_weights;
    
    const usageNorm = params.maxUsage > 0 ? params.usageCount / params.maxUsage : 0;
    const entityNorm = Math.min(params.entityDensity, 1.0);
    const recencyNorm = Math.exp(-0.693 * params.daysSinceAccess / this.config.decay_half_life_days);
    const connectionNorm = params.maxConnections > 0 ? params.connectionCount / params.maxConnections : 0;
    
    return w.usage * usageNorm + w.entity_density * entityNorm + w.recency * recencyNorm + w.connections * connectionNorm;
  }

  applyBoost(searchScore: number, importanceScore: number): number {
    return searchScore * (1 + this.config.weight * importanceScore);
  }

  getScore(docid: string): number {
    return this.scoreCache.get(docid) ?? 0;
  }

  async recalculateAll(): Promise<number> {
    log('importance', 'Recalculating importance scores');
    this.scoreCache.clear();
    
    if (!this.store) {
      return 0;
    }
    
    try {
      const docs = this.store.getActiveDocumentsWithAccess();
      if (docs.length === 0) {
        return 0;
      }
      
      const maxUsage = Math.max(...docs.map(d => d.access_count || 0), 1);
      
      const connectionCounts = new Map<number, number>();
      for (const doc of docs) {
        connectionCounts.set(doc.id, this.store.getConnectionCount(doc.id));
      }
      const maxConnections = Math.max(...connectionCounts.values(), 1);
      
      const now = Date.now();
      
      for (const doc of docs) {
        const tagCount = this.store.getTagCountForDocument(doc.id);
        const entityDensity = Math.min(tagCount / 5.0, 1.0);
        
        let daysSinceAccess = 0;
        if (doc.last_accessed_at) {
          const lastAccess = new Date(doc.last_accessed_at).getTime();
          daysSinceAccess = Math.max(0, (now - lastAccess) / (1000 * 60 * 60 * 24));
        }
        
        const score = this.calculateScore({
          usageCount: doc.access_count || 0,
          entityDensity,
          daysSinceAccess,
          connectionCount: connectionCounts.get(doc.id) || 0,
          maxUsage,
          maxConnections,
        });
        
        const docid = doc.hash.substring(0, 6);
        this.scoreCache.set(docid, score);
      }
      
      log('importance', `Recalculated ${docs.length} importance scores`);
      return docs.length;
    } catch (err) {
      log('importance', `Failed to recalculate: ${err instanceof Error ? err.message : String(err)}`, 'warn');
      return 0;
    }
  }

  getConfig(): ImportanceConfig {
    return this.config;
  }
}
