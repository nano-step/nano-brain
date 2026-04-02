import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { fetchSearch } from '../api/client';
import { useAppStore } from '../store/app';
import SearchResult from '../components/SearchResult';
import QueryStatus from '../components/QueryStatus';

export default function Search() {
  const workspace = useAppStore((state) => state.workspace);
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState(() => searchParams.get('q') ?? '');
  const [debounced, setDebounced] = useState(() => searchParams.get('q') ?? '');
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebounced(query);
      if (query.trim().length > 1) {
        setSearchParams({ q: query }, { replace: true });
      } else {
        setSearchParams({}, { replace: true });
      }
    }, 300);
    return () => window.clearTimeout(handle);
  }, [query, setSearchParams]);

  const { data, isFetching, isError, error, refetch } = useQuery({
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
        <label htmlFor="search-query" className="text-xs uppercase text-[#8888a0]">Search query</label>
        <input
          id="search-query"
          name="search-query"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search docs, entities, code..."
          className="mt-2 w-full rounded-xl border border-[#26263a] bg-[#0f0f16] px-3 py-2 text-sm text-[#e4e4ed]"
        />
        <div className="mt-3 flex items-center justify-between text-xs text-[#8888a0]">
          <span className="flex items-center gap-2">
            {isFetching ? 'Searching...' : data ? `${data.results.length} results` : 'Idle'}
            {data?.fallback === 'fts' && (
              <span className="rounded-full bg-orange-500/20 px-2 py-0.5 text-[11px] text-orange-400">FTS fallback</span>
            )}
          </span>
          <span>{data ? `${data.executionMs} ms` : '—'}</span>
        </div>
      </div>

      {isError && <QueryStatus isLoading={false} isError={true} error={error} refetch={refetch} />}

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
