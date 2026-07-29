import { AlertCircle, Database, FolderOpen, HardDrive, Server, Zap } from 'lucide-react';
import { Link } from 'react-router';
import { PageHeader } from '@/components/ui/page-header';
import { IconTile } from '@/components/ui/icon-tile';
import { EmptyState } from '@/components/ui/empty-state';
import { BucketUsageChart } from '@/components/charts/BucketUsageChart';
import { useDashboardData } from '@/hooks/useApi';
import { formatBytes } from '@/lib/file-utils';
import type { ClusterHealth } from '@/types';

type StatTone = 'primary' | 'destructive' | 'neutral';
type HealthLabel = 'Healthy' | 'Degraded' | 'Unhealthy' | 'Unknown';

function deriveHealth(health: ClusterHealth | null): { label: HealthLabel; tone: StatTone } {
  if (!health) return { label: 'Unknown', tone: 'neutral' };
  if (
    health.storageNodesUp === health.storageNodes &&
    health.partitionsAllOk === health.partitions &&
    health.connectedNodes === health.knownNodes
  ) return { label: 'Healthy', tone: 'primary' };
  if (health.storageNodesUp > 0 && health.partitionsQuorum > 0) return { label: 'Degraded', tone: 'primary' };
  return { label: 'Unhealthy', tone: 'destructive' };
}

export function Dashboard() {
  const { metrics: metricsQuery, buckets: bucketsQuery, health: healthQuery, isLoading } = useDashboardData();
  const metrics = metricsQuery.data;
  const buckets = bucketsQuery.data ?? [];
  const clusterHealth = healthQuery.data ?? null;
  const health = deriveHealth(clusterHealth);

  return (
    <div>
      <PageHeader
        title="Dashboard"
        subtitle={
          clusterHealth
            ? `${clusterHealth.connectedNodes}/${clusterHealth.knownNodes} nodes connected`
            : 'Loading cluster status…'
        }
      />

      {isLoading ? (
        <div className="flex min-h-[360px] items-center justify-center">
          <div className="text-center">
            <div className="inline-block h-6 w-6 animate-spin rounded-full border-2 border-[var(--primary)] border-r-transparent" />
            <p className="mt-3 text-[13.5px] text-[var(--muted-foreground)]">Loading dashboard…</p>
          </div>
        </div>
      ) : (
        <div className="space-y-6 px-6 py-5">
          {/* KPI row */}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <StatCard
              label="Total storage"
              value={metrics ? formatBytes(metrics.totalSize) : '—'}
              sub={`across ${metrics?.bucketCount ?? 0} bucket${metrics?.bucketCount === 1 ? '' : 's'}`}
              icon={<HardDrive />}
            />
            <StatCard
              label="Objects"
              value={metrics?.objectCount.toLocaleString() ?? '—'}
              sub="files and folders"
              icon={<FolderOpen />}
            />
            <StatCard
              label="Buckets"
              value={metrics?.bucketCount.toLocaleString() ?? '—'}
              sub="active storage buckets"
              icon={<Database />}
            />
            <StatCard
              label="Cluster"
              value={health.label}
              valueTone={health.tone}
              sub={
                clusterHealth
                  ? `${clusterHealth.storageNodesUp}/${clusterHealth.storageNodes} storage nodes`
                  : '—'
              }
              icon={health.label === 'Unhealthy' ? <AlertCircle /> : <Zap />}
              iconTone={health.tone}
            />
          </div>

          {/* Cluster row */}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <StatCard
              label="Storage nodes"
              value={clusterHealth ? `${clusterHealth.storageNodesUp}/${clusterHealth.storageNodes}` : '—'}
              sub="healthy"
              icon={<Server />}
            />
            <StatCard
              label="Partitions"
              value={clusterHealth ? `${clusterHealth.partitionsAllOk}/${clusterHealth.partitions}` : '—'}
              sub="healthy"
              icon={<Zap />}
            />
            <StatCard
              label="Connected nodes"
              value={clusterHealth ? `${clusterHealth.connectedNodes}/${clusterHealth.knownNodes}` : '—'}
              sub="cluster membership"
              icon={<Server />}
            />
          </div>

          {/* Charts */}
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <Card title="Storage usage by bucket" description="Distribution of storage across buckets">
              {metrics?.usageByBucket && metrics.usageByBucket.length > 0 ? (
                <BucketUsageChart data={metrics.usageByBucket} />
              ) : (
                <div className="py-8 text-center text-[13.5px] text-[var(--muted-foreground)]">No data available</div>
              )}
            </Card>

            <Card title="Breakdown" description="Detailed breakdown of storage across all buckets">
              {metrics?.usageByBucket && metrics.usageByBucket.length > 0 ? (
                <div className="space-y-4">
                  {metrics.usageByBucket.map((bucket) => (
                    <div key={bucket.bucketName} className="space-y-1.5">
                      <div className="flex items-center justify-between gap-2 text-[13.5px]">
                        <span className="truncate font-medium">{bucket.bucketName}</span>
                        <div className="flex items-center gap-3 text-[13px] text-[var(--muted-foreground)]">
                          <span>{bucket.objectCount.toLocaleString()} objects</span>
                          <span className="font-medium text-[var(--foreground)]">{formatBytes(bucket.size)}</span>
                          <span className="w-10 text-right">{bucket.percentage.toFixed(1)}%</span>
                        </div>
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-[var(--muted)]">
                        <div
                          className="h-full bg-[var(--primary)] transition-all"
                          style={{ width: `${bucket.percentage}%` }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="py-8 text-center text-[13.5px] text-[var(--muted-foreground)]">No buckets available</div>
              )}
            </Card>
          </div>

          {/* Recent buckets */}
          <Card title="Recent buckets" description="Your most recently created buckets">
            {buckets.length === 0 ? (
              <EmptyState
                icon={<Database />}
                title="No buckets yet"
                description="Create your first bucket from the Buckets page to start storing objects."
                tone="neutral"
              />
            ) : (
              <ul className="divide-y divide-[var(--border)]">
                {buckets.slice(0, 5).map((bucket) => (
                  <li key={bucket.name}>
                    <Link
                      to={`/buckets/${bucket.name}/objects`}
                      className="-mx-2 flex items-center gap-3 rounded-lg px-2 py-3 transition-colors hover:bg-[var(--muted)]"
                    >
                      <IconTile icon={<Database />} tone="primary" size="md" />
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-[14px] font-medium">{bucket.name}</p>
                        <p className="truncate text-[12.5px] text-[var(--muted-foreground)]">
                          Created {new Date(bucket.creationDate).toLocaleDateString()}
                        </p>
                      </div>
                      <div className="text-right">
                        <p className="text-[14px] font-medium">{bucket.objectCount?.toLocaleString() ?? '—'} objects</p>
                        <p className="text-[12.5px] text-[var(--muted-foreground)]">
                          {bucket.size ? formatBytes(bucket.size) : '—'}
                        </p>
                      </div>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  sub,
  icon,
  iconTone = 'primary',
  valueTone = 'neutral',
}: {
  label: string;
  value: string;
  sub?: string;
  icon: React.ReactNode;
  iconTone?: StatTone;
  valueTone?: StatTone;
}) {
  const valueColor =
    valueTone === 'primary'
      ? 'text-[var(--primary)]'
      : valueTone === 'destructive'
      ? 'text-[var(--destructive)]'
      : 'text-[var(--foreground)]';
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--card)] p-4">
      <div className="flex items-start justify-between gap-3">
        <span className="text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--muted-foreground)]">
          {label}
        </span>
        <IconTile icon={icon} tone={iconTone} size="sm" />
      </div>
      <div className={`mt-2 text-[26px] font-semibold tracking-[-0.02em] leading-none ${valueColor}`}>
        {value}
      </div>
      {sub && <div className="mt-1.5 text-[13px] text-[var(--muted-foreground)]">{sub}</div>}
    </div>
  );
}

function Card({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
      <header className="border-b border-[var(--border)] px-5 py-3">
        <h2 className="text-[15px] font-semibold">{title}</h2>
        {description && <p className="mt-0.5 text-[12.5px] text-[var(--muted-foreground)]">{description}</p>}
      </header>
      <div className="px-5 py-5">{children}</div>
    </section>
  );
}
