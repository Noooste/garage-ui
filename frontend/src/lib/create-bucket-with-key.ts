import type { AccessKey } from '@/types';

export interface KeyPermissions {
  read: boolean;
  write: boolean;
  owner: boolean;
}

export interface NewKeyRequest {
  name: string;
  permissions: KeyPermissions;
}

export interface CreateBucketResult {
  bucket: 'ok' | 'failed';
  key?: { name: string; accessKeyId: string; secretKey: string };
  keyError?: string;
  grantError?: string;
}

export interface CreateBucketDeps {
  createBucket: (name: string) => Promise<void>;
  createKey: (name: string) => Promise<AccessKey>;
  grant: (bucket: string, accessKeyId: string, permissions: KeyPermissions) => Promise<void>;
}

const message = (e: unknown): string => (e instanceof Error ? e.message : String(e));

/**
 * Bucket, then key, then grant, against three separate endpoints. There is no
 * rollback: whatever succeeded stays and the caller renders the partial
 * outcome. A failed grant still reports the key because the API returns the
 * secret exactly once.
 */
export async function createBucketWithKey(
  deps: CreateBucketDeps,
  bucketName: string,
  key?: NewKeyRequest,
): Promise<CreateBucketResult> {
  try {
    await deps.createBucket(bucketName);
  } catch {
    return { bucket: 'failed' };
  }

  if (!key) {
    return { bucket: 'ok' };
  }

  let created: AccessKey;
  try {
    created = await deps.createKey(key.name);
  } catch (e) {
    return { bucket: 'ok', keyError: message(e) };
  }

  const result: CreateBucketResult = {
    bucket: 'ok',
    key: {
      name: created.name || key.name,
      accessKeyId: created.accessKeyId,
      secretKey: created.secretKey ?? '',
    },
  };

  try {
    await deps.grant(bucketName, created.accessKeyId, key.permissions);
  } catch (e) {
    result.grantError = message(e);
  }

  return result;
}
