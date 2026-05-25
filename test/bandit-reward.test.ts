import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createStore, evictCachedStore } from '../src/store.js';
import { ThompsonSampler, type BanditConfig } from '../src/bandits.js';
import type { Store } from '../src/types.js';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

describe('Bandit Reward Loop', () => {
  let store: Store;
  let dbPath: string;

  beforeEach(() => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nano-brain-bandit-test-'));
    dbPath = path.join(tmpDir, 'test.db');
    store = createStore(dbPath);
  });

  afterEach(() => {
    evictCachedStore(dbPath);
    const dir = path.dirname(dbPath);
    if (fs.existsSync(dir)) {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  describe('config variant storage', () => {
    it('should store config_variant in search telemetry', () => {
      const configVariant = JSON.stringify({ rrf_k: 60, centrality_weight: 0.1 });
      store.logSearchQuery(
        'query-123',
        'test query',
        'hybrid',
        configVariant,
        ['doc1', 'doc2'],
        100,
        'session-1',
        'cache-key-1',
        'workspace-hash'
      );

      const retrieved = store.getConfigVariantByCacheKey('cache-key-1');
      expect(retrieved).toBe(configVariant);
    });

    it('should store null config_variant when no bandit is used', () => {
      store.logSearchQuery(
        'query-456',
        'test query',
        'fts',
        null,
        ['doc1'],
        50,
        'session-2',
        'cache-key-2',
        'workspace-hash'
      );

      const retrieved = store.getConfigVariantByCacheKey('cache-key-2');
      expect(retrieved).toBeNull();
    });
  });

  describe('getConfigVariantByCacheKey', () => {
    it('should return config_variant for existing cache key', () => {
      const configVariant = JSON.stringify({ rrf_k: 30 });
      store.logSearchQuery(
        'query-1',
        'search test',
        'hybrid',
        configVariant,
        ['doc1'],
        100,
        'session-1',
        'my-cache-key',
        'workspace'
      );

      const result = store.getConfigVariantByCacheKey('my-cache-key');
      expect(result).toBe(configVariant);
    });

    it('should return null for non-existent cache key', () => {
      const result = store.getConfigVariantByCacheKey('non-existent-key');
      expect(result).toBeNull();
    });

    it('should return a config_variant when multiple queries share cache key', () => {
      store.logSearchQuery('q1', 'query 1', 'hybrid', '{"rrf_k":30}', [], 100, 's1', 'shared-key', 'ws');
      store.logSearchQuery('q2', 'query 2', 'hybrid', '{"rrf_k":60}', [], 100, 's1', 'shared-key', 'ws');

      const result = store.getConfigVariantByCacheKey('shared-key');
      expect(result).not.toBeNull();
      expect(['{"rrf_k":30}', '{"rrf_k":60}']).toContain(result);
    });
  });

  describe('getConfigVariantById', () => {
    it('should return config_variant for existing telemetry id', () => {
      const configVariant = JSON.stringify({ centrality_weight: 0.05 });
      store.logSearchQuery(
        'query-id-1',
        'test',
        'hybrid',
        configVariant,
        [],
        50,
        'session',
        null,
        'workspace'
      );

      const recentQueries = store.getRecentQueries('session');
      expect(recentQueries.length).toBeGreaterThan(0);
      const telemetryId = recentQueries[0].id;

      const result = store.getConfigVariantById(telemetryId);
      expect(result).toBe(configVariant);
    });

    it('should return null for non-existent telemetry id', () => {
      const result = store.getConfigVariantById(999999);
      expect(result).toBeNull();
    });
  });

  describe('reward recording integration', () => {
    const createTestSampler = (): ThompsonSampler => {
      const configs: BanditConfig[] = [{
        parameterName: 'rrf_k',
        variants: [
          { value: 30, successes: 10, failures: 10 },
          { value: 60, successes: 10, failures: 10 },
          { value: 90, successes: 10, failures: 10 },
        ],
        minObservations: 5,
        dampeningFactor: 0.1,
        bounds: { min: 10, max: 100 },
      }];
      return new ThompsonSampler(configs);
    };

    it('should record positive reward when expanding results', () => {
      const sampler = createTestSampler();
      const initialState = sampler.getState();
      const initialSuccesses = initialState[0].variants[1].successes;

      const configVariant = JSON.stringify({ rrf_k: 60 });
      store.logSearchQuery('q1', 'test', 'hybrid', configVariant, [], 100, 's1', 'expand-cache', 'ws');

      store.logSearchExpand('expand-cache', [1, 2]);

      const retrievedVariant = store.getConfigVariantByCacheKey('expand-cache');
      expect(retrievedVariant).toBe(configVariant);

      if (retrievedVariant) {
        const variants = JSON.parse(retrievedVariant) as Record<string, number>;
        for (const [paramName, value] of Object.entries(variants)) {
          sampler.recordReward(paramName, value, true);
        }
      }

      const newState = sampler.getState();
      expect(newState[0].variants[1].successes).toBe(initialSuccesses + 1);
    });

    it('should record negative reward on query reformulation', () => {
      const sampler = createTestSampler();
      const initialState = sampler.getState();
      const initialFailures = initialState[0].variants[0].failures;

      const configVariant = JSON.stringify({ rrf_k: 30 });
      store.logSearchQuery('q1', 'original query', 'hybrid', configVariant, [], 100, 'session-reform', null, 'ws');

      const recentQueries = store.getRecentQueries('session-reform');
      expect(recentQueries.length).toBe(1);
      const reformulatedId = recentQueries[0].id;

      store.markReformulation(reformulatedId);

      const retrievedVariant = store.getConfigVariantById(reformulatedId);
      expect(retrievedVariant).toBe(configVariant);

      if (retrievedVariant) {
        const variants = JSON.parse(retrievedVariant) as Record<string, number>;
        for (const [paramName, value] of Object.entries(variants)) {
          sampler.recordReward(paramName, value, false);
        }
      }

      const newState = sampler.getState();
      expect(newState[0].variants[0].failures).toBe(initialFailures + 1);
    });

    it('should not affect sampler when config_variant is null', () => {
      const sampler = createTestSampler();
      const initialState = JSON.stringify(sampler.getState());

      store.logSearchQuery('q1', 'test', 'fts', null, [], 100, 's1', 'null-cache', 'ws');

      const retrievedVariant = store.getConfigVariantByCacheKey('null-cache');
      expect(retrievedVariant).toBeNull();

      const newState = JSON.stringify(sampler.getState());
      expect(newState).toBe(initialState);
    });

    it('should handle multiple parameters in config_variant', () => {
      const configs: BanditConfig[] = [
        {
          parameterName: 'rrf_k',
          variants: [{ value: 60, successes: 5, failures: 5 }],
          minObservations: 5,
          dampeningFactor: 0.1,
          bounds: { min: 10, max: 100 },
        },
        {
          parameterName: 'centrality_weight',
          variants: [{ value: 0.1, successes: 5, failures: 5 }],
          minObservations: 5,
          dampeningFactor: 0.1,
          bounds: { min: 0, max: 1 },
        },
      ];
      const sampler = new ThompsonSampler(configs);

      const configVariant = JSON.stringify({ rrf_k: 60, centrality_weight: 0.1 });
      store.logSearchQuery('q1', 'test', 'hybrid', configVariant, [], 100, 's1', 'multi-param', 'ws');

      const retrievedVariant = store.getConfigVariantByCacheKey('multi-param');
      const variants = JSON.parse(retrievedVariant!) as Record<string, number>;

      for (const [paramName, value] of Object.entries(variants)) {
        sampler.recordReward(paramName, value, true);
      }

      const state = sampler.getState();
      expect(state[0].variants[0].successes).toBe(6);
      expect(state[1].variants[0].successes).toBe(6);
    });
  });

  describe('end-to-end reward loop', () => {
    it('should complete full positive reward cycle: search → expand → reward', () => {
      const configs: BanditConfig[] = [{
        parameterName: 'rrf_k',
        variants: [
          { value: 30, successes: 1, failures: 1 },
          { value: 60, successes: 1, failures: 1 },
        ],
        minObservations: 100,
        dampeningFactor: 0.1,
        bounds: { min: 10, max: 100 },
      }];
      const sampler = new ThompsonSampler(configs, 42);

      const selectedVariants = sampler.selectSearchConfig();
      const configVariantJson = JSON.stringify(selectedVariants);

      store.logSearchQuery(
        'query-e2e-1',
        'end to end test',
        'hybrid+rerank',
        configVariantJson,
        ['doc1', 'doc2', 'doc3'],
        150,
        'e2e-session',
        'e2e-cache-key',
        'e2e-workspace'
      );

      store.logSearchExpand('e2e-cache-key', [1]);

      const variant = store.getConfigVariantByCacheKey('e2e-cache-key');
      expect(variant).toBe(configVariantJson);

      const parsed = JSON.parse(variant!) as Record<string, number>;
      for (const [param, value] of Object.entries(parsed)) {
        sampler.recordReward(param, value, true);
      }

      const state = sampler.getState();
      const selectedValue = selectedVariants.rrf_k;
      const variantState = state[0].variants.find(v => v.value === selectedValue);
      expect(variantState!.successes).toBe(2);
    });

    it('should complete full negative reward cycle: search → reformulate → reward', () => {
      const configs: BanditConfig[] = [{
        parameterName: 'rrf_k',
        variants: [
          { value: 30, successes: 1, failures: 1 },
          { value: 60, successes: 1, failures: 1 },
        ],
        minObservations: 100,
        dampeningFactor: 0.1,
        bounds: { min: 10, max: 100 },
      }];
      const sampler = new ThompsonSampler(configs, 42);

      const selectedVariants = sampler.selectSearchConfig();
      const configVariantJson = JSON.stringify(selectedVariants);

      store.logSearchQuery(
        'query-reform-1',
        'initial search query',
        'hybrid',
        configVariantJson,
        ['doc1'],
        100,
        'reform-session',
        null,
        'reform-workspace'
      );

      const recentQueries = store.getRecentQueries('reform-session');
      const reformulatedId = recentQueries[0].id;

      store.markReformulation(reformulatedId);

      const variant = store.getConfigVariantById(reformulatedId);
      expect(variant).toBe(configVariantJson);

      const parsed = JSON.parse(variant!) as Record<string, number>;
      for (const [param, value] of Object.entries(parsed)) {
        sampler.recordReward(param, value, false);
      }

      const state = sampler.getState();
      const selectedValue = selectedVariants.rrf_k;
      const variantState = state[0].variants.find(v => v.value === selectedValue);
      expect(variantState!.failures).toBe(2);
    });
  });
});
