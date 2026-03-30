/**
 * FTS Client — Main-thread Promise-based RPC wrapper for the FTS Worker.
 * Sends search requests to the worker thread and returns results as Promises.
 */
import { Worker } from 'worker_threads';
import { randomUUID } from 'crypto';
import { log } from './logger.js';
import type { SearchResult, StoreSearchOptions } from './types.js';

interface PendingRequest {
  resolve: (value: SearchResult[]) => void;
  reject: (reason: Error) => void;
  timer: NodeJS.Timeout;
}

const TIMEOUT_MS = 30_000;
const pending = new Map<string, PendingRequest>();
let worker: Worker | null = null;
let ready = false;
let dbPathCached = '';

export function isFTSWorkerReady(): boolean {
  return ready && worker !== null;
}

export function initFTSWorker(dbPath: string): Promise<void> {
  dbPathCached = dbPath;
  return new Promise((resolve, reject) => {
    const workerPath = new URL('./fts-worker.ts', import.meta.url);

    // tsx registers itself as the loader for the main process, but worker threads
    // need their own registration. Try multiple approaches for compatibility:
    // 1. --import tsx/esm (tsx 4.x preferred)
    // 2. --import tsx (fallback)
    // 3. bare (if tsx auto-registers globally, e.g. via NODE_OPTIONS)
    const tsxImportPath = (() => {
      try {
        // Resolve tsx/esm relative to the project to get an absolute specifier
        const resolved = import.meta.resolve('tsx/esm');
        return resolved;
      } catch {
        return 'tsx/esm';
      }
    })();

    worker = new Worker(workerPath, {
      workerData: { dbPath },
      execArgv: ['--import', tsxImportPath],
    });

    const initTimeout = setTimeout(() => {
      reject(new Error('FTS worker failed to start within 10s'));
    }, 10_000);

    worker.on('message', (msg: { type?: string; id?: string; result?: SearchResult[]; error?: { message: string } }) => {
      if (msg.type === 'ready') {
        ready = true;
        clearTimeout(initTimeout);
        log('fts-client', 'FTS worker ready (dbPath=' + dbPath + ')');
        resolve();
        return;
      }

      if (msg.id) {
        const p = pending.get(msg.id);
        if (!p) return;
        clearTimeout(p.timer);
        pending.delete(msg.id);
        if (msg.error) {
          p.reject(new Error(msg.error.message));
        } else {
          p.resolve(msg.result ?? []);
        }
      }
    });

    worker.on('error', (err: Error) => {
      log('fts-client', 'FTS worker error: ' + err.message, 'error');
      clearTimeout(initTimeout);
      for (const [, p] of pending) {
        clearTimeout(p.timer);
        p.reject(err);
      }
      pending.clear();
      ready = false;
    });

    worker.on('exit', (code) => {
      log('fts-client', 'FTS worker exited code=' + code, code === 0 ? 'info' : 'error');
      ready = false;
      worker = null;
      for (const [, p] of pending) {
        clearTimeout(p.timer);
        p.reject(new Error('FTS worker exited with code ' + code));
      }
      pending.clear();
    });
  });
}

function sendRequest(method: 'searchFTS' | 'searchVec', args: Record<string, unknown>): Promise<SearchResult[]> {
  if (!worker || !ready) {
    return Promise.reject(new Error('FTS worker not initialized'));
  }

  const id = randomUUID();
  return new Promise<SearchResult[]>((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error('FTS ' + method + ' timed out after ' + TIMEOUT_MS + 'ms'));
    }, TIMEOUT_MS);

    pending.set(id, { resolve, reject, timer });
    worker!.postMessage({ id, method, args });
  });
}

export function searchFTSAsync(query: string, options: StoreSearchOptions = {}): Promise<SearchResult[]> {
  return sendRequest('searchFTS', { query, options });
}

export function searchVecWorker(query: string, embedding: number[], options: StoreSearchOptions = {}): Promise<SearchResult[]> {
  return sendRequest('searchVec', { query, embedding, options });
}

export async function shutdownFTSWorker(): Promise<void> {
  if (worker) {
    log('fts-client', 'Shutting down FTS worker');
    for (const [id, p] of pending) {
      clearTimeout(p.timer);
      p.reject(new Error('FTS worker shutting down'));
    }
    pending.clear();
    ready = false;
    await worker.terminate();
    worker = null;
  }
}
