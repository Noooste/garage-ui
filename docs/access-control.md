# Multi-User Access Control

Garage UI can limit what each user sees and does, based on the teams in their OIDC claims.

It's optional. With no config, every authenticated user has full access, exactly like before.

## Not a security boundary

Read this before using access control for anything sensitive.

Garage UI talks to Garage with one admin token and one set of S3 keys. Access control lives in the UI only; Garage itself does not enforce it. Anyone holding the underlying admin token or raw S3 keys bypasses it completely.

Use it to give teams a convenient, scoped UI. Don't use it as a replacement for real per-tenant credentials or network isolation.

## Configuration

Two settings drive access control:

1. `team_attribute_path`: the OIDC claim that lists a user's teams.
2. `access_control`: maps teams to permissions.

```yaml
auth:
  oidc:
    # Existing keys unchanged. New:
    team_attribute_path: "groups"   # go-jmespath, same convention as role_attribute_path

access_control:                     # absent = full access for everyone; present = default-deny
  presets:
    bucket_readonly: [bucket.list, bucket.read, object.list, object.read]
    bucket_owner: ["preset:bucket_readonly", bucket.create, bucket.update,
                   bucket.delete, object.write, object.delete]
  teams:
    - name: backend
      claim_values: ["garage-team-backend"]   # matched against the team_attribute_path claim
      bindings:
        - bucket_prefixes: ["backend-"]
          permissions: ["preset:bucket_owner"]
        - bucket_prefixes: ["shared-"]
          permissions: ["preset:bucket_readonly"]
      cluster_permissions: [cluster.status, cluster.health]
```

A few things to know:

- `team_attribute_path` is a [go-jmespath](https://github.com/jmespath/go-jmespath) expression evaluated against the OIDC claims, the same way `role_attribute_path` works. It's required when `access_control.teams` is set and OIDC is on. If it's missing, startup fails with a clear error.
- `access_control` can only be set in the config file. There's no environment variable for it, because nested team and binding lists don't fit flat `GARAGE_UI_*` variables.
- If `access_control` is present but OIDC is off, the server still starts but logs a warning. Without OIDC users the policy gates nothing, since admin-password and token logins are always full admin (see [Admin model](#admin-model)).

## How it works

### Default-deny

With `access_control` set, an OIDC user who matches no team gets a 403 on every `/api/v1` endpoint. The one exception is `GET /api/v1/capabilities`, which returns their (empty) permissions so the frontend can show a "no access" screen.

### Union of teams

A user who matches several teams gets everything those teams grant.

Bindings stay separate, though. Say one binding grants `read` on `backend-*` and another grants `write` on `data-*`. The user does not end up with both permissions on both prefixes. Each binding keeps its own prefixes and its own permissions.

### Prefix match

`bucket_prefixes` are plain string prefixes on bucket names (no globbing on the name itself). Use `"*"` to match every bucket.

### Presets

Reference a preset with the `preset:` prefix inside any `permissions` or `cluster_permissions` list, for example `"preset:bucket_owner"`. Presets can reference other presets. Unknown references and cycles both fail startup.

### Permission globs

A trailing-star glob like `bucket.*`, `object.*`, or `cluster.layout.*` expands against the permission vocabulary when config loads. Use scoped globs:

- `bucket.*`, `object.*` inside a binding's `permissions`
- `cluster.*`, `node.*`, `worker.*`, `block.*` under `cluster_permissions`

A bare `*` is technically a glob, but it almost always fails validation: it mixes prefix-scoped and global-scoped permissions, and a permission placed in the wrong scope is rejected at startup. Globs never include admin-only permissions, and in v1 there's no team-level way to grant those.

### Admin model

These identities become a synthetic admin:

- OIDC users with a configured `admin_role` / `admin_roles`
- all non-OIDC logins (admin-password, Garage admin token)

An admin gets every permission on every bucket, plus every cluster permission. Admins run through the same authorizer as any team; there's no `IsAdmin` shortcut that skips the check.

### Startup validation

The server refuses to start when the policy is invalid: an unknown permission, an unknown or cyclic preset, an admin-only permission granted to a team, a duplicate team name, a team with empty `claim_values`, or a team with no bindings and no cluster permissions.

It also refuses to start if any `/api/v1` route has no declared permission, so a route can never ship un-gated (see [Troubleshooting](#troubleshooting)).

## Not in v1

- **No non-OIDC team mapping.** Admin-password and Garage-admin-token logins are always full admin. Only OIDC users can be scoped to a team.
- **`ListKeys` is not filtered.** Anyone with `key.list` sees every access key. Everything past `key.list` / `key.read` is admin-only (`key.read_secret`, `key.create`, `key.import`, `key.update`, `key.delete`).
- **No `admin_token.*` permissions.** Direct access to the raw Garage admin token is admin-only and not part of the vocabulary.
- **No ABAC, policy language, database-backed policy, or per-user grants.** Policy is YAML, compiled once at startup.

## Permission vocabulary (v1)

Permission names are lowercase and dot-separated: two segments, or three for `cluster.layout.*`. The source of truth is `backend/internal/authz/vocabulary.go`. This table mirrors it by hand, and there's no doc generation in v1, so update the table whenever you change the registry.

| Permission | Scope | Admin-only v1 | Garage endpoint / backing |
|---|---|---|---|
| `bucket.list` | prefix | | ListBuckets (response-filtered) |
| `bucket.read` | prefix | | GetBucketInfo |
| `bucket.create` | prefix | | CreateBucket (new name must match a prefix) |
| `bucket.update` | prefix | | UpdateBucket |
| `bucket.delete` | prefix | | DeleteBucket |
| `bucket.cleanup_uploads` | prefix | | CleanupIncompleteUploads |
| `bucket.inspect_object` | prefix | | InspectObject |
| `bucket_alias.add` | prefix | | AddBucketAlias |
| `bucket_alias.remove` | prefix | | RemoveBucketAlias |
| `object.list` | prefix | | S3 data plane (ListObjects) |
| `object.read` | prefix | | S3 data plane (Get/Head/Metadata/Presign; presign is download-only) |
| `object.write` | prefix | | S3 data plane (Upload, CreateDirectory) |
| `object.delete` | prefix | | S3 data plane (Delete, DeleteMultiple) |
| `permission.allow_bucket_key` | prefix | | AllowBucketKey |
| `permission.deny_bucket_key` | prefix | | DenyBucketKey |
| `key.list` | global | | ListKeys (unfiltered in v1; grantee sees all keys) |
| `key.read` | global | | GetKeyInfo (without secret) |
| `key.read_secret` | global | yes | GetKeyInfo with secret material |
| `key.create` | global | yes | CreateKey |
| `key.import` | global | yes | ImportKey |
| `key.update` | global | yes | UpdateKey |
| `key.delete` | global | yes | DeleteKey |
| `cluster.status` | global | | GetClusterStatus |
| `cluster.health` | global | | GetClusterHealth |
| `cluster.statistics` | global | | GetClusterStatistics |
| `cluster.connect_nodes` | global | | ConnectClusterNodes |
| `cluster.layout.read` | global | | GetClusterLayout |
| `cluster.layout.history` | global | | GetClusterLayoutHistory |
| `cluster.layout.apply` | global | | ApplyClusterLayout |
| `cluster.layout.skip_dead_nodes` | global | | ClusterLayoutSkipDeadNodes |
| `node.info` | global | | GetNodeInfo |
| `node.statistics` | global | | GetNodeStatistics |
| `node.snapshot` | global | | CreateMetadataSnapshot |
| `node.repair` | global | | LaunchRepairOperation |
| `worker.list` | global | | ListWorkers |
| `worker.info` | global | | GetWorkerInfo |
| `worker.get_variable` | global | | GetWorkerVariable |
| `worker.set_variable` | global | | SetWorkerVariable |
| `block.list_errors` | global | | ListBlockErrors |
| `block.info` | global | | GetBlockInfo |

Some permissions have no UI route yet: `bucket.cleanup_uploads`, `bucket.inspect_object`, `bucket_alias.*`, `cluster.layout.*`, `worker.*`, `block.*`, `cluster.connect_nodes`, `node.snapshot`, `node.repair`, and `key.import`. They're valid in config but don't gate anything in the UI yet. The vocabulary is complete up front so your config keeps working as the UI grows.

Dangerous operations (`cluster.layout.apply`, `node.repair`, `worker.set_variable`) are separate, individually grantable permissions. They're never bundled into a read-only preset, so you can give a team cluster visibility without also giving it the power to break the cluster.

`POST /api/v1/buckets/:name/permissions` sets permissions by doing an allow and a deny in one call, so it needs **both** `permission.allow_bucket_key` and `permission.deny_bucket_key`. One of the two alone is not enough.

## Troubleshooting

**A user gets 403s they shouldn't.** Open `GET /api/v1/capabilities` while logged in as that user. The `access_control` block shows their resolved `bindings` and `cluster_permissions` (empty arrays mean they matched no team). Check that the IdP actually sends the claim named by `team_attribute_path`, and that its values match a team's `claim_values` exactly (string match, no wildcards on the claim value).

**403 responses name the missing permission.** The message is `Missing permission: <permission.name>`. That's the exact permission that was denied, so you know which binding, preset, or `cluster_permissions` entry to add.

**Decision logs.** Every check logs one line, `authz_decision`, with fields `subject`, `action`, `resource`, `decision` (`allow` / `deny`), and `reason` (such as `binding_match`, `any_binding`, `no_matching_binding`, `cluster_permission`, `no_cluster_permission`, `no_subject`). Denials log at `warn`, allows at `debug`. Set `logging.level` to `debug` to see successful checks too. There's no separate audit log in v1; this goes through the normal application logger.

**Startup fails with `access_control: ...` or `authz: routes without Require permission declaration: ...`.** Both are intentional fail-closed checks, not bugs:

- An invalid policy (unknown permission, bad preset reference, admin-only permission handed to a team, duplicate team name, empty `claim_values`, or a team with no bindings or cluster permissions) stops startup with an error naming the problem.
- A `/api/v1` route wired without a permission requirement also stops startup. This is a build-time safety net, not something you trigger by editing config, but it can show up after a `git pull` that adds a route without its enforcement wiring.

## See also

- [config.example.yaml](../config.example.yaml): the full commented `access_control` example.
- [garage-setup.md](garage-setup.md): general Garage UI and Garage setup.
