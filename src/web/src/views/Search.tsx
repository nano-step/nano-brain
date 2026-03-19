import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchSearch } from '../api/client';
import { useAppStore } from '../store/app';
import SearchResult from '../components/SearchResult';

export default function Search() {
  const workspace = useAppStore((state) => state.workspace);
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    const handle = window.setTimeout(() => setDebounced(query), 300);
    return () => window.clearTimeout(handle);
  }, [query]);

  const { data, isFetching } = useQuery({
    queryKey: ['search', debounced, workspace],
    queryFn: () => fetchSearch(debounced, 20, workspace),
    enabled: debounced.trim().length > 1,
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Hybrid Search</h1>
        <p className="mt-1 text-sm text-[#8888a0]">BM25 + vector + knowledge graph reranking.</p>
      </div>

      <div className="card p-4">
        <label className="text-xs uppercase text-[#8888a0]">Search query</label>
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search docs, entities, code..."
          className="mt-2 w-full rounded-xl border border-[#26263a] bg-[#0f0f16] px-3 py-2 text-sm text-[#e4e4ed]"
        />
        <div className="mt-3 flex items-center justify-between text-xs text-[#8888a0]">
          <span>{isFetching ? 'Searching...' : data ? `${data.results.length} results` : 'Idle'}</span>
          <span>{data ? `${data.executionMs} ms` : '—'}</span>
        </div>
      </div>

      <div className="space-y-4">
        {data?.results.map((result) => (
          <SearchResult
            key={result.id}
            result={result}
            expanded={expanded === result.id}
            onToggle={() => setExpanded(expanded === result.id ? null : result.id)}
          />
        ))}
        {data && data.results.length === 0 && (
          <div className="card p-6 text-sm text-[#8888a0]">No results found.</div>
        )}
        {debounced.trim().length <= 1 && (
          <div className="card p-6 text-sm text-[#8888a0]">Type at least 2 characters to search.</div>
        )}
      </div>
    </div>
  );
}
