/**
 * FTS Worker Thread
 * Handles Full-Text Search queries in a dedicated thread to prevent blocking the main event loop.
 * Opens a read-only SQLite connection and processes searchFTS/searchVec requests via message passing.
 */
import { parentPort, workerData } from 'worker_threads';
import Database from 'better-sqlite3';
import type { SearchResult, StoreSearchOptions } from './types.js';

// Message types
interface WorkerRequest {
  id: string;
  method: 'searchFTS' | 'searchVec';
  args: {
    query: string;
    options: StoreSearchOptions;
    embedding?: number[];
  };
}

interface WorkerResponse {
  id: string;
  result?: SearchResult[];
  error?: { message: string };
}

interface ReadyMessage {
  type: 'ready';
}

// Replicate sanitizeFTS5Query from store.ts (line 70-78)
function sanitizeFTS5Query(query: string): string {
  const trimmed = query.trim();
  if (!trimmed) return '';
  const tokens = trimmed.split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return '';
  const quotedTokens = tokens.map((token) => `"${token.replace(/"/g, '""')}"`);
  if (quotedTokens.length === 1) return quotedTokens[0];
  return quotedTokens.join(' OR ');
}

// Open read-only connection
const dbPath = workerData.dbPath as string;
const db = new Database(dbPath, { readonly: true });
db.pragma('journal_mode = WAL');

// Try to load sqlite-vec extension for vector search
let vecAvailable = false;
try {
  const sqliteVec = await import('sqlite-vec');
  sqliteVec.load(db);
  vecAvailable = true;
} catch {
  // sqlite-vec not available, vector search will return empty results
}

/**
 * searchFTS - Replicates store.ts lines 1526-1595
 */
function searchFTS(query: string, options: StoreSearchOptions = {}): SearchResult[] {
  const { limit = 10, collection, projectHash, tags, since, until } = options;
  const sanitized = sanitizeFTS5Query(query);
  if (!sanitized) return [];

  let sql = `
    SELECT 
      d.id, d.path, d.collection, d.title, d.hash, d.agent, d.project_hash,
      d.centrality, d.cluster_id, d.superseded_by,
      d.access_count, d.last_accessed_at as lastAccessedAt,
      snippet(documents_fts, 2, '<mark>', '</mark>', '...', 64) as snippet,
      bm25(documents_fts) as score
    FROM documents_fts f
    JOIN documents d ON f.filepath = d.collection || '/' || d.path
    WHERE documents_fts MATCH ? AND d.active = 1
  `;
  const params: (string | number)[] = [sanitized];

  if (collection) {
    sql += ` AND d.collection = ?`;
    params.push(collection);
  }
  if (projectHash && projectHash !== 'all') {
    sql += ` AND d.project_hash IN (?, 'global')`;
    params.push(projectHash);
  }
  if (since) {
    sql += ` AND d.modified_at >= ?`;
    params.push(since);
  }
  if (until) {
    sql += ` AND d.modified_at <= ?`;
    params.push(until);
  }
  if (tags && tags.length > 0) {
    sql += ` AND d.id IN (
      SELECT dt.document_id FROM document_tags dt
      WHERE dt.tag IN (${tags.map(() => '?').join(',')})
      GROUP BY dt.document_id
      HAVING COUNT(DISTINCT dt.tag) = ?
    )`;
    params.push(...tags.map(t => t.toLowerCase().trim()));
    params.push(tags.length);
  }

  sql += ` ORDER BY bm25(documents_fts) LIMIT ?`;
  params.push(limit);

  const rows = db.prepare(sql).all(...params) as Array<Record<string, unknown>>;

  return rows.map(row => ({
    id: String(row.id),
    path: row.path as string,
    collection: row.collection as string,
    title: row.title as string,
    snippet: row.snippet as string,
    score: Math.abs(row.score as number),
    startLine: 0,
    endLine: 0,
    docid: (row.hash as string).substring(0, 6),
    agent: row.agent as string | undefined,
    projectHash: projectHash === 'all' ? (row.project_hash as string | undefined) : undefined,
    centrality: row.centrality as number | undefined,
    clusterId: row.cluster_id as number | undefined,
    supersededBy: row.superseded_by as number | null | undefined,
    access_count: row.access_count as number | undefined,
    lastAccessedAt: row.lastAccessedAt as string | null | undefined,
  }));
}

/**
 * searchVec - Replicates store.ts lines 1597-1672
 */
function searchVec(query: string, embedding: number[], options: StoreSearchOptions = {}): SearchResult[] {
  const { limit = 10, collection, projectHash, tags, since, until } = options;
  if (!vecAvailable) {
    return [];
  }

  try {
    let sql = `
      SELECT v.hash_seq, v.distance, d.id, d.path, d.collection, d.title, d.hash, d.agent, d.project_hash,
             d.centrality, d.cluster_id, d.superseded_by,
             d.access_count, d.last_accessed_at as lastAccessedAt,
             substr(c.body, 1, 700) as snippet
      FROM vectors_vec v
      JOIN documents d ON substr(v.hash_seq, 1, instr(v.hash_seq, ':') - 1) = d.hash
      LEFT JOIN content c ON c.hash = d.hash
      WHERE v.embedding MATCH ?
        AND k = ?
        AND d.active = 1
    `;

    const params: (Float32Array | string | number)[] = [new Float32Array(embedding), limit];
    if (collection) {
      sql += ` AND d.collection = ?`;
      params.push(collection);
    }
    if (projectHash && projectHash !== 'all') {
      sql += ` AND d.project_hash IN (?, 'global')`;
      params.push(projectHash);
    }
    if (since) {
      sql += ` AND d.modified_at >= ?`;
      params.push(since);
    }
    if (until) {
      sql += ` AND d.modified_at <= ?`;
      params.push(until);
    }
    if (tags && tags.length > 0) {
      sql += ` AND d.id IN (
        SELECT dt.document_id FROM document_tags dt
        WHERE dt.tag IN (${tags.map(() => '?').join(',')})
        GROUP BY dt.document_id
        HAVING COUNT(DISTINCT dt.tag) = ?
      )`;
      params.push(...tags.map(t => t.toLowerCase().trim()));
      params.push(tags.length);
    }
    sql += ` ORDER BY v.distance`;

    const stmt = db.prepare(sql);
    const rows = stmt.all(...params) as Array<Record<string, unknown>>;

    return rows.map(row => ({
      id: String(row.id),
      path: row.path as string,
      collection: row.collection as string,
      title: row.title as string,
      snippet: (row.snippet as string) || '',
      score: 1 - (row.distance as number),
      startLine: 0,
      endLine: 0,
      docid: (row.hash as string).substring(0, 6),
      agent: row.agent as string | undefined,
      projectHash: projectHash === 'all' ? (row.project_hash as string | undefined) : undefined,
      centrality: row.centrality as number | undefined,
      clusterId: row.cluster_id as number | undefined,
      supersededBy: row.superseded_by as number | null | undefined,
      access_count: row.access_count as number | undefined,
      lastAccessedAt: row.lastAccessedAt as string | null | undefined,
    }));
  } catch {
    return [];
  }
}

// Handle messages from main thread
if (parentPort) {
  parentPort.on('message', (msg: WorkerRequest) => {
    const response: WorkerResponse = { id: msg.id };

    try {
      if (msg.method === 'searchFTS') {
        response.result = searchFTS(msg.args.query, msg.args.options);
      } else if (msg.method === 'searchVec') {
        if (!msg.args.embedding) {
          throw new Error('embedding is required for searchVec');
        }
        response.result = searchVec(msg.args.query, msg.args.embedding, msg.args.options);
      } else {
        throw new Error(`Unknown method: ${msg.method}`);
      }
    } catch (err) {
      response.error = { message: err instanceof Error ? err.message : String(err) };
    }

    parentPort!.postMessage(response);
  });

  // Signal that worker is ready
  const readyMsg: ReadyMessage = { type: 'ready' };
  parentPort.postMessage(readyMsg);
}
