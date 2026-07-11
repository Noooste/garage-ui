# Multi-User Access Control

Garage UI can restrict what authenticated users can see and do, based on teams resolved from
OIDC claims. This is optional: without configuration, garage-ui behaves exactly as before —
every authenticated user has full access.

## Not a security boundary

**Read this before you rely on access control for anything sensitive.** Garage UI talks to Garage
with a single admin API token and a single S3 credential set. Access control here is **UI-layer
policy**, not enforced by Garage itself: anyone holding the underlying Garage admin token or raw
S3 keys bypasses it entirely. Use it to give teams a scoped, convenient UI — not as a substitute
for real per-tenant credentials or network isolation.

## Configuration

Access control is driven by two settings: an OIDC claim path that carries team membership, and
an `access_control` section that maps teams to permissions.

```yaml
auth:
  oidc:
    # Existing keys unchanged. New:
    team_attribute_path: "groups"   # go-jmespath, same convention as role_attribute_path

access_control:                     # absent = today's behavior; present = default-deny
  presets:
    bucket_readonly: [bucket.list, bucket.read, object.list, object.read]
    bucket_owner: ["preset:bucket_readonly", bucket.create, bucket.update,
                   bucket.delete, object.write, object.delete]
  teams:
    - name: backend
      claim_values: ["garage-team-backend"]   # matched against team_attribute_path claim
      bindings:
        - bucket_prefixes: ["backend-"]
          permissions: ["preset:bucket_owner"]
        - bucket_prefixes: ["shared-"]
          permissions: ["preset:bucket_readonly"]
      cluster_permissions: [cluster.status, cluster.health]
```

- `team_attribute_path` is a [go-jmespath](https://github.com/jmespath/go-jmespath) expression
  evaluated against the ID token / userinfo claims, same convention as the existing
  `role_attribute_path`. It's required when `access_control.teams` is set and OIDC is enabled;
  startup fails with a clear error otherwise.
- `access_control` is config-file only — there is no environment-variable binding for it (nested
  lists of teams/bindings don't map onto flat `GARAGE_UI_*` env vars).
- If `access_control` is present but OIDC is disabled, the server still starts, but logs a
  prominent warning: with no OIDC users, the policy currently gates nothing (admin-password and
  token logins are always full-admin — see below).

## Semantics

- **Default-deny.** With `access_control` present, an authenticated OIDC user who matches no
  team gets a uniform 403 on every `/api/v1` endpoint except `GET /api/v1/capabilities`, which
  returns their (empty) permissions so the frontend can render a "no access" screen. There is no
  partial/health floor for unmatched users.
- **Union.** A user matching multiple teams (or multiple teams that share a `claim_values` entry)
  gets the union of all their bindings and cluster permissions. Bindings are never flattened
  together: `read` granted on `backend-*` plus `write` granted on `data-*` does not become both
  permissions on both prefixes — each binding keeps its own prefix list and permission set.
- **Prefix match.** `bucket_prefixes` are exact string prefixes on bucket names (no globbing on
  the bucket name itself). `"*"` as a prefix matches every bucket.
- **Presets.** Referenced with a `preset:` prefix inside any `permissions` or
  `cluster_permissions` list (e.g. `"preset:bucket_owner"`). Presets may reference other presets;
  unknown references and reference cycles both fail startup.
- **Permission globs.** Trailing-star globs (e.g. `bucket.*`, `object.*`, `cluster.layout.*`) are
  expanded against the permission vocabulary at config-load time. A bare `*` is also accepted as a
  glob, but in practice it will almost always fail validation: it expands across every scope
  (prefix- and global-scoped permissions together), and a prefix-scoped permission placed under
  `cluster_permissions` (or vice versa) is rejected at startup. **Use scoped globs like
  `bucket.*` or `object.*` inside a binding's `permissions`, and `cluster.*` / `node.*` /
  `worker.*` / `block.*` style globs under `cluster_permissions`** — this is the supported
  pattern for teams. Globs never expand to admin-only permissions (see below); if you need one of
  those, there is no team-level way to get it in v1.
- **Admin model.** Users holding one of the configured `admin_role` / `admin_roles` OIDC roles,
  and all non-OIDC identities in v1 (admin-password logins, Garage-admin-token logins), resolve
  to a synthetic admin subject: one binding granting every prefix-scoped permission (including
  admin-only ones) on every bucket (`bucket_prefixes: ["*"]`), plus every global/cluster
  permission. Admin enforcement runs through the exact same authorizer code path as any team —
  there is no `IsAdmin` shortcut that skips a check.
- **Startup validation.** The server refuses to start (with a specific error message) on: an
  unknown permission name, an unknown or cyclic preset reference, an admin-only permission granted
  to a team (directly or via preset), a duplicate team name, a team with empty `claim_values`, or
  a team with neither `bindings` nor `cluster_permissions`. It also refuses to start if any
  `/api/v1` route is wired without a declared permission requirement (see Troubleshooting) —
  a route can never accidentally ship un-gated.

## Deferred to a future version

- **Non-OIDC identity to team mapping is not implemented.** Admin-password and Garage-admin-token
  logins always resolve to the synthetic admin subject, regardless of `access_control`. Only OIDC
  identities can be scoped to a team.
- **`ListKeys` is not filtered by team.** Any subject holding `key.list` sees every access key,
  not just their team's. Key management beyond `key.list` / `key.read` (`key.read_secret`,
  `key.create`, `key.import`, `key.update`, `key.delete`) is admin-only and cannot be granted to
  a team in v1.
- **`admin_token.*`-style permissions do not exist.** Direct exposure of the raw Garage admin
  token is implicitly admin-only and outside the vocabulary entirely.
- **No ABAC, policy language, database-backed policy CRUD, or per-user (non-team) grants.**
  Policy is config-file YAML, compiled once at startup.

## Permission vocabulary (v1)

Permission names are two segments (three for `cluster.layout.*`), lowercase, dot-separated. The
registry lives in `backend/internal/authz/vocabulary.go` — it is the only place Garage endpoint
names appear; this table is a hand-maintained mirror of it (there is no doc-generation step in
v1, so if you change the registry, update this table too).

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
| `object.read` | prefix | | S3 data plane (Get/Head/Metadata/Presign — presign is download-only) |
| `object.write` | prefix | | S3 data plane (Upload, CreateDirectory) |
| `object.delete` | prefix | | S3 data plane (Delete, DeleteMultiple) |
| `permission.allow_bucket_key` | prefix | | AllowBucketKey |
| `permission.deny_bucket_key` | prefix | | DenyBucketKey |
| `key.list` | global | | ListKeys (unfiltered in v1 — grantee sees all keys) |
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

Permissions with no UI route today (`bucket.cleanup_uploads`, `bucket.inspect_object`,
`bucket_alias.*`, `cluster.layout.*`, `worker.*`, `block.*`, `cluster.connect_nodes`,
`node.snapshot`, `node.repair`, `key.import`) are still valid in config and gate nothing yet in the UI — the
vocabulary is exhaustive up front so an operator's config survives future UI growth without
changes. Dangerous operations (`cluster.layout.apply`, `node.repair`, `worker.set_variable`) are
deliberately kept as separate, individually-grantable permissions — never bundled into a
read-oriented preset — so a team can be handed cluster visibility without also being handed the
ability to break the cluster.

`POST /api/v1/buckets/:name/permissions` is a "set permissions" endpoint that performs both an
allow and a deny operation against Garage in one call, so it requires **both**
`permission.allow_bucket_key` and `permission.deny_bucket_key` — granting only one of the two is
not sufficient to use it.

## Troubleshooting

**A user gets 403s they shouldn't.** Check `GET /api/v1/capabilities` while logged in as that
user: the `access_control` block reports their resolved `bindings` and `cluster_permissions`
(empty arrays mean they matched no team). Confirm the IdP is actually sending the claim named by
`team_attribute_path`, and that its values line up with a team's `claim_values` exactly (string
match, no wildcards on the claim value itself).

**403 responses name the missing permission.** The error message is
`Missing permission: <permission.name>` — this is the specific permission that was checked and
denied for the request, so you know exactly which binding/preset/cluster_permissions entry to add.

**Decision logs.** Every authorization check emits one structured log line,
`authz_decision`, with fields `subject`, `action`, `resource`, `decision` (`allow`/`deny`), and
`reason` (e.g. `binding_match`, `any_binding`, `no_matching_binding`, `cluster_permission`,
`no_cluster_permission`, `no_subject`). Denials are logged at `warn`; allows at `debug` (raise
`logging.level` to `debug` to see the full trail, including successful checks). There is no
separate audit sink in v1 — this goes through the normal application logger.

**Startup fails with "access_control: ..." or "authz: routes without Require permission
declaration: ...".** Both are intentional fail-closed checks, not bugs:
- An invalid policy (unknown permission, bad preset reference, admin-only permission handed to a
  team, duplicate team name, empty `claim_values`, or a team with no bindings/cluster
  permissions) refuses to start with a specific error naming the problem.
- A `/api/v1` route wired without a permission requirement also refuses to start — this is a
  build-time safety net, not something an operator triggers by editing config, but it can surface
  after a `git pull` that adds a route if the accompanying enforcement wiring is missing.

## See also

- [config.example.yaml](../config.example.yaml) — the full commented `access_control` example.
- [garage-setup.md](garage-setup.md) — general Garage UI / Garage setup.
