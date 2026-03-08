export interface QueryExpander {
  expand(query: string): Promise<string[]>;
  dispose(): void;
}

export interface QueryExpanderOptions {
  modelPath?: string;
  cacheDir?: string;
}

export async function createQueryExpander(
  _options?: QueryExpanderOptions
): Promise<QueryExpander | null> {
  return null;
}
