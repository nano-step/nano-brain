import type { Store, MemoryEntity } from './types.js'
import { log } from './logger.js'

export interface MergeConfig {
  enabled: boolean
  interval_ms: number
  similarity_threshold: number
  batch_size: number
}

export const DEFAULT_MERGE_CONFIG: MergeConfig = {
  enabled: true,
  interval_ms: 86400000,
  similarity_threshold: 0.8,
  batch_size: 50,
}

export function parseMergeConfig(raw?: Partial<MergeConfig>): MergeConfig {
  return {
    enabled: raw?.enabled ?? DEFAULT_MERGE_CONFIG.enabled,
    interval_ms: raw?.interval_ms ?? DEFAULT_MERGE_CONFIG.interval_ms,
    similarity_threshold: raw?.similarity_threshold ?? DEFAULT_MERGE_CONFIG.similarity_threshold,
    batch_size: raw?.batch_size ?? DEFAULT_MERGE_CONFIG.batch_size,
  }
}

export interface MergeGroup {
  canonicalId: number
  duplicateIds: number[]
  canonicalName: string
}

export interface MergeResult {
  merged: number
  groups: number
}

export function levenshteinDistance(a: string, b: string): number {
  const aLower = a.toLowerCase()
  const bLower = b.toLowerCase()
  
  if (aLower === bLower) return 0
  if (aLower.length === 0) return bLower.length
  if (bLower.length === 0) return aLower.length
  
  const matrix: number[][] = []
  
  for (let i = 0; i <= aLower.length; i++) {
    matrix[i] = [i]
  }
  for (let j = 0; j <= bLower.length; j++) {
    matrix[0][j] = j
  }
  
  for (let i = 1; i <= aLower.length; i++) {
    for (let j = 1; j <= bLower.length; j++) {
      const cost = aLower[i - 1] === bLower[j - 1] ? 0 : 1
      matrix[i][j] = Math.min(
        matrix[i - 1][j] + 1,
        matrix[i][j - 1] + 1,
        matrix[i - 1][j - 1] + cost
      )
    }
  }
  
  return matrix[aLower.length][bLower.length]
}

export function isPrefixMatch(shorter: string, longer: string): boolean {
  const shortLower = shorter.toLowerCase().trim()
  const longLower = longer.toLowerCase().trim()
  
  if (shortLower.length >= longLower.length) return false
  
  return longLower.startsWith(shortLower + ' ') || longLower.startsWith(shortLower + '-')
}

export function areSimilar(name1: string, name2: string, threshold: number): boolean {
  const n1 = name1.toLowerCase().trim()
  const n2 = name2.toLowerCase().trim()
  
  if (n1 === n2) return true
  
  const shorter = n1.length <= n2.length ? n1 : n2
  const longer = n1.length <= n2.length ? n2 : n1
  if (isPrefixMatch(shorter, longer)) return true
  
  const lengthDiff = Math.abs(n1.length - n2.length)
  if (lengthDiff > 3) return false
  
  const distance = levenshteinDistance(n1, n2)
  if (distance <= 2) return true
  
  const maxLen = Math.max(n1.length, n2.length)
  const similarity = 1 - (distance / maxLen)
  return similarity >= threshold
}

function selectCanonical(entities: MemoryEntity[], store: Store): MemoryEntity {
  let best = entities[0]
  let bestScore = 0
  
  for (const entity of entities) {
    let score = 0
    
    score -= entity.name.length * 10
    
    const edgeCount = store.getEntityEdgeCount(entity.id)
    score += edgeCount * 100
    
    const firstLearned = new Date(entity.firstLearnedAt).getTime()
    score -= firstLearned / 1000000000
    
    if (score > bestScore || bestScore === 0) {
      bestScore = score
      best = entity
    }
  }
  
  return best
}

export function findSimilarEntities(
  store: Store,
  config: MergeConfig,
  projectHash?: string
): MergeGroup[] {
  const entities = store.getActiveEntitiesByTypeAndProject(projectHash)
  
  const byTypeAndProject = new Map<string, MemoryEntity[]>()
  for (const entity of entities) {
    const key = `${entity.type}:${entity.projectHash}`
    const group = byTypeAndProject.get(key) ?? []
    group.push(entity)
    byTypeAndProject.set(key, group)
  }
  
  const mergeGroups: MergeGroup[] = []
  const processed = new Set<number>()
  
  for (const [, groupEntities] of byTypeAndProject) {
    for (let i = 0; i < groupEntities.length; i++) {
      const entity = groupEntities[i]
      if (processed.has(entity.id)) continue
      
      const similar: MemoryEntity[] = [entity]
      
      for (let j = i + 1; j < groupEntities.length; j++) {
        const other = groupEntities[j]
        if (processed.has(other.id)) continue
        
        if (areSimilar(entity.name, other.name, config.similarity_threshold)) {
          similar.push(other)
          processed.add(other.id)
        }
      }
      
      if (similar.length > 1) {
        const canonical = selectCanonical(similar, store)
        const duplicateIds = similar
          .filter(e => e.id !== canonical.id)
          .map(e => e.id)
        
        mergeGroups.push({
          canonicalId: canonical.id,
          duplicateIds,
          canonicalName: canonical.name,
        })
        
        processed.add(canonical.id)
        
        if (mergeGroups.length >= config.batch_size) {
          return mergeGroups
        }
      }
    }
  }
  
  return mergeGroups
}

export function mergeEntities(
  store: Store,
  canonicalId: number,
  duplicateIds: number[]
): void {
  const db = store.getDb()
  
  const transaction = db.transaction(() => {
    const canonical = store.getEntityById(canonicalId)
    if (!canonical) {
      log('entity-merger', `Canonical entity ${canonicalId} not found, skipping merge`)
      return
    }
    
    let maxLastConfirmed = new Date(canonical.lastConfirmedAt).getTime()
    const descriptions: string[] = []
    if (canonical.description) {
      descriptions.push(canonical.description)
    }
    
    for (const duplicateId of duplicateIds) {
      const duplicate = store.getEntityById(duplicateId)
      if (!duplicate) continue
      
      const dupLastConfirmed = new Date(duplicate.lastConfirmedAt).getTime()
      if (dupLastConfirmed > maxLastConfirmed) {
        maxLastConfirmed = dupLastConfirmed
      }
      
      if (duplicate.description && !descriptions.includes(duplicate.description)) {
        descriptions.push(duplicate.description)
      }
      
      store.redirectEntityEdges(duplicateId, canonicalId)
      
      store.deduplicateEdges(canonicalId)
      
      store.deleteEntity(duplicateId)
    }
    
    if (maxLastConfirmed > new Date(canonical.lastConfirmedAt).getTime()) {
      db.prepare(`
        UPDATE memory_entities SET last_confirmed_at = datetime(?, 'unixepoch')
        WHERE id = ?
      `).run(Math.floor(maxLastConfirmed / 1000), canonicalId)
    }
    
    if (descriptions.length > 1) {
      const mergedDescription = descriptions.join(' | ')
      db.prepare(`
        UPDATE memory_entities SET description = ?
        WHERE id = ?
      `).run(mergedDescription.substring(0, 1000), canonicalId)
    }
  })
  
  transaction()
}

export function runMergeCycle(
  store: Store,
  config: MergeConfig,
  projectHash?: string
): MergeResult {
  const groups = findSimilarEntities(store, config, projectHash)
  
  if (groups.length === 0) {
    return { merged: 0, groups: 0 }
  }
  
  let totalMerged = 0
  
  for (const group of groups) {
    try {
      mergeEntities(store, group.canonicalId, group.duplicateIds)
      totalMerged += group.duplicateIds.length
      log('entity-merger', `Merged ${group.duplicateIds.length} entities into "${group.canonicalName}" (id=${group.canonicalId})`)
    } catch (err) {
      log('entity-merger', `Failed to merge group for "${group.canonicalName}": ${err instanceof Error ? err.message : String(err)}`, 'warn')
    }
  }
  
  log('entity-merger', `Merge cycle complete: ${groups.length} groups, ${totalMerged} entities merged`)
  
  return { merged: totalMerged, groups: groups.length }
}
