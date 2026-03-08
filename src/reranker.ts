import type { RerankResult, RerankDocument } from './types.js';
import { log } from './logger.js';

export interface Reranker {
  rerank(query: string, documents: RerankDocument[]): Promise<RerankResult>;
  dispose(): void;
}

export interface RerankerOptions {
  modelPath?: string;
  cacheDir?: string;
}

export async function createReranker(
  _options?: RerankerOptions
): Promise<Reranker | null> {
  log('reranker', 'local reranker removed — use external reranker or rely on BM25+vector fusion');
  return null;
}
