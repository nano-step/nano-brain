import { useQuery } from '@tanstack/react-query';
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { fetchStatus, fetchTelemetry } from '../api/client';
import { useAppStore } from '../store/app';
import QueryStatus from '../components/QueryStatus';
import { SkeletonCard } from '../components/Skeleton';

export default function Dashboard() {
  const workspace = useAppStore((state) => state.workspace);

  const { data: status, isLoading: statusLoading, isError: statusError, error: statusErr, refetch: refetchStatus } = useQuery({ queryKey: ['status'], queryFn: fetchStatus });
  const { data: telemetry, isLoading: telemetryLoading, isError: telemetryError, error: telemetryErr, refetch: refetchTelemetry } = useQuery({
    queryKey: ['telemetry', workspace],
    queryFn: () => fetchTelemetry(workspace),
  });

  const banditData = telemetry?.banditStats && !Array.isArray(telemetry.banditStats)
    ? Object.entries(telemetry.banditStats).map(([variant, stats]) => ({
        variant,
        success: (stats as { success: number; failure: number }).success,
        failure: (stats as { success: number; failure: number }).failure,
      }))
    : [];

  const weightData = telemetry?.preferenceWeights
    ? Object.entries(telemetry.preferenceWeights).map(([category, weight]) => ({
        category,
        weight,
      }))
    : [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">System Dashboard</h1>
        <p className="mt-1 text-sm text-[#8888a0]">Live view of nano-brain health, indexing, and learning telemetry.</p>
      </div>

      {statusError && (
        <QueryStatus isLoading={false} isError={true} error={statusErr} refetch={refetchStatus} />
      )}

      <div className="grid-cards">
        {statusLoading ? (
          <>
            <SkeletonCard />
            <SkeletonCard />
            <SkeletonCard />
            <SkeletonCard />
          </>
        ) : (
          <>
            <div className="card p-4">
              <p className="text-xs uppercase text-[#8888a0]">Version</p>
              <p className="mt-2 text-2xl font-semibold">{status?.version ?? '—'}</p>
            </div>
            <div className="card p-4">
              <p className="text-xs uppercase text-[#8888a0]">Uptime</p>
              <p className="mt-2 text-2xl font-semibold">
                {status ? `${Math.floor(status.uptime / 60)}m` : '—'}
              </p>
            </div>
            <div className="card p-4">
              <p className="text-xs uppercase text-[#8888a0]">Documents</p>
              <p className="mt-2 text-2xl font-semibold">{status?.documents ?? '—'}</p>
            </div>
            <div className="card p-4">
              <p className="text-xs uppercase text-[#8888a0]">Embeddings</p>
              <p className="mt-2 text-2xl font-semibold">{status?.embeddings ?? '—'}</p>
            </div>
          </>
        )}
      </div>

      {telemetryError && (
        <QueryStatus isLoading={false} isError={true} error={telemetryErr} refetch={refetchTelemetry} />
      )}

      <div className="grid-dense">
        <div className="card col-span-7 p-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold">Bandit Stats</h2>
              <p className="text-xs text-[#8888a0]">Success vs failure per variant.</p>
            </div>
            <span className="text-xs text-[#8888a0]">{telemetryLoading ? 'loading' : `${banditData.length} variants`}</span>
          </div>
          <div className="mt-4 h-64">
            {banditData.length === 0 && !telemetryLoading ? (
              <div className="flex h-full items-center justify-center text-xs text-[#8888a0]">
                No bandit variants recorded yet — bandit stats populate after hybrid searches are run.
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={banditData} margin={{ top: 10, right: 10, left: -10, bottom: 10 }}>
                  <XAxis dataKey="variant" stroke="#8888a0" fontSize={12} />
                  <YAxis stroke="#8888a0" fontSize={12} />
                  <Tooltip contentStyle={{ background: '#111118', border: '1px solid #1f1f2c' }} />
                  <Bar dataKey="success" stackId="a" fill="#3b82f6" radius={[6, 6, 0, 0]} />
                  <Bar dataKey="failure" stackId="a" fill="#f97316" radius={[6, 6, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>

        <div className="card col-span-5 p-4">
          <h2 className="text-lg font-semibold">Expand Rate</h2>
          <p className="text-xs text-[#8888a0]">Queries that expanded into memory graph.</p>
          <div className="mt-6 flex items-end gap-3">
            <p className="text-5xl font-semibold">
              {telemetry ? `${Math.round(telemetry.expandRate * 100)}%` : '—'}
            </p>
          </div>
          <div className="mt-6">
            <p className="text-xs uppercase text-[#8888a0]">Query volume</p>
            <p className="mt-1 text-xl font-semibold">{telemetry?.queryCount ?? '—'}</p>
          </div>
          <div className="mt-4 grid grid-cols-2 gap-3 text-xs text-[#8888a0]">
            <div>
              <p>Importance mean</p>
              <p className="text-sm text-[#e4e4ed]">{telemetry?.importanceStats.mean?.toFixed(2) ?? '—'}</p>
            </div>
            <div>
              <p>Importance median</p>
              <p className="text-sm text-[#e4e4ed]">{telemetry?.importanceStats.median?.toFixed(2) ?? '—'}</p>
            </div>
          </div>
        </div>
      </div>

      <div className="card p-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Preference Weights</h2>
            <p className="text-xs text-[#8888a0]">Current category weighting in ranking model.</p>
          </div>
          <span className="text-xs text-[#8888a0]">{weightData.length} categories</span>
        </div>
        <div className="mt-4 h-64">
          {weightData.length === 0 && !telemetryLoading ? (
            <div className="flex h-full items-center justify-center text-xs text-[#8888a0]">
              No preference weights recorded yet — weights build up as search queries are made and feedback is collected.
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={weightData} layout="vertical" margin={{ top: 10, right: 10, left: 60, bottom: 10 }}>
                <XAxis type="number" stroke="#8888a0" fontSize={12} />
                <YAxis dataKey="category" type="category" stroke="#8888a0" fontSize={12} width={120} />
                <Tooltip contentStyle={{ background: '#111118', border: '1px solid #1f1f2c' }} />
                <Bar dataKey="weight" fill="#8b5cf6" radius={[0, 6, 6, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>
    </div>
  );
}
