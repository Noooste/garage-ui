// Bucket types
export interface BucketQuotas {
  maxSize?: number;
  maxObjects?: number;
}

export interface Bucket {
  name: string;
  creationDate: string;
  objectCount?: number;
  size?: number;
  region?: string;
  websiteAccess: boolean;
  websiteConfig?: {
    indexDocument: string;
    errorDocument?: string;
  };
  quotas?: BucketQuotas | null;
}

export interface BucketDetails extends Bucket {
  versioning?: boolean;
  encryption?: boolean;
  publicAccess?: boolean;
  lifecycleRules?: LifecycleRule[];
}

export interface LifecycleRule {
  id: string;
  enabled: boolean;
  prefix?: string;
  expirationDays?: number;
  transitions?: Transition[];
}

export interface Transition {
  days: number;
  storageClass: string;
}

// Object types
export interface S3Object {
  key: string;
  size: number;
  lastModified: string;
  etag?: string;
  contentType?: string;
  storageClass?: string;
  isFolder?: boolean;
}

export interface ObjectListResponse {
  bucket: string;
  objects: S3Object[];
  prefixes: string[];
  count: number;
  isTruncated: boolean;
  nextContinuationToken?: string;
}

export interface ObjectMetadata {
  key: string;
  size: number;
  lastModified: string;
  contentType: string;
  etag: string;
  storageClass?: string;
  metadata?: Record<string, string>;
  versionId?: string;
}

// Access Control types
export interface AccessKey {
  accessKeyId: string;
  name: string;
  secretKey?: string;
  createdAt: string;
  status: 'active' | 'inactive';
  permissions: BucketPermission[];
  expiration?: string;
}

export interface BucketPermission {
  bucketId: string;
  bucketName: string;
  read: boolean;
  write: boolean;
  owner: boolean;
}

export interface Permission {
  resource: string;
  actions: string[];
  effect: 'Allow' | 'Deny';
}

export interface BucketPolicy {
  bucketName: string;
  policy: PolicyStatement[];
}

export interface PolicyStatement {
  sid?: string;
  effect: 'Allow' | 'Deny';
  principal: string | string[];
  action: string | string[];
  resource: string | string[];
  condition?: Record<string, any>;
}

// User types
export interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'user' | 'readonly';
  createdAt: string;
}

// Storage Analytics types
export interface StorageMetrics {
  totalSize: number;
  objectCount: number;
  bucketCount: number;
  usageByBucket: BucketUsage[];
  requestMetrics: RequestMetrics;
}

export interface BucketUsage {
  bucketName: string;
  size: number;
  objectCount: number;
  percentage: number;
}

export interface RequestMetrics {
  getRequests: number;
  putRequests: number;
  deleteRequests: number;
  listRequests: number;
  period: string;
}

// Upload types
export interface UploadTask {
  id: string;
  file: File;
  key: string;
  bucket: string;
  progress: number;
  status: 'pending' | 'uploading' | 'completed' | 'error';
  error?: string;
}

// API Response types
export interface ApiResponse<T> {
  data?: T;
  error?: string;
  message?: string;
}

// Filter and Sort types
export interface TableFilter {
  search?: string;
  sortBy?: string;
  sortOrder?: 'asc' | 'desc';
  pageSize?: number;
  page?: number;
}

// Garage Cluster types
export interface ClusterHealth {
  status: string;
  connectedNodes: number;
  knownNodes: number;
  storageNodes: number;
  storageNodesUp: number;
  partitions: number;
  partitionsQuorum: number;
  partitionsAllOk: number;
}

export interface ClusterStatistics {
  timestamp: number;
  uptime: number;
  freeform: string;
  [key: string]: any;
}

export interface ClusterStatus {
  layoutVersion: number;
  nodes: ClusterNode[];
}

export interface ClusterNode {
  id: string;
  isUp: boolean;
  lastSeenSecsAgo?: number;
  hostname?: string;
  addr?: string;
  garageVersion?: string;
  role?: NodeRole;
  draining: boolean;
  dataPartition?: FreeSpaceInfo;
  metadataPartition?: FreeSpaceInfo;
}

export interface NodeRole {
  zone: string;
  capacity?: number;
  tags: string[];
}

export interface FreeSpaceInfo {
  available: number;
  total: number;
}

export interface LocalNodeInfo {
  nodeId: string;
  garageVersion: string;
  rustVersion: string;
  dbEngine: string;
  garageFeatures?: string[];
}

export interface MultiNodeResponse {
  success: Record<string, LocalNodeInfo>;
  error: Record<string, string>;
}

export interface NodeStatistics {
  freeform: string;
}

export interface MultiNodeStatisticsResponse {
  success: Record<string, NodeStatistics>;
  error: Record<string, string>;
}

export interface GarageCapabilities {
  garageApiVersion: string;
  features: {
    clusterStatistics: boolean;
    nodeInfo: boolean;
    nodeStatistics: boolean;
  };
}

export interface NodeInfo {
  nodeId: string;
  version: string;
  rustVersion: string;
  uptime: number;
  dbSize: number;
  blockReferenceTableSize: number;
  blockMetricsTableSize: number;
  objectTableSize: number;
  objectVersionTableSize: number;
  bucketTableSize: number;
  bucketAliasTableSize: number;
  [key: string]: any;
}

export interface RequestTypeMetrics {
  read: number;
  write: number;
  delete: number;
  list: number;
}

export interface GarageMetrics extends StorageMetrics {
  clusterHealth?: ClusterHealth;
  clusterStatistics?: ClusterStatistics;
  nodeInfo?: NodeInfo;
  requestTypeMetrics?: RequestTypeMetrics;
}
