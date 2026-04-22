import axios from 'axios';
import {toast} from 'sonner';
import type {
  AccessKey,
  ApiResponse,
  Bucket,
  BucketDetails,
  ClusterHealth,
  ClusterStatistics,
  ClusterStatus,
  GarageCapabilities,
  GarageMetrics,
  MultiNodeResponse,
  MultiNodeStatisticsResponse,
  ObjectListResponse,
  ObjectMetadata,
  S3Object,
  StorageMetrics,
} from '@/types';
import type { AuthUser } from '@/types/auth';

// Helper function to encode object keys for URLs
// Encodes the entire key including slashes to ensure proper handling of special characters
const encodeObjectKey = (key: string): string => {
  return encodeURIComponent(key);
};

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Separate axios instance for auth endpoints (which are not under /api)
const authApiClient = axios.create({
  baseURL: '/auth',
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth-token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

authApiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth-token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => {
    // If response has success=false in data, treat it as an error
    if (response.data && response.data.success === false && response.data.error) {
      const error = response.data.error;
      const errorMessage = error.message || 'An error occurred';
      const errorCode = error.code || 'UNKNOWN_ERROR';

      // Display toast with error details
      toast.error(errorMessage, {
        description: `Error Code: ${errorCode}`,
      });

      // Reject the promise so it's treated as an error
      return Promise.reject(new Error(errorMessage));
    }
    return response;
  },
  (error) => {
    // Handle 401 Unauthorized - redirect to login
    if (error.response?.status === 401) {
      // Clear auth token
      localStorage.removeItem('auth-token');

      // Only redirect if not already on login page
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }

      return Promise.reject(error);
    }

    // 501 Not Implemented = expected for unsupported Garage version features
    if (error.response?.status === 501) {
      return Promise.reject(error);
    }

    // Handle axios errors
    if (error.response) {
      // Server responded with error status
      const data = error.response.data;

      if (data && data.error) {
        const errorMessage = data.error.message || 'An error occurred';
        const errorCode = data.error.code || 'UNKNOWN_ERROR';

        toast.error(errorMessage, {
          description: `Error Code: ${errorCode}`,
        });
      } else {
        // Generic HTTP error
        toast.error(`Request failed: ${error.response.status}`, {
          description: error.response.statusText || 'Unknown error',
        });
      }
    } else if (error.request) {
      // Request made but no response received
      toast.error('Network Error', {
        description: 'Unable to reach the server. Please check your connection.',
      });
    } else {
      // Something else happened
      toast.error('Error', {
        description: error.message || 'An unexpected error occurred',
      });
    }

    return Promise.reject(error);
  }
);

// Auth API
export const authApi = {
  getConfig: async () => {
    const response = await authApiClient.get<{
      admin: { enabled: boolean };
      oidc: { enabled: boolean; provider?: string };
    }>('/config');
    return response;
  },

  loginAdmin: async (username: string, password: string) => {
    const response = await authApiClient.post<{ success: boolean; token: string; user: AuthUser }>('/login', {
      username,
      password,
    });
    return response;
  },

  me: async () => {
    const response = await authApiClient.get<{ success: boolean; user: AuthUser }>('/me');
    return response;
  },

  logoutAdmin: async () => {
    // For admin, just clear local storage (no server logout needed)
    return Promise.resolve();
  },

  logoutOIDC: async () => {
    const response = await authApiClient.post('/oidc/logout');
    return response;
  },

  loginOIDC: () => {
    window.location.href = '/auth/oidc/login';
  },
};

// Health API
export const healthApi = {
  getVersion: async (): Promise<string> => {
    const response = await api.get('/v1/health');
    return response.data.data.version as string;
  },
};

// Capabilities API
export const capabilitiesApi = {
  get: async (): Promise<GarageCapabilities> => {
    const response = await api.get('/v1/capabilities');
    return response.data.data;
  },
};

// Bucket API
export const bucketsApi = {
  list: async (): Promise<Bucket[]> => {
    const response = await api.get('/v1/buckets');
    return response.data.data.buckets || [];
  },

  get: async (name: string): Promise<BucketDetails> => {
    const response = await api.get(`/v1/buckets/${name}`);
    return response.data.data;
  },

  create: async (bucketName: string, bucketRegion?: string): Promise<void> => {
    await api.post('/v1/buckets', { name: bucketName, region: bucketRegion });
  },

  delete: async (name: string): Promise<void> => {
    await api.delete(`/v1/buckets/${name}`);
  },

  grantPermission: async (
    bucketName: string,
    accessKeyId: string,
    permissions: { read: boolean; write: boolean; owner: boolean }
  ): Promise<void> => {
    await api.post(`/v1/buckets/${bucketName}/permissions`, {
      accessKeyId,
      permissions,
    });
  },

  updateSettings: async (name: string, settings: Partial<BucketDetails>): Promise<void> => {
    await api.patch(`/v1/buckets/${name}/settings`, settings);
  },

  updateBucketWebsite: async (
    name: string,
    payload: { enabled: boolean; indexDocument?: string; errorDocument?: string }
  ) => {
    const response = await api.put<ApiResponse<any>>(
      `/v1/buckets/${encodeURIComponent(name)}/website`,
      payload
    );
    return response.data.data;
  },
};

// Objects API
export const objectsApi = {
  list: async (bucket: string, prefix?: string, maxKeys?: number, continuationToken?: string): Promise<ObjectListResponse> => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const params: any = {};
    if (prefix) params.prefix = prefix;
    if (maxKeys) params.max_keys = maxKeys;
    if (continuationToken) params.continuation_token = continuationToken;

    const response = await api.get(`/v1/buckets/${bucket}/objects`, { params });
    const data = response.data.data;

    // Combine objects and prefixes (folders)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const objects: S3Object[] = data.objects?.map((obj: any) => ({
      key: obj.key,
      size: obj.size,
      lastModified: obj.last_modified,
      etag: obj.etag,
      contentType: obj.content_type,
      storageClass: obj.storage_class,
      isFolder: false,
    })) || [];

    const folders: S3Object[] = data.prefixes?.map((prefix: string) => ({
      key: prefix,
      size: 0,
      lastModified: null,
      isFolder: true,
    })) || [];

    return {
      bucket: data.bucket,
      objects: [...folders, ...objects],
      prefixes: data.prefixes || [],
      count: data.count,
      isTruncated: data.is_truncated || false,
      nextContinuationToken: data.next_continuation_token,
    };
  },

  get: async (bucket: string, key: string): Promise<Blob> => {
    const response = await api.get(`/v1/buckets/${bucket}/objects/${encodeObjectKey(key)}`, {
      responseType: 'blob'
    });
    return response.data;
  },

  getMetadata: async (bucket: string, key: string): Promise<ObjectMetadata> => {
    const response = await api.get(`/v1/buckets/${bucket}/objects/${encodeObjectKey(key)}/metadata`);
    const data = response.data.data;
    return {
      key: data.key,
      size: data.size,
      lastModified: data.last_modified,
      contentType: data.content_type,
      etag: data.etag,
      storageClass: data.storage_class,
      metadata: data.metadata,
    };
  },

  createDirectory: async (bucket: string, key: string): Promise<void> => {
    await api.post(`/v1/buckets/${bucket}/directories`, { key });
  },

  upload: async (bucket: string, key: string, file: File, onProgress?: (progress: number) => void): Promise<void> => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('key', key);
    await api.post(`/v1/buckets/${bucket}/objects`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          onProgress(progress);
        }
      },
    });
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  uploadMultiple: async (bucket: string, files: File[]): Promise<any> => {
    const formData = new FormData();
    files.forEach(file => {
      formData.append('files', file);
    });
    const response = await api.post(`/v1/buckets/${bucket}/objects/upload-multiple`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data.data;
  },

  delete: async (bucket: string, key: string): Promise<void> => {
    await api.delete(`/v1/buckets/${bucket}/objects/${encodeObjectKey(key)}`);
  },

  deleteMultiple: async (bucket: string, keys: string[], prefix?: string): Promise<void> => {
    const payload = { keys, ...(prefix && { prefix }) };
    await api.post(`/v1/buckets/${bucket}/objects/delete-multiple`, payload);
  },

  getPresignedUrl: async (bucket: string, key: string, expiresIn: number = 3600): Promise<string> => {
    const response = await api.get(`/v1/buckets/${bucket}/objects/${encodeObjectKey(key)}/presign`, {
      params: { expires_in: expiresIn }
    });
    return response.data.data.url;
  },
};

// Access Control API (Users/Keys)
export const accessApi = {
  listKeys: async (): Promise<AccessKey[]> => {
    const response = await api.get('/v1/users');
    return response.data.data.users || [];
  },

  getKey: async (accessKey: string): Promise<AccessKey> => {
    const response = await api.get(`/v1/users/${accessKey}`);
    return response.data.data;
  },

  getSecretKey: async (accessKey: string): Promise<string> => {
    const response = await api.get(`/v1/users/${accessKey}/secret`);
    return response.data.data.secretKey;
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  createKey: async (name: string, permissions?: any[]): Promise<AccessKey> => {
    const response = await api.post('/v1/users', { name, permissions });
    return response.data.data;
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  updateKey: async (accessKey: string, updates: any): Promise<void> => {
    await api.patch(`/v1/users/${accessKey}`, updates);
  },

  deleteKey: async (accessKey: string): Promise<void> => {
    await api.delete(`/v1/users/${accessKey}`);
  },
};

// Analytics API
export const analyticsApi = {
  getMetrics: async (): Promise<StorageMetrics> => {
    const response = await api.get('/v1/monitoring/dashboard');
    return response.data.data;
  },
};

// Garage Cluster & Monitoring API
export const garageApi = {
  getClusterHealth: async (): Promise<ClusterHealth> => {
    const response = await api.get('/v1/cluster/health');
    return response.data.data;
  },

  getClusterStatus: async (): Promise<ClusterStatus> => {
    const response = await api.get('/v1/cluster/status');
    return response.data.data;
  },

  getClusterStatistics: async (): Promise<ClusterStatistics> => {
    const response = await api.get('/v1/cluster/statistics');
    return response.data.data;
  },

  getNodeInfo: async (nodeId: string = 'self'): Promise<MultiNodeResponse> => {
    const response = await api.get(`/v1/cluster/nodes/${nodeId}`);
    return response.data.data;
  },

  getNodeStatistics: async (nodeId: string): Promise<MultiNodeStatisticsResponse> => {
    const response = await api.get(`/v1/cluster/nodes/${nodeId}/statistics`);
    return response.data.data;
  },

  getFullMetrics: async (): Promise<GarageMetrics> => {
    // Fetch all cluster-related metrics
    const [health, statistics, storageMetrics] = await Promise.all([
      garageApi.getClusterHealth(),
      garageApi.getClusterStatistics(),
      analyticsApi.getMetrics(),
    ]);

    return {
      ...storageMetrics,
      clusterHealth: health,
      clusterStatistics: statistics,
    };
  },
};

// Monitoring API
export const monitoringApi = {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  getMetrics: async (): Promise<any> => {
    const response = await api.get('/v1/monitoring/metrics');
    return response.data.data;
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  checkAdminHealth: async (): Promise<any> => {
    const response = await api.get('/v1/monitoring/admin-health');
    return response.data.data;
  },
};

export default api;
