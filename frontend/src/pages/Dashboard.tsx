import { useMemo, useState } from 'react';
import { AlertCircle, ArrowRight, CircleAlert, CircleCheck, Database, FolderOpen, HardDrive, Zap } from 'lucide-react';
import { Link } from 'react-router';
import { PageHeader } from '@/components/ui/page-header';
import { IconTile } from '@/components/ui/icon-tile';
import { EmptyState } from '@/components/ui/empty-state';
import { BucketUsageChart } from '@/components/charts/BucketUsageChart';
import { useDashboardData } from '@/hooks/useApi';
import { formatBytes, formatCount } from '@/lib/file-utils';
import type { BucketUsage, ClusterHealth } from '@/types';

type StatTone = 'primary' | 'destructive' | 'neutral';

// Breakdown rows shown before "Show all"
const BREAKDOWN_ROWS = 5;

const objectLabel = (n: number) => `${formatCount(n)} ${n === 1 ? 'object' : 'objects'}`;
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
  const usage = useMemo(
    () => [...(metrics?.usageByBucket ?? [])].sort((a, b) => b.size - a.size),
    [metrics?.usageByBucket],
  );

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
              value={metrics ? formatCount(metrics.objectCount) : '—'}
              valueTitle={metrics?.objectCount.toLocaleString()}
              sub="files and folders"
              icon={<FolderOpen />}
            />
            <StatCard
              label="Buckets"
              value={metrics ? formatCount(metrics.bucketCount) : '—'}
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

          <div className="grid grid-cols-1 items-start gap-4 xl:grid-cols-2">
            {/* Storage: where the space goes */}
            <div className="space-y-4">
              <Card title="Storage usage by bucket" description="Distribution of storage across buckets">
                {usage.length > 0 ? (
                  <BucketUsageChart data={usage} />
                ) : (
                  <div className="py-8 text-center text-[13.5px] text-[var(--muted-foreground)]">No data available</div>
                )}
              </Card>

              <Card title="Breakdown" description="Size and object count per bucket, largest first">
                {usage.length > 0 ? (
                  <Breakdown usage={usage} />
                ) : (
                  <div className="py-8 text-center text-[13.5px] text-[var(--muted-foreground)]">No buckets available</div>
                )}
              </Card>
            </div>

            {/* Cluster state and recent activity */}
            <div className="space-y-4">
              <Card
                title="Cluster"
                description="Node and partition health"
                action={<CardLink to="/cluster">Details</CardLink>}
              >
                {clusterHealth ? (
                  <ul className="divide-y divide-[var(--border)]">
                    <HealthRow
                      label="Storage nodes"
                      up={clusterHealth.storageNodesUp}
                      total={clusterHealth.storageNodes}
                      note="up"
                    />
                    <HealthRow
                      label="Partitions"
                      up={clusterHealth.partitionsAllOk}
                      total={clusterHealth.partitions}
                      note="fully replicated"
                    />
                    <HealthRow
                      label="Connected nodes"
                      up={clusterHealth.connectedNodes}
                      total={clusterHealth.knownNodes}
                      note="reachable"
                    />
                  </ul>
                ) : (
                  <div className="py-6 text-center text-[13.5px] text-[var(--muted-foreground)]">Cluster status unavailable</div>
                )}
              </Card>

              <Card
                title="Recent buckets"
                description="Your most recently created buckets"
                action={buckets.length > 0 && <CardLink to="/buckets">View all</CardLink>}
              >
                {buckets.length === 0 ? (
                  <EmptyState
                    icon={<Database />}
                    title="No buckets yet"
                    description="Create your first bucket from the Buckets page to start storing objects."
                    tone="neutral"
                  />
                ) : (
                  <ul className="-my-2 divide-y divide-[var(--border)]">
                    {buckets.slice(0, 5).map((bucket) => (
                      <li key={bucket.name}>
                        <Link
                          to={`/buckets/${bucket.name}/objects`}
                          className="-mx-2 flex items-center gap-3 rounded-lg px-2 py-2.5 transition-colors hover:bg-[var(--muted)]"
                        >
                          <IconTile icon={<Database />} tone="primary" size="md" />
                          <div className="min-w-0 flex-1">
                            <p className="truncate text-[14px] font-medium">{bucket.name}</p>
                            <p className="truncate text-[12.5px] text-[var(--muted-foreground)]">
                              Created {new Date(bucket.creationDate).toLocaleDateString()}
                            </p>
                          </div>
                          <div className="text-right">
                            <p className="text-[14px] font-medium tabular-nums" title={bucket.objectCount?.toLocaleString()}>
                              {bucket.objectCount !== undefined ? objectLabel(bucket.objectCount) : '—'}
                            </p>
                            <p className="text-[12.5px] tabular-nums text-[var(--muted-foreground)]">
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
          </div>
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
  valueTitle,
}: {
  label: string;
  value: string;
  sub?: string;
  icon: React.ReactNode;
  iconTone?: StatTone;
  valueTone?: StatTone;
  valueTitle?: string;
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
      <div title={valueTitle} className={`mt-2 text-[26px] font-semibold tracking-[-0.02em] leading-none ${valueColor}`}>
        {value}
      </div>
      {sub && <div className="mt-1.5 text-[13px] text-[var(--muted-foreground)]">{sub}</div>}
    </div>
  );
}

function Card({
  title,
  description,
  action,
  children,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
      <header className="flex items-start justify-between gap-3 border-b border-[var(--border)] px-5 py-3">
        <div className="min-w-0">
          <h2 className="text-[15px] font-semibold">{title}</h2>
          {description && <p className="mt-0.5 text-[12.5px] text-[var(--muted-foreground)]">{description}</p>}
        </div>
        {action}
      </header>
      <div className="px-5 py-5">{children}</div>
    </section>
  );
}

function CardLink({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <Link
      to={to}
      className="mt-0.5 inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 py-0.5 text-[12.5px] font-medium text-[var(--muted-foreground)] transition-colors hover:text-[var(--foreground)] focus-visible:outline-2 focus-visible:outline-[var(--primary)]"
    >
      {children}
      <ArrowRight className="size-3.5" />
    </Link>
  );
}

function HealthRow({ label, up, total, note }: { label: string; up: number; total: number; note: string }) {
  const ok = up === total;
  const Icon = ok ? CircleCheck : CircleAlert;
  return (
    <li className="flex items-center gap-3 py-2.5 first:pt-0 last:pb-0">
      <Icon className={`size-4 shrink-0 ${ok ? 'text-[var(--primary)]' : 'text-[var(--destructive)]'}`} />
      <span className="flex-1 text-[13.5px]">{label}</span>
      <span className="text-[12.5px] text-[var(--muted-foreground)]">{note}</span>
      <span className="w-16 text-right text-[14px] font-semibold tabular-nums">
        {up}/{total}
      </span>
    </li>
  );
}

function Breakdown({ usage }: { usage: BucketUsage[] }) {
  const [expanded, setExpanded] = useState(false);
  const rows = expanded ? usage : usage.slice(0, BREAKDOWN_ROWS);

  return (
    <div className="@container">
      <ul className="-my-1.5">
        {rows.map((bucket) => (
          <li
            key={bucket.bucketName}
            className="grid grid-cols-[minmax(0,1fr)_4.5rem_3.25rem] items-center gap-x-3 py-1.5 text-[13px] @md:grid-cols-[minmax(0,1fr)_6rem_6rem_4.5rem_3.25rem]"
          >
            <span className="truncate font-medium">{bucket.bucketName}</span>
            <div className="hidden h-1.5 overflow-hidden rounded-full bg-[var(--muted)] @md:block">
              {bucket.percentage > 0 && (
                <div
                  className="h-full min-w-0.5 rounded-full bg-[var(--primary)]"
                  style={{ width: `${bucket.percentage}%` }}
                />
              )}
            </div>
            <span
              className="hidden whitespace-nowrap text-right tabular-nums text-[var(--muted-foreground)] @md:block"
              title={bucket.objectCount.toLocaleString()}
            >
              {objectLabel(bucket.objectCount)}
            </span>
            <span className="text-right font-medium tabular-nums">{formatBytes(bucket.size)}</span>
            <span className="text-right tabular-nums text-[var(--muted-foreground)]">
              {bucket.percentage.toFixed(1)}%
            </span>
          </li>
        ))}
      </ul>
      {usage.length > BREAKDOWN_ROWS && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
          className="mt-3 w-full rounded-md border border-[var(--border)] py-1.5 text-[12.5px] font-medium text-[var(--muted-foreground)] transition-colors hover:bg-[var(--muted)] hover:text-[var(--foreground)] focus-visible:outline-2 focus-visible:outline-[var(--primary)]"
        >
          {expanded ? 'Show fewer' : `Show all ${usage.length} buckets`}
        </button>
      )}
    </div>
  );
}
