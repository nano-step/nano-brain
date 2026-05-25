import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  levenshteinDistance,
  isPrefixMatch,
  areSimilar,
  findSimilarEntities,
  mergeEntities,
  runMergeCycle,
  MergeConfig,
  DEFAULT_MERGE_CONFIG,
  parseMergeConfig,
} from '../src/entity-merger.js'
import type { Store, MemoryEntity } from '../src/types.js'

function createMockStore(overrides: Partial<Store> = {}): Store {
  return {
    getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([]),
    getEntityEdgeCount: vi.fn().mockReturnValue(0),
    getEntityById: vi.fn().mockReturnValue(null),
    redirectEntityEdges: vi.fn(),
    deleteEntity: vi.fn(),
    deduplicateEdges: vi.fn(),
    getDb: vi.fn().mockReturnValue({
      transaction: (fn: () => void) => fn,
      prepare: vi.fn().mockReturnValue({ run: vi.fn() }),
    }),
    ...overrides,
  } as unknown as Store
}

function createEntity(id: number, name: string, type: string, projectHash: string, edgeCount = 0): MemoryEntity {
  return {
    id,
    name,
    type: type as MemoryEntity['type'],
    projectHash,
    firstLearnedAt: '2024-01-01T00:00:00Z',
    lastConfirmedAt: '2024-01-01T00:00:00Z',
  }
}

describe('entity-merger', () => {
  describe('levenshteinDistance', () => {
    it('returns 0 for identical strings', () => {
      expect(levenshteinDistance('hello', 'hello')).toBe(0)
    })

    it('returns 0 for case-insensitive identical strings', () => {
      expect(levenshteinDistance('Hello', 'hello')).toBe(0)
    })

    it('returns correct distance for single character difference', () => {
      expect(levenshteinDistance('cat', 'bat')).toBe(1)
    })

    it('returns correct distance for insertions', () => {
      expect(levenshteinDistance('cat', 'cats')).toBe(1)
    })

    it('returns correct distance for deletions', () => {
      expect(levenshteinDistance('cats', 'cat')).toBe(1)
    })

    it('returns correct distance for multiple edits', () => {
      expect(levenshteinDistance('kitten', 'sitting')).toBe(3)
    })

    it('handles empty strings', () => {
      expect(levenshteinDistance('', 'hello')).toBe(5)
      expect(levenshteinDistance('hello', '')).toBe(5)
      expect(levenshteinDistance('', '')).toBe(0)
    })
  })

  describe('isPrefixMatch', () => {
    it('returns true when shorter is prefix of longer with space', () => {
      expect(isPrefixMatch('Redis', 'Redis server')).toBe(true)
    })

    it('returns true when shorter is prefix of longer with hyphen', () => {
      expect(isPrefixMatch('Redis', 'Redis-cache')).toBe(true)
    })

    it('returns false when not a prefix', () => {
      expect(isPrefixMatch('Redis', 'Rediscover')).toBe(false)
    })

    it('returns false when shorter is longer', () => {
      expect(isPrefixMatch('Redis server', 'Redis')).toBe(false)
    })

    it('returns false when same length', () => {
      expect(isPrefixMatch('Redis', 'Redis')).toBe(false)
    })

    it('is case insensitive', () => {
      expect(isPrefixMatch('redis', 'REDIS server')).toBe(true)
    })
  })

  describe('areSimilar', () => {
    it('returns true for identical strings', () => {
      expect(areSimilar('Redis', 'Redis', 0.8)).toBe(true)
    })

    it('returns true for case-insensitive match', () => {
      expect(areSimilar('Redis', 'redis', 0.8)).toBe(true)
    })

    it('returns true for prefix match', () => {
      expect(areSimilar('Redis', 'Redis server', 0.8)).toBe(true)
    })

    it('returns true for small Levenshtein distance', () => {
      expect(areSimilar('Redis', 'Redi', 0.8)).toBe(true)
    })

    it('returns false for large length difference', () => {
      expect(areSimilar('Redis', 'PostgreSQL database server', 0.8)).toBe(false)
    })

    it('returns false for dissimilar strings', () => {
      expect(areSimilar('Redis', 'MySQL', 0.8)).toBe(false)
    })
  })

  describe('parseMergeConfig', () => {
    it('returns defaults when no config provided', () => {
      const config = parseMergeConfig()
      expect(config).toEqual(DEFAULT_MERGE_CONFIG)
    })

    it('merges partial config with defaults', () => {
      const config = parseMergeConfig({ enabled: false, batch_size: 100 })
      expect(config.enabled).toBe(false)
      expect(config.batch_size).toBe(100)
      expect(config.interval_ms).toBe(DEFAULT_MERGE_CONFIG.interval_ms)
      expect(config.similarity_threshold).toBe(DEFAULT_MERGE_CONFIG.similarity_threshold)
    })
  })

  describe('findSimilarEntities', () => {
    let mockStore: Store
    let config: MergeConfig

    beforeEach(() => {
      mockStore = createMockStore()
      config = { ...DEFAULT_MERGE_CONFIG }
    })

    it('returns empty array when no entities', () => {
      const result = findSimilarEntities(mockStore, config)
      expect(result).toEqual([])
    })

    it('returns empty array when no similar entities', () => {
      mockStore = createMockStore({
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([
          createEntity(1, 'Redis', 'service', 'proj1'),
          createEntity(2, 'MySQL', 'service', 'proj1'),
        ]),
        getEntityEdgeCount: vi.fn().mockReturnValue(0),
      })

      const result = findSimilarEntities(mockStore, config)
      expect(result).toEqual([])
    })

    it('groups similar entities by prefix match', () => {
      mockStore = createMockStore({
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([
          createEntity(1, 'Redis', 'service', 'proj1'),
          createEntity(2, 'Redis server', 'service', 'proj1'),
          createEntity(3, 'Redis cache', 'service', 'proj1'),
        ]),
        getEntityEdgeCount: vi.fn().mockReturnValue(0),
      })

      const result = findSimilarEntities(mockStore, config)
      expect(result.length).toBe(1)
      expect(result[0].canonicalName).toBe('Redis')
      expect(result[0].duplicateIds).toContain(2)
      expect(result[0].duplicateIds).toContain(3)
    })

    it('does not merge entities of different types', () => {
      mockStore = createMockStore({
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([
          createEntity(1, 'Redis', 'service', 'proj1'),
          createEntity(2, 'Redis', 'tool', 'proj1'),
        ]),
        getEntityEdgeCount: vi.fn().mockReturnValue(0),
      })

      const result = findSimilarEntities(mockStore, config)
      expect(result).toEqual([])
    })

    it('does not merge entities of different project_hash', () => {
      mockStore = createMockStore({
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([
          createEntity(1, 'Redis', 'service', 'proj1'),
          createEntity(2, 'Redis server', 'service', 'proj2'),
        ]),
        getEntityEdgeCount: vi.fn().mockReturnValue(0),
      })

      const result = findSimilarEntities(mockStore, config)
      expect(result).toEqual([])
    })

    it('prefers entity with more edges as canonical', () => {
      mockStore = createMockStore({
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([
          createEntity(1, 'Redis', 'service', 'proj1'),
          createEntity(2, 'Redi', 'service', 'proj1'),
        ]),
        getEntityEdgeCount: vi.fn().mockImplementation((id: number) => id === 2 ? 10 : 0),
      })

      const result = findSimilarEntities(mockStore, config)
      expect(result.length).toBe(1)
      expect(result[0].canonicalId).toBe(2)
      expect(result[0].duplicateIds).toContain(1)
    })

    it('respects batch_size limit', () => {
      const entities = Array.from({ length: 200 }, (_, i) => 
        createEntity(i + 1, `Entity${Math.floor(i / 2)}`, 'service', 'proj1')
      )
      mockStore = createMockStore({
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue(entities),
        getEntityEdgeCount: vi.fn().mockReturnValue(0),
      })
      config.batch_size = 10

      const result = findSimilarEntities(mockStore, config)
      expect(result.length).toBeLessThanOrEqual(10)
    })
  })

  describe('mergeEntities', () => {
    let mockStore: Store
    let mockDb: any

    beforeEach(() => {
      mockDb = {
        transaction: (fn: () => void) => fn,
        prepare: vi.fn().mockReturnValue({ run: vi.fn() }),
      }
      mockStore = createMockStore({
        getDb: vi.fn().mockReturnValue(mockDb),
        getEntityById: vi.fn().mockImplementation((id: number) => {
          if (id === 1) return createEntity(1, 'Redis', 'service', 'proj1')
          if (id === 2) return createEntity(2, 'Redis server', 'service', 'proj1')
          return null
        }),
        redirectEntityEdges: vi.fn(),
        deduplicateEdges: vi.fn(),
        deleteEntity: vi.fn(),
      })
    })

    it('redirects edges from duplicates to canonical', () => {
      mergeEntities(mockStore, 1, [2])
      expect(mockStore.redirectEntityEdges).toHaveBeenCalledWith(2, 1)
    })

    it('deduplicates edges after redirect', () => {
      mergeEntities(mockStore, 1, [2])
      expect(mockStore.deduplicateEdges).toHaveBeenCalledWith(1)
    })

    it('deletes duplicate entities', () => {
      mergeEntities(mockStore, 1, [2])
      expect(mockStore.deleteEntity).toHaveBeenCalledWith(2)
    })

    it('handles multiple duplicates', () => {
      mockStore = createMockStore({
        getDb: vi.fn().mockReturnValue(mockDb),
        getEntityById: vi.fn().mockImplementation((id: number) => {
          if (id === 1) return createEntity(1, 'Redis', 'service', 'proj1')
          if (id === 2) return createEntity(2, 'Redis server', 'service', 'proj1')
          if (id === 3) return createEntity(3, 'Redis cache', 'service', 'proj1')
          return null
        }),
        redirectEntityEdges: vi.fn(),
        deduplicateEdges: vi.fn(),
        deleteEntity: vi.fn(),
      })

      mergeEntities(mockStore, 1, [2, 3])
      expect(mockStore.redirectEntityEdges).toHaveBeenCalledTimes(2)
      expect(mockStore.deleteEntity).toHaveBeenCalledTimes(2)
    })

    it('skips if canonical entity not found', () => {
      mockStore = createMockStore({
        getDb: vi.fn().mockReturnValue(mockDb),
        getEntityById: vi.fn().mockReturnValue(null),
        redirectEntityEdges: vi.fn(),
        deleteEntity: vi.fn(),
      })

      mergeEntities(mockStore, 999, [2])
      expect(mockStore.redirectEntityEdges).not.toHaveBeenCalled()
      expect(mockStore.deleteEntity).not.toHaveBeenCalled()
    })
  })

  describe('runMergeCycle', () => {
    let mockStore: Store
    let config: MergeConfig
    let mockDb: any

    beforeEach(() => {
      mockDb = {
        transaction: (fn: () => void) => fn,
        prepare: vi.fn().mockReturnValue({ run: vi.fn() }),
      }
      mockStore = createMockStore({
        getDb: vi.fn().mockReturnValue(mockDb),
      })
      config = { ...DEFAULT_MERGE_CONFIG }
    })

    it('returns zeros when no groups found', () => {
      const result = runMergeCycle(mockStore, config)
      expect(result.merged).toBe(0)
      expect(result.groups).toBe(0)
    })

    it('merges all groups and returns stats', () => {
      mockStore = createMockStore({
        getDb: vi.fn().mockReturnValue(mockDb),
        getActiveEntitiesByTypeAndProject: vi.fn().mockReturnValue([
          createEntity(1, 'Redis', 'service', 'proj1'),
          createEntity(2, 'Redis server', 'service', 'proj1'),
          createEntity(3, 'MySQL', 'service', 'proj1'),
          createEntity(4, 'MySql', 'service', 'proj1'),
        ]),
        getEntityEdgeCount: vi.fn().mockReturnValue(0),
        getEntityById: vi.fn().mockImplementation((id: number) => {
          const entities: Record<number, MemoryEntity> = {
            1: createEntity(1, 'Redis', 'service', 'proj1'),
            2: createEntity(2, 'Redis server', 'service', 'proj1'),
            3: createEntity(3, 'MySQL', 'service', 'proj1'),
            4: createEntity(4, 'MySql', 'service', 'proj1'),
          }
          return entities[id] ?? null
        }),
        redirectEntityEdges: vi.fn(),
        deduplicateEdges: vi.fn(),
        deleteEntity: vi.fn(),
      })

      const result = runMergeCycle(mockStore, config)
      expect(result.groups).toBe(2)
      expect(result.merged).toBe(2)
    })

    it('passes projectHash to findSimilarEntities', () => {
      const projectHash = 'test-project'
      runMergeCycle(mockStore, config, projectHash)
      expect(mockStore.getActiveEntitiesByTypeAndProject).toHaveBeenCalledWith(projectHash)
    })
  })
})
