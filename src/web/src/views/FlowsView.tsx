import { Fragment, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchFlows } from '../api/client';
import { useAppStore } from '../store/app';
import QueryStatus from '../components/QueryStatus';
import { SkeletonList } from '../components/Skeleton';

function FlowTypeBadge({ flowType }: { flowType: string }) {
  if (flowType === 'intra_community') {
    return <span className="rounded-full px-2 py-0.5 text-xs bg-blue-500/20 text-blue-400">intra</span>;
  }
  if (flowType === 'cross_community') {
    return <span className="rounded-full px-2 py-0.5 text-xs bg-orange-500/20 text-orange-400">cross</span>;
  }
  // doc_flow or any other type
  return (
    <span className="rounded-full px-2 py-0.5 text-xs bg-green-500/20 text-green-400">
      {flowType === 'doc_flow' ? 'doc' : flowType}
    </span>
  );
}

export default function FlowsView() {
  const workspace = useAppStore((state) => state.workspace);
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['flows', workspace],
    queryFn: () => fetchFlows(workspace),
  });
  const [selectedFlowId, setSelectedFlowId] = useState<number | null>(null);
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [search, setSearch] = useState('');

  const flows = data?.flows ?? [];
  const isDocFlow = (f: any) => f.id >= 100000;

  const filtered = flows.filter((f: any) => {
    if (typeFilter === 'doc_flow' && !isDocFlow(f)) return false;
    if (typeFilter === 'intra_community' && f.flowType !== 'intra_community') return false;
    if (typeFilter === 'cross_community' && f.flowType !== 'cross_community') return false;
    if (search && !f.label.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const selectedFlow = flows.find((f: any) => f.id === selectedFlowId) as any;

  const codeFlowCount = flows.filter((f: any) => !isDocFlow(f)).length;
  const docFlowCount = flows.filter((f: any) => isDocFlow(f)).length;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Execution Flows</h1>
        <p className="mt-1 text-sm text-[#8888a0]">
          Call chains from code analysis + architecture flows from docs.
          {!isLoading && flows.length > 0 && (
            <span className="ml-2">
              <span className="text-blue-400">{codeFlowCount} code</span>
              {' · '}
              <span className="text-green-400">{docFlowCount} doc</span>
            </span>
          )}
        </p>
      </div>

      <div className="flex items-center gap-4">
        <select
          className="panel px-3 py-2 text-sm text-[#e4e4ed]"
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
        >
          <option value="all">All types</option>
          <option value="intra_community">Intra-community</option>
          <option value="cross_community">Cross-community</option>
          <option value="doc_flow">Doc flows</option>
        </select>
        <input
          className="panel flex-1 px-3 py-2 text-sm text-[#e4e4ed] placeholder-[#8888a0]"
          placeholder="Search flows..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <span className="text-xs text-[#8888a0]">
          {isLoading ? 'Loading...' : `${filtered.length} of ${flows.length} flows`}
        </span>
      </div>

      {isError && <QueryStatus isLoading={false} isError={true} error={error} refetch={refetch} />}

      <div className="grid grid-cols-[1fr_1fr] gap-6 max-lg:grid-cols-1">
        <div className="space-y-2 overflow-auto" style={{ maxHeight: 'calc(100vh - 260px)' }}>
          {isLoading && <SkeletonList count={5} />}
          {filtered.length === 0 && !isLoading && !isError && (
            <div className="card p-4 text-sm text-[#8888a0]">
              {flows.length === 0
                ? "No flows found. Code flows are detected during codebase indexing — run 'npx nano-brain index-codebase' to populate."
                : 'No flows match the current filter.'}
            </div>
          )}
          {filtered.map((flow: any) => (
            <div
              key={flow.id}
              className={`card cursor-pointer px-4 py-3 transition hover:bg-[#1c1c27] ${
                selectedFlowId === flow.id ? 'border-indigo-500/50 bg-[#1c1c27]' : ''
              }`}
              onClick={() => setSelectedFlowId(flow.id)}
            >
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">{flow.label}</p>
                <FlowTypeBadge flowType={flow.flowType} />
              </div>
              {isDocFlow(flow) ? (
                <p className="mt-1 text-xs text-[#8888a0] truncate">
                  {flow.description ?? flow.services ?? flow.entryFile?.split('/').pop() ?? 'Architecture doc flow'}
                </p>
              ) : (
                <p className="mt-1 text-xs text-[#8888a0]">
                  {flow.stepCount} steps · {flow.entryFile?.split('/').pop() ?? '?'} → {flow.terminalFile?.split('/').pop() ?? '?'}
                </p>
              )}
            </div>
          ))}
        </div>

        <div>
          {selectedFlow ? (
            <div className="card p-4">
              <h2 className="text-lg font-semibold">{selectedFlow.label}</h2>
              <div className="mt-1 flex items-center gap-2">
                <FlowTypeBadge flowType={selectedFlow.flowType} />
                {!isDocFlow(selectedFlow) && (
                  <span className="text-xs text-[#8888a0]">{selectedFlow.stepCount} steps</span>
                )}
              </div>

              {isDocFlow(selectedFlow) ? (
                <div className="mt-4 space-y-3">
                  {selectedFlow.description && (
                    <div>
                      <p className="text-xs font-medium text-[#8888a0] uppercase tracking-wider mb-1">Description</p>
                      <p className="text-sm">{selectedFlow.description}</p>
                    </div>
                  )}
                  {selectedFlow.services && (
                    <div>
                      <p className="text-xs font-medium text-[#8888a0] uppercase tracking-wider mb-1">Services</p>
                      <div className="flex flex-wrap gap-1">
                        {selectedFlow.services.split('\n').filter(Boolean).map((svc: string, i: number) => (
                          <span key={i} className="rounded bg-[#1c1c27] border border-[#232331] px-2 py-0.5 text-xs text-green-300">
                            {svc.replace(/^[-\s*]+/, '').trim()}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {selectedFlow.lastUpdated && (
                    <div className="border-t border-[#1f1f2c] pt-3 text-xs text-[#8888a0]">
                      <p>Last updated: {selectedFlow.lastUpdated}</p>
                      {selectedFlow.entryFile && <p>Source: {selectedFlow.entryFile.split('/').pop()}</p>}
                    </div>
                  )}
                </div>
              ) : (
                <>
                  <div className="mt-4 flex flex-wrap items-center gap-2">
                    {selectedFlow.steps.map((step: any, i: number) => (
                      <Fragment key={step.stepIndex}>
                        {i > 0 && <span className="text-lg text-[#8888a0]">→</span>}
                        <div className="shrink-0 rounded-lg border border-[#232331] bg-[#1c1c27] px-3 py-2">
                          <p className="text-sm font-medium">{step.name}</p>
                          <p className="text-xs text-[#8888a0]">
                            {step.kind} · {step.filePath?.split('/').pop() ?? '?'}:{step.startLine}
                          </p>
                        </div>
                      </Fragment>
                    ))}
                  </div>
                  <div className="mt-4 border-t border-[#1f1f2c] pt-3 text-xs text-[#8888a0]">
                    <p>Entry: {selectedFlow.entryName ?? '?'} ({selectedFlow.entryFile ?? '?'})</p>
                    <p>Terminal: {selectedFlow.terminalName ?? '?'} ({selectedFlow.terminalFile ?? '?'})</p>
                  </div>
                </>
              )}
            </div>
          ) : (
            <div className="card p-4 text-sm text-[#8888a0]">Select a flow to see details.</div>
          )}
        </div>
      </div>
    </div>
  );
}
