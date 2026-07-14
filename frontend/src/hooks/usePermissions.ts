import { useCapabilities } from './useCapabilities';

/**
 * Permission view derived from /api/v1/capabilities.
 *
 * Fail-closed: while capabilities are loading (or errored), every check
 * returns false. Gated UI stays hidden until the server has spoken.
 * When access control is disabled server-side, every check returns true.
 */
export function usePermissions() {
  const { data, isLoading, isError } = useCapabilities();
  const ac = data?.access_control;
  const settled = !isLoading && !isError && data !== undefined;
  const enabled = ac?.enabled ?? false;
  const isAdmin = settled && (!enabled || (ac?.is_admin ?? false));
  const bindings = ac?.bindings ?? [];
  const clusterPerms = ac?.cluster_permissions ?? [];

  const hasClusterPerm = (perm: string): boolean => {
    if (!settled) return false;
    if (!enabled || isAdmin) return true;
    return clusterPerms.includes(perm);
  };

  const hasAnyPerm = (perm: string): boolean => {
    if (!settled) return false;
    if (!enabled || isAdmin) return true;
    return bindings.some((b) => b.permissions.includes(perm));
  };

  const hasAnyClusterAccess =
    settled &&
    (!enabled || isAdmin || clusterPerms.some((p) => p.startsWith('cluster.') || p.startsWith('node.')));

  const noAccess =
    settled && enabled && !isAdmin && bindings.length === 0 && clusterPerms.length === 0;

  return { loading: !settled, enabled, isAdmin, hasClusterPerm, hasAnyPerm, hasAnyClusterAccess, noAccess };
}

/**
 * Per-bucket check against the server-computed effective_permissions carried
 * on bucket payloads. Does not know about loading/enabled state, so it treats
 * a missing effective_permissions field as "access control is disabled" and
 * allows. That's only correct when access control really is disabled, so
 * this is kept for internal use (by useBucketCan below); UI code should call
 * useBucketCan() instead, which fails closed when access control is enabled.
 */
export function bucketCan(
  bucket: { effective_permissions?: string[] } | undefined,
  perm: string,
): boolean {
  const perms = bucket?.effective_permissions;
  if (!perms) return true;
  return perms.includes(perm);
}

/**
 * Hook returning a per-bucket permission check closed over the current
 * loading/enabled state from usePermissions().
 *
 * Fail-closed: while capabilities are loading, every check denies. When
 * access control is enabled, a bucket without effective_permissions (not
 * loaded yet, or filtered out of the bucket list) also denies instead of
 * falling open like bucketCan does. When access control is disabled, every
 * check allows, same as bucketCan.
 */
export function useBucketCan() {
  const { loading, enabled } = usePermissions();
  return (bucket: { effective_permissions?: string[] } | undefined, perm: string): boolean => {
    if (loading) return false;
    if (!enabled) return true;
    if (!bucket?.effective_permissions) return false;
    return bucketCan(bucket, perm);
  };
}
