import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {PageHeader} from '@/components/ui/page-header';
import {formatBytes} from '@/lib/file-utils';
import {Activity, AlertCircle, CheckCircle2, Clock, Cpu, Database, Info, Network, Server, XCircle,} from 'lucide-react';
import {useQuery} from '@tanstack/react-query';
import {garageApi} from '@/lib/api';
import {Badge} from '@/components/ui/badge';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import type {ClusterNode, LocalNodeInfo, NodeStatistics} from '@/types';
import {useState} from 'react';
import { useCapabilities } from '@/hooks/useCapabilities';

function UnsupportedFeatureCard({ title, description }: { title: string; description?: string }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-[var(--muted-foreground)]">
          <Info className="h-4 w-4" />
          {title}
        </CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>
        <p className="text-sm text-[var(--muted-foreground)]">
          Requires Garage v2.0+
        </p>
      </CardContent>
    </Card>
  );
}

export function Cluster() {
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  const { data: capabilities } = useCapabilities();
  const features = capabilities?.features;

  const { data: health, isLoading: healthLoading } = useQuery({
    queryKey: ['cluster-health'],
    queryFn: () => garageApi.getClusterHealth(),
    refetchInterval: 10000,
  });

  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ['cluster-status'],
    queryFn: () => garageApi.getClusterStatus(),
    refetchInterval: 15000,
  });

  const { data: statistics, isLoading: statisticsLoading } = useQuery({
    queryKey: ['cluster-statistics'],
    queryFn: () => garageApi.getClusterStatistics(),
    refetchInterval: 30000,
    enabled: features?.clusterStatistics !== false,
  });

  const { data: nodeInfo, isLoading: nodeInfoLoading } = useQuery({
    queryKey: ['node-info', selectedNodeId || '*'],
    queryFn: () => garageApi.getNodeInfo(selectedNodeId || '*'),
    enabled: features?.nodeInfo !== false && (!!selectedNodeId || selectedNodeId === null),
  });

  const { data: nodeStats } = useQuery({
    queryKey: ['node-statistics', selectedNodeId || '*'],
    queryFn: () => garageApi.getNodeStatistics(selectedNodeId || '*'),
    enabled: features?.nodeStatistics !== false && !!selectedNodeId,
  });

  const isLoading = healthLoading || statusLoading || statisticsLoading;

  const getHealthStatus = () => {
    if (!health) return { color: 'text-gray-500', bgColor: 'bg-gray-100', label: 'Unknown', icon: AlertCircle };
    if (
      health.storageNodesUp === health.storageNodes &&
      health.partitionsAllOk === health.partitions &&
      health.connectedNodes === health.knownNodes
    ) {
      return { color: 'text-green-600', bgColor: 'bg-green-100', label: 'Healthy', icon: CheckCircle2 };
    }
    if (health.storageNodesUp > 0 && health.partitionsQuorum > 0) {
      return { color: 'text-yellow-600', bgColor: 'bg-yellow-100', label: 'Degraded', icon: AlertCircle };
    }
    return { color: 'text-red-600', bgColor: 'bg-red-100', label: 'Unhealthy', icon: XCircle };
  };

  const healthStatus = getHealthStatus();
  const HealthIcon = healthStatus.icon;

  const getNodeStatus = (node: ClusterNode) => {
    if (!node.isUp) {
      return { color: 'text-red-600', bgColor: 'bg-red-100', label: 'Down', icon: XCircle };
    }
    if (node.draining) {
      return { color: 'text-yellow-600', bgColor: 'bg-yellow-100', label: 'Draining', icon: AlertCircle };
    }
    return { color: 'text-green-600', bgColor: 'bg-green-100', label: 'Up', icon: CheckCircle2 };
  };

  const formatUptime = (seconds?: number) => {
    if (!seconds) return 'N/A';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${days}d ${hours}h ${minutes}m`;
  };

  if (isLoading) {
    return (
      <div>
        <PageHeader title="Cluster" />
        <div className="p-4 sm:p-6 flex items-center justify-center min-h-[400px]">
          <div className="text-center">
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
            <p className="mt-2 text-sm text-muted-foreground">Loading cluster information...</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader title="Cluster management" subtitle="Node layout, partitions, and health" />
      <div className="p-4 sm:p-6 space-y-4 sm:space-y-6">
        {/* Cluster Health Overview */}
        <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Cluster Status</CardTitle>
              <HealthIcon className={`h-4 w-4 ${healthStatus.color}`} />
            </CardHeader>
            <CardContent>
              <div className={`text-2xl font-bold ${healthStatus.color}`}>{healthStatus.label}</div>
              <p className="text-xs text-muted-foreground mt-2">
                Layout v{status?.layoutVersion || 0}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Connected Nodes</CardTitle>
              <Network className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {health?.connectedNodes || 0}/{health?.knownNodes || 0}
              </div>
              <p className="text-xs text-muted-foreground">
                Nodes online
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Storage Nodes</CardTitle>
              <Server className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {health?.storageNodesUp || 0}/{health?.storageNodes || 0}
              </div>
              <p className="text-xs text-muted-foreground">
                Healthy storage nodes
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Partitions</CardTitle>
              <Database className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {health?.partitionsAllOk || 0}/{health?.partitions || 0}
              </div>
              <p className="text-xs text-muted-foreground">
                Healthy partitions
              </p>
            </CardContent>
          </Card>
        </div>

        {/* Tabs for different views */}
        <Tabs defaultValue="nodes" className="space-y-4">
          <TabsList>
            <TabsTrigger value="nodes">Nodes</TabsTrigger>
            <TabsTrigger value="statistics">Statistics</TabsTrigger>
            <TabsTrigger value="details">Details</TabsTrigger>
          </TabsList>

          {/* Nodes Tab */}
          <TabsContent value="nodes" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Cluster Nodes</CardTitle>
                <CardDescription>
                  Overview of all nodes in the Garage cluster
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {status?.nodes && status.nodes.length > 0 ? (
                    status.nodes.map((node) => {
                      const nodeStatus = getNodeStatus(node);
                      const NodeIcon = nodeStatus.icon;
                      const dataUsage = node.dataPartition
                        ? ((node.dataPartition.total - node.dataPartition.available) / node.dataPartition.total) * 100
                        : 0;
                      const metadataUsage = node.metadataPartition
                        ? ((node.metadataPartition.total - node.metadataPartition.available) / node.metadataPartition.total) * 100
                        : 0;

                      return (
                        <Card
                          key={node.id}
                          className={`cursor-pointer transition-all hover:shadow-md ${
                            selectedNodeId === node.id ? 'ring-2 ring-primary' : ''
                          }`}
                          onClick={() => setSelectedNodeId(node.id)}
                        >
                          <CardContent className="pt-6">
                            <div className="flex items-start justify-between">
                              <div className="flex-1 space-y-2">
                                <div className="flex items-center gap-3">
                                  <NodeIcon className={`h-5 w-5 ${nodeStatus.color}`} />
                                  <div>
                                    <div className="font-mono text-sm font-medium">
                                      {node.id.substring(0, 16)}...
                                    </div>
                                    {node.hostname && (
                                      <div className="text-xs text-muted-foreground">{node.hostname}</div>
                                    )}
                                  </div>
                                </div>

                                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-2">
                                  <div>
                                    <div className="text-xs text-muted-foreground">Status</div>
                                    <Badge variant={node.isUp ? 'primary' : 'danger'} className="mt-1">
                                      {nodeStatus.label}
                                    </Badge>
                                  </div>

                                  {node.addr && (
                                    <div>
                                      <div className="text-xs text-muted-foreground">Address</div>
                                      <div className="text-sm font-mono">{node.addr}</div>
                                    </div>
                                  )}

                                  {node.garageVersion && (
                                    <div>
                                      <div className="text-xs text-muted-foreground">Version</div>
                                      <div className="text-sm">{node.garageVersion}</div>
                                    </div>
                                  )}

                                  {node.role && (
                                    <div>
                                      <div className="text-xs text-muted-foreground">Zone</div>
                                      <div className="text-sm">{node.role.zone}</div>
                                    </div>
                                  )}
                                </div>

                                {node.role?.capacity && (
                                  <div className="pt-2">
                                    <div className="text-xs text-muted-foreground mb-1">
                                      Capacity: {formatBytes(node.role.capacity)}
                                    </div>
                                  </div>
                                )}

                                {(node.dataPartition || node.metadataPartition) && (
                                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                                    {node.dataPartition && (
                                      <div>
                                        <div className="text-xs text-muted-foreground mb-1">
                                          Data Partition: {formatBytes(node.dataPartition.total - node.dataPartition.available)} / {formatBytes(node.dataPartition.total)}
                                        </div>
                                        <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                                          <div
                                            className={`h-full transition-all ${
                                              dataUsage > 90 ? 'bg-red-500' : dataUsage > 70 ? 'bg-yellow-500' : 'bg-green-500'
                                            }`}
                                            style={{ width: `${dataUsage}%` }}
                                          />
                                        </div>
                                      </div>
                                    )}

                                    {node.metadataPartition && (
                                      <div>
                                        <div className="text-xs text-muted-foreground mb-1">
                                          Metadata Partition: {formatBytes(node.metadataPartition.total - node.metadataPartition.available)} / {formatBytes(node.metadataPartition.total)}
                                        </div>
                                        <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                                          <div
                                            className={`h-full transition-all ${
                                              metadataUsage > 90 ? 'bg-red-500' : metadataUsage > 70 ? 'bg-yellow-500' : 'bg-green-500'
                                            }`}
                                            style={{ width: `${metadataUsage}%` }}
                                          />
                                        </div>
                                      </div>
                                    )}
                                  </div>
                                )}

                                {!node.isUp && node.lastSeenSecsAgo !== undefined && (
                                  <div className="text-xs text-muted-foreground pt-2">
                                    <Clock className="inline h-3 w-3 mr-1" />
                                    Last seen: {node.lastSeenSecsAgo === null ? 'Never' : formatUptime(node.lastSeenSecsAgo) + ' ago'}
                                  </div>
                                )}
                              </div>
                            </div>
                          </CardContent>
                        </Card>
                      );
                    })
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      <Server className="h-12 w-12 mx-auto mb-2 opacity-50" />
                      <p>No nodes found in the cluster</p>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Statistics Tab */}
          <TabsContent value="statistics" className="space-y-4">
            {features?.clusterStatistics === false ? (
              <UnsupportedFeatureCard title="Cluster Statistics" description="Global cluster metrics and statistics" />
            ) : (
              <Card>
                <CardHeader>
                  <CardTitle>Cluster Statistics</CardTitle>
                  <CardDescription>
                    Detailed statistics and metrics from the Garage cluster
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {statistics ? (
                    <div className="space-y-4">
                      <div className="rounded-lg bg-muted p-4">
                        <pre className="text-xs overflow-x-auto whitespace-pre-wrap font-mono">
                          {statistics.freeform}
                        </pre>
                      </div>
                    </div>
                  ) : (
                    <div className="text-center text-muted-foreground py-8">
                      <Activity className="h-12 w-12 mx-auto mb-2 opacity-50" />
                      <p>No statistics available</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}
          </TabsContent>

          {/* Details Tab */}
          <TabsContent value="details" className="space-y-4">
            {selectedNodeId ? (
              <>
                {features?.nodeInfo === false ? (
                  <UnsupportedFeatureCard title="Node Details" description="Per-node information and configuration" />
                ) : (
                  <Card>
                    <CardHeader>
                      <CardTitle>Node Information</CardTitle>
                      <CardDescription>
                        Detailed information for node: {selectedNodeId.substring(0, 16)}...
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      {nodeInfoLoading ? (
                        <div className="text-center py-8">
                          <div className="inline-block h-6 w-6 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
                          <p className="mt-2 text-sm text-muted-foreground">Loading node info...</p>
                        </div>
                      ) : nodeInfo ? (
                        <div className="space-y-4">
                          {/* Success responses */}
                          {Object.entries(nodeInfo.success || {}).map(([nodeId, info]) => (
                            <div key={nodeId} className="space-y-3">
                              <div className="flex items-center gap-2 mb-3">
                                <Info className="h-4 w-4 text-primary" />
                                <h4 className="font-medium">
                                  Node: {nodeId.substring(0, 16)}...
                                </h4>
                              </div>

                              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                <div className="rounded-lg border p-3">
                                  <div className="text-xs text-muted-foreground mb-1">Node ID</div>
                                  <div className="font-mono text-sm break-all">{(info as LocalNodeInfo).nodeId}</div>
                                </div>

                                <div className="rounded-lg border p-3">
                                  <div className="text-xs text-muted-foreground mb-1">Garage Version</div>
                                  <div className="text-sm">{(info as LocalNodeInfo).garageVersion}</div>
                                </div>

                                <div className="rounded-lg border p-3">
                                  <div className="text-xs text-muted-foreground mb-1">Rust Version</div>
                                  <div className="text-sm">{(info as LocalNodeInfo).rustVersion}</div>
                                </div>

                                <div className="rounded-lg border p-3">
                                  <div className="text-xs text-muted-foreground mb-1">Database Engine</div>
                                  <div className="text-sm">{(info as LocalNodeInfo).dbEngine}</div>
                                </div>
                              </div>

                              {(info as LocalNodeInfo).garageFeatures && (info as LocalNodeInfo).garageFeatures!.length > 0 && (
                                <div className="rounded-lg border p-3">
                                  <div className="text-xs text-muted-foreground mb-2">Garage Features</div>
                                  <div className="flex flex-wrap gap-2">
                                    {(info as LocalNodeInfo).garageFeatures!.map((feature) => (
                                      <Badge key={feature} variant="neutral">
                                        {feature}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                              )}
                            </div>
                          ))}

                          {/* Error responses */}
                          {Object.entries(nodeInfo.error || {}).map(([nodeId, error]) => (
                            <div key={nodeId} className="rounded-lg border border-red-200 bg-red-50 p-3">
                              <div className="flex items-center gap-2 text-red-600 mb-1">
                                <XCircle className="h-4 w-4" />
                                <div className="font-medium">Error for node {nodeId.substring(0, 16)}...</div>
                              </div>
                              <div className="text-sm text-red-800">{error}</div>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="text-center text-muted-foreground py-8">
                          <Info className="h-12 w-12 mx-auto mb-2 opacity-50" />
                          <p>No node information available</p>
                        </div>
                      )}
                    </CardContent>
                  </Card>
                )}

                {features?.nodeStatistics === false ? (
                  <UnsupportedFeatureCard title="Node Statistics" description="Per-node performance metrics" />
                ) : nodeStats ? (
                  <Card>
                    <CardHeader>
                      <CardTitle>Node Statistics</CardTitle>
                      <CardDescription>
                        Performance metrics for the selected node
                      </CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-4">
                        {/* Success responses */}
                        {Object.entries(nodeStats.success || {}).map(([nodeId, stats]) => (
                          <div key={nodeId} className="space-y-3">
                            <div className="flex items-center gap-2 mb-3">
                              <Cpu className="h-4 w-4 text-primary" />
                              <h4 className="font-medium">
                                Statistics for: {nodeId.substring(0, 16)}...
                              </h4>
                            </div>

                            <div className="rounded-lg bg-muted p-4">
                              <pre className="text-xs overflow-x-auto whitespace-pre-wrap font-mono">
                                {(stats as NodeStatistics).freeform}
                              </pre>
                            </div>
                          </div>
                        ))}

                        {/* Error responses */}
                        {Object.entries(nodeStats.error || {}).map(([nodeId, error]) => (
                          <div key={nodeId} className="rounded-lg border border-red-200 bg-red-50 p-3">
                            <div className="flex items-center gap-2 text-red-600 mb-1">
                              <XCircle className="h-4 w-4" />
                              <div className="font-medium">Error for node {nodeId.substring(0, 16)}...</div>
                            </div>
                            <div className="text-sm text-red-800">{error}</div>
                          </div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                ) : null}
              </>
            ) : (
              <Card>
                <CardContent className="pt-6">
                  <div className="text-center text-muted-foreground py-12">
                    <Server className="h-16 w-16 mx-auto mb-4 opacity-50" />
                    <p className="text-lg font-medium mb-2">Select a Node</p>
                    <p className="text-sm">
                      Click on a node in the Nodes tab to view detailed information and statistics
                    </p>
                  </div>
                </CardContent>
              </Card>
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}


