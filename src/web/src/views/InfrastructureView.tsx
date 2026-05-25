import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchInfrastructure } from '../api/client';
import { infraTypeColorMap } from '../lib/colors';
import { useAppStore } from '../store/app';
import QueryStatus from '../components/QueryStatus';
import { SkeletonList } from '../components/Skeleton';

export default function InfrastructureView() {
  const workspace = useAppStore((state) => state.workspace);
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['infrastructure', workspace],
    queryFn: () => fetchInfrastructure(workspace),
  });
  const [expandedTypes, setExpandedTypes] = useState<Record<string, boolean>>({});
  const [expandedPatterns, setExpandedPatterns] = useState<Record<string, boolean>>({});
  const [typeFilter, setTypeFilter] = useState('all');
  const [repoFilter, setRepoFilter] = useState('');
  const [operationFilter, setOperationFilter] = useState('all');

  const grouped = data?.grouped ?? {};
  const types = Object.keys(grouped);

  // Derive repos and operations from grouped data (server no longer sends raw symbols array)
  const repoOptions = useMemo(() => {
    const repos = new Set<string>();
    for (const patterns of Object.values(grouped)) {
      for (const pattern of patterns) {
        for (const op of pattern.operations) {
          repos.add(op.repo);
        }
      }
    }
    return Array.from(repos.values()).sort();
  }, [grouped]);

  const operationOptions = useMemo(() => {
    const ops = new Set<string>();
    for (const patterns of Object.values(grouped)) {
      for (const pattern of patterns) {
        for (const op of pattern.operations) {
          ops.add(op.op);
        }
      }
    }
    return Array.from(ops.values()).sort();
  }, [grouped]);

  const filteredGroups = useMemo(() => {
    const repoNeedle = repoFilter.trim().toLowerCase();
    return Object.entries(grouped)
      .filter(([type]) => typeFilter === 'all' || type === typeFilter)
      .map(([type, patterns]) => {
        const filteredPatterns = patterns.filter((pattern) => {
          const matchesRepo = !repoNeedle
            || pattern.operations.some((op) => op.repo.toLowerCase().includes(repoNeedle));
          const matchesOp = operationFilter === 'all'
            || pattern.operations.some((op) => op.op === operationFilter);
          return matchesRepo && matchesOp;
        });
        return [type, filteredPatterns] as const;
      })
      .filter(([, patterns]) => patterns.length > 0);
  }, [grouped, typeFilter, repoFilter, operationFilter]);

  const toggleType = (type: string) => {
    setExpandedTypes((prev) => ({ ...prev, [type]: !prev[type] }));
  };

  const togglePattern = (key: string) => {
    setExpandedPatterns((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold">Infrastructure Symbols</h1>
        <p className="mt-1 text-sm text-[#8888a0]">Cross-repo Redis, MySQL, API, and queue dependencies.</p>
        <div className="mt-4 grid gap-3 lg:grid-cols-3">
          <label className="text-xs uppercase text-[#8888a0]">
            Type
            <select
              className="panel mt-2 w-full px-3 py-2 text-sm text-[#e4e4ed]"
              value={typeFilter}
              onChange={(event) => setTypeFilter(event.target.value)}
            >
              <option value="all">All types</option>
              {types.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
          <label className="text-xs uppercase text-[#8888a0]">
            Repository
            <input
              value={repoFilter}
              onChange={(event) => setRepoFilter(event.target.value)}
              placeholder="Filter by repo..."
              list="repo-options"
              className="mt-2 w-full rounded-xl border border-[#26263a] bg-[#0f0f16] px-3 py-2 text-sm text-[#e4e4ed]"
            />
            <datalist id="repo-options">
              {repoOptions.map((repo) => (
                <option key={repo} value={repo} />
              ))}
            </datalist>
          </label>
          <label className="text-xs uppercase text-[#8888a0]">
            Operation
            <select
              className="panel mt-2 w-full px-3 py-2 text-sm text-[#e4e4ed]"
              value={operationFilter}
              onChange={(event) => setOperationFilter(event.target.value)}
            >
              <option value="all">All operations</option>
              {operationOptions.map((op) => (
                <option key={op} value={op}>
                  {op}
                </option>
              ))}
            </select>
          </label>
        </div>
        <div className="mt-3 text-xs text-[#8888a0]">
          {isLoading ? 'Loading symbols...' : `${filteredGroups.reduce((sum, [, patterns]) => sum + patterns.length, 0)} patterns`}
        </div>
      </header>

      {isError && <QueryStatus isLoading={false} isError={true} error={error} refetch={refetch} />}
      {isLoading && <SkeletonList count={4} />}
      {filteredGroups.length === 0 && !isLoading && !isError ? (
        <div className="card p-6 text-sm text-[#8888a0]">No infrastructure symbols found.</div>
      ) : (
        <div className="space-y-6">
          {filteredGroups.map(([type, patterns]) => {
            const expanded = expandedTypes[type] ?? true;
            return (
              <div key={type} className="card overflow-hidden">
                <div
                  className="flex cursor-pointer items-center gap-3 p-4"
                  onClick={() => toggleType(type)}
                >
                  <span className="h-3 w-3 rounded-full" style={{ background: infraTypeColorMap[type] ?? '#64748b' }} />
                  <h2 className="text-sm font-semibold uppercase tracking-[0.2em]">{type}</h2>
                  <span className="text-xs text-[#8888a0]">{patterns.length} patterns</span>
                  <span className="ml-auto text-xs text-[#646478]">{expanded ? 'Collapse' : 'Expand'}</span>
                </div>
                {expanded && (
                  <div className="border-t border-[#232331]">
                    {patterns.map((pattern) => {
                      const key = `${type}-${pattern.pattern}`;
                      const ops = Array.from(new Set(pattern.operations.map((op) => op.op)));
                      const expandedPattern = expandedPatterns[key] ?? false;
                      return (
                        <div key={key} className="border-b border-[#1f1f2c] px-4 py-3">
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <button
                              type="button"
                              className="flex items-center gap-2 text-left text-sm text-[#e4e4ed]"
                              onClick={() => togglePattern(key)}
                            >
                              <span className="text-xs text-[#646478]">{expandedPattern ? 'v' : '>'}</span>
                              <code className="text-sm text-[#e4e4ed]">{pattern.pattern}</code>
                            </button>
                            <div className="flex flex-wrap gap-2">
                              {ops.map((op) => (
                                <span key={op} className="rounded-full bg-[#1c1c27] px-2 py-0.5 text-xs">
                                  {op}
                                </span>
                              ))}
                              <span className="rounded-full bg-[#151520] px-2 py-0.5 text-xs text-[#8888a0]">
                                {new Set(pattern.operations.map((op) => op.repo)).size} repos
                              </span>
                            </div>
                          </div>
                          {expandedPattern && (
                            <div className="mt-3 space-y-1 text-xs text-[#8888a0]">
                              {pattern.operations.map((op, index) => (
                                <div key={`${key}-${op.repo}-${index}`}>
                                  [{op.op}] {op.repo}: {op.file}:{op.line}
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
