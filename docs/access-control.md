# Multi-User Access Control

Garage UI can scope what each user sees and does, based on the teams in their OIDC claims. A typical setup: the backend team manages every `backend-*` bucket, can read the shared ones, and sees nothing else.

This is optional. If your config has no `access_control` section, every authenticated user has full access.

## Not a security boundary

Read this before relying on access control for anything sensitive.

Garage UI talks to Garage with a single admin token and a single set of S3 keys. Access control is enforced by the UI, not by Garage. Anyone who holds the underlying admin token or raw S3 keys bypasses it completely.

Use it to give each team a convenient, scoped view of the cluster. Don't use it as a substitute for real per-tenant credentials or network isolation.

## Setup

Get OIDC login working first. Access control only applies to OIDC users, and [config.example.yaml](../config.example.yaml) covers the OIDC settings.

From there, it takes two additions to your config:

1. `team_attribute_path` tells Garage UI which OIDC claim lists a user's teams.
2. `access_control` maps those teams to permissions.

```yaml
auth:
  oidc:
    # ...your existing OIDC settings...
    team_attribute_path: "groups"

access_control:
  presets:
    bucket_readonly: [bucket.list, bucket.read, object.list, object.read]
    bucket_owner: ["preset:bucket_readonly", bucket.create, bucket.update,
                   bucket.delete, object.write, object.delete]
  teams:
    - name: backend
      claim_values: ["garage-team-backend"]
      bindings:
        - bucket_prefixes: ["backend-"]
          permissions: ["preset:bucket_owner"]
        - bucket_prefixes: ["shared-"]
          permissions: ["preset:bucket_readonly"]
      cluster_permissions: [cluster.status, cluster.health]
```

Reading the example top to bottom:

- `presets` are named permission bundles you can reuse across teams. They're optional; you can always list permissions directly.
- A user whose `groups` claim contains `garage-team-backend` lands in the `backend` team.
- That team owns buckets starting with `backend-`, can read buckets starting with `shared-`, and can view cluster status and health.
- Everyone else, including OIDC users who match no team, gets a 403 on everything. The moment `access_control` exists, the UI switches to default-deny.

A few things to know before writing your own:

- `team_attribute_path` is a [go-jmespath](https://github.com/jmespath/go-jmespath) expression evaluated against the OIDC claims, the same convention as `role_attribute_path`. It's required whenever `access_control.teams` is set and OIDC is enabled. If it's missing, the server refuses to start and the error says why.
- `access_control` can only be set in the config file. There's no environment variable for it, because nested team and binding lists don't fit flat `GARAGE_UI_*` variables.
- If `access_control` is present but OIDC is disabled, the server still starts but logs a warning. The policy would gate nothing, since admin-password and token logins are always full admin (see [Admins](#admins)).

### Check that it works

Log in as a test user and open `GET /api/v1/capabilities`. The `access_control` block in the response shows the user's resolved `bindings` and `cluster_permissions`. Empty arrays mean the user matched no team; [Troubleshooting](#troubleshooting) covers the usual reasons.

## How permissions are resolved

### Default-deny

Once `access_control` is set, an OIDC user who matches no team gets a 403 on every `/api/v1` endpoint. The one exception is `GET /api/v1/capabilities`, which returns their (empty) permissions so the frontend can show a "no access" screen.

### Union of teams

A user who matches several teams gets everything those teams grant.

Bindings stay separate, though. Say one binding grants `read` on `backend-*` and another grants `write` on `data-*`. The user does not end up with both permissions on both prefixes. Each binding keeps its own prefixes and its own permissions.

### Prefix match

`bucket_prefixes` are plain string prefixes on bucket names, with no globbing on the name itself. Use `"*"` to match every bucket.

### Presets

Reference a preset with the `preset:` prefix inside any `permissions` or `cluster_permissions` list, for example `"preset:bucket_owner"`. Presets can reference other presets. Unknown references and cycles both fail startup.

### Permission globs

A trailing-star glob like `bucket.*`, `object.*`, or `cluster.layout.*` expands against the permission vocabulary when the config loads. Keep globs inside their scope:

- `bucket.*` and `object.*` go in a binding's `permissions`
- `cluster.*`, `node.*`, `worker.*`, and `block.*` go under `cluster_permissions`

A bare `*` is technically a glob, but it almost always fails validation: it mixes prefix-scoped and global-scoped permissions, and a permission placed in the wrong scope is rejected at startup. Globs never expand to admin-only permissions, and there's no way to grant those to a team.

### Admins

These identities become a synthetic admin:

- OIDC users holding a configured `admin_role` / `admin_roles`
- all non-OIDC logins (admin password, Garage admin token)

An admin gets every permission on every bucket, plus every cluster permission. Admins go through the same authorizer as any team; there's no `IsAdmin` shortcut that skips the check.

### Startup validation

The server refuses to start when the policy is invalid: an unknown permission, an unknown or cyclic preset, an admin-only permission granted to a team, a duplicate team name, a team with empty `claim_values`, or a team with no bindings and no cluster permissions.

It also refuses to start if any `/api/v1` route has no declared permission, so a route can never ship un-gated (see [Troubleshooting](#troubleshooting)).

## Current limitations

- **Only OIDC users can be scoped to a team.** Admin-password and Garage-admin-token logins are always full admin.
- **`ListKeys` is not filtered.** Anyone with `key.list` sees every access key. Everything past `key.list` and `key.read` is admin-only: `key.read_secret`, `key.create`, `key.import`, `key.update`, `key.delete`.
- **No `admin_token.*` permissions.** Direct access to the raw Garage admin token is admin-only and not part of the vocabulary.
- **No ABAC, policy language, database-backed policy, or per-user grants.** Policy is YAML, compiled once at startup.

## Permission reference

Permission names are lowercase and dot-separated: two segments, or three for `cluster.layout.*`. The source of truth is `backend/internal/authz/vocabulary.go`; this table is maintained by hand, so update it whenever the registry changes.

| Permission | Scope | Admin-only | Garage endpoint / backing |
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
| `key.list` | global | | ListKeys (unfiltered; grantee sees all keys) |
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

Some permissions have no UI route yet: `bucket.cleanup_uploads`, `bucket.inspect_object`, `bucket_alias.*`, `cluster.layout.*`, `worker.*`, `block.*`, `cluster.connect_nodes`, `node.snapshot`, `node.repair`, and `key.import`. They're valid in config but don't gate anything in the UI so far. The vocabulary is complete up front so your config keeps working as the UI grows.

Dangerous operations (`cluster.layout.apply`, `node.repair`, `worker.set_variable`) are separate, individually grantable permissions. They're never bundled into a read-only preset, so you can give a team cluster visibility without also giving it the power to break the cluster.

One endpoint needs two permissions: `POST /api/v1/buckets/:name/permissions` performs an allow and a deny in a single call, so it requires both `permission.allow_bucket_key` and `permission.deny_bucket_key`. One of the two alone is not enough.

## Troubleshooting

- **A user gets 403s they shouldn't.** Open `GET /api/v1/capabilities` while logged in as that user. The `access_control` block shows their resolved `bindings` and `cluster_permissions` (empty arrays mean they matched no team). Check that the IdP actually sends the claim named by `team_attribute_path`, and that its values match a team's `claim_values` exactly. It's a plain string match, with no wildcards on the claim value.

- **403 responses name the missing permission.** The message is `Missing permission: <permission.name>`. That's the exact permission that was denied, so you know which binding, preset, or `cluster_permissions` entry to add.

- **Decision logs.** Every check logs one line, `authz_decision`, with fields `subject`, `action`, `resource`, `decision` (`allow` / `deny`), and `reason` (such as `binding_match`, `any_binding`, `no_matching_binding`, `cluster_permission`, `no_cluster_permission`, `no_subject`). Denials log at `warn`, allows at `debug`. Set `logging.level` to `debug` to see successful checks too. There's no separate audit log; this goes through the normal application logger.

- **Startup fails with `access_control: ...` or `authz: routes without Require permission declaration: ...`.** Both are intentional fail-closed checks, not bugs:
  - An invalid policy (unknown permission, bad preset reference, admin-only permission handed to a team, duplicate team name, empty `claim_values`, or a team with no bindings or cluster permissions) stops startup with an error naming the problem.
  - A `/api/v1` route wired without a permission requirement also stops startup. This is a safety net for developers, not something you trigger by editing config, but it can show up after a `git pull` that adds a route without its enforcement wiring.

## See also

- [config.example.yaml](../config.example.yaml): the full commented `access_control` example.
- [garage-setup.md](garage-setup.md): general Garage UI and Garage setup.
