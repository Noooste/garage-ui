// Package authz implements the UI-layer access-control policy for garage-ui.
//
// IMPORTANT: this is UI-layer policy, not a security boundary. garage-ui talks
// to Garage with a single admin token and a single S3 credential set; anyone
// holding those bypasses everything here.
package authz

import "strings"

// ScopeKind says what a permission applies to.
type ScopeKind int

const (
	// ScopeGlobal permissions are all-or-nothing per team (cluster_permissions).
	ScopeGlobal ScopeKind = iota
	// ScopePrefix permissions are gated by a binding's bucket_prefixes.
	ScopePrefix
)

// PermSpec describes one abstract permission. Endpoints lists the Garage admin
// API endpoint names (or S3 data-plane operations) the permission maps to.
// This registry is the only place those names appear in authz.
type PermSpec struct {
	Scope     ScopeKind
	AdminOnly bool // not grantable to teams in v1; only the synthetic admin subject holds it
	Endpoints []string
}

// Permission constants for every permission referenced from route wiring.
const (
	PermBucketList        = "bucket.list"
	PermBucketRead        = "bucket.read"
	PermBucketCreate      = "bucket.create"
	PermBucketUpdate      = "bucket.update"
	PermBucketDelete      = "bucket.delete"
	PermObjectList        = "object.list"
	PermObjectRead        = "object.read"
	PermObjectWrite       = "object.write"
	PermObjectDelete      = "object.delete"
	PermAllowBucketKey    = "permission.allow_bucket_key"
	PermDenyBucketKey     = "permission.deny_bucket_key"
	PermKeyList           = "key.list"
	PermKeyRead           = "key.read"
	PermKeyReadSecret     = "key.read_secret"
	PermKeyCreate         = "key.create"
	PermKeyUpdate         = "key.update"
	PermKeyDelete         = "key.delete"
	PermClusterStatus     = "cluster.status"
	PermClusterHealth     = "cluster.health"
	PermClusterStatistics = "cluster.statistics"
	PermNodeInfo          = "node.info"
	PermNodeStatistics    = "node.statistics"
)

// Vocabulary is the full v1 permission registry, ratified in issue #33.
// admin_token.* is deliberately absent (admin-only implicitly, not modeled).
var Vocabulary = map[string]PermSpec{
	"bucket.list":            {Scope: ScopePrefix, Endpoints: []string{"ListBuckets"}},
	"bucket.read":            {Scope: ScopePrefix, Endpoints: []string{"GetBucketInfo"}},
	"bucket.create":          {Scope: ScopePrefix, Endpoints: []string{"CreateBucket"}},
	"bucket.update":          {Scope: ScopePrefix, Endpoints: []string{"UpdateBucket"}},
	"bucket.delete":          {Scope: ScopePrefix, Endpoints: []string{"DeleteBucket"}},
	"bucket.cleanup_uploads": {Scope: ScopePrefix, Endpoints: []string{"CleanupIncompleteUploads"}},
	"bucket.inspect_object":  {Scope: ScopePrefix, Endpoints: []string{"InspectObject"}},

	"bucket_alias.add":    {Scope: ScopePrefix, Endpoints: []string{"AddBucketAlias"}},
	"bucket_alias.remove": {Scope: ScopePrefix, Endpoints: []string{"RemoveBucketAlias"}},

	// S3 data plane (object browser), no Garage admin endpoint.
	"object.list":   {Scope: ScopePrefix, Endpoints: []string{"S3:ListObjectsV2"}},
	"object.read":   {Scope: ScopePrefix, Endpoints: []string{"S3:GetObject", "S3:HeadObject", "S3:PresignGet"}},
	"object.write":  {Scope: ScopePrefix, Endpoints: []string{"S3:PutObject"}},
	"object.delete": {Scope: ScopePrefix, Endpoints: []string{"S3:DeleteObject", "S3:DeleteObjects"}},

	"permission.allow_bucket_key": {Scope: ScopePrefix, Endpoints: []string{"AllowBucketKey"}},
	"permission.deny_bucket_key":  {Scope: ScopePrefix, Endpoints: []string{"DenyBucketKey"}},

	"key.list":        {Scope: ScopeGlobal, Endpoints: []string{"ListKeys"}},
	"key.read":        {Scope: ScopeGlobal, Endpoints: []string{"GetKeyInfo"}},
	"key.read_secret": {Scope: ScopeGlobal, AdminOnly: true, Endpoints: []string{"GetKeyInfo(showSecretKey)"}},
	"key.create":      {Scope: ScopeGlobal, AdminOnly: true, Endpoints: []string{"CreateKey"}},
	"key.import":      {Scope: ScopeGlobal, AdminOnly: true, Endpoints: []string{"ImportKey"}},
	"key.update":      {Scope: ScopeGlobal, AdminOnly: true, Endpoints: []string{"UpdateKey"}},
	"key.delete":      {Scope: ScopeGlobal, AdminOnly: true, Endpoints: []string{"DeleteKey"}},

	"cluster.status":        {Scope: ScopeGlobal, Endpoints: []string{"GetClusterStatus"}},
	"cluster.health":        {Scope: ScopeGlobal, Endpoints: []string{"GetClusterHealth"}},
	"cluster.statistics":    {Scope: ScopeGlobal, Endpoints: []string{"GetClusterStatistics"}},
	"cluster.connect_nodes": {Scope: ScopeGlobal, Endpoints: []string{"ConnectClusterNodes"}},

	"cluster.layout.read":            {Scope: ScopeGlobal, Endpoints: []string{"GetClusterLayout"}},
	"cluster.layout.history":         {Scope: ScopeGlobal, Endpoints: []string{"GetClusterLayoutHistory"}},
	"cluster.layout.apply":           {Scope: ScopeGlobal, Endpoints: []string{"ApplyClusterLayout"}},
	"cluster.layout.skip_dead_nodes": {Scope: ScopeGlobal, Endpoints: []string{"ClusterLayoutSkipDeadNodes"}},

	"node.info":       {Scope: ScopeGlobal, Endpoints: []string{"GetNodeInfo"}},
	"node.statistics": {Scope: ScopeGlobal, Endpoints: []string{"GetNodeStatistics"}},
	"node.snapshot":   {Scope: ScopeGlobal, Endpoints: []string{"CreateMetadataSnapshot"}},
	"node.repair":     {Scope: ScopeGlobal, Endpoints: []string{"LaunchRepairOperation"}},

	"worker.list":         {Scope: ScopeGlobal, Endpoints: []string{"ListWorkers"}},
	"worker.info":         {Scope: ScopeGlobal, Endpoints: []string{"GetWorkerInfo"}},
	"worker.get_variable": {Scope: ScopeGlobal, Endpoints: []string{"GetWorkerVariable"}},
	"worker.set_variable": {Scope: ScopeGlobal, Endpoints: []string{"SetWorkerVariable"}},

	"block.list_errors": {Scope: ScopeGlobal, Endpoints: []string{"ListBlockErrors"}},
	"block.info":        {Scope: ScopeGlobal, Endpoints: []string{"GetBlockInfo"}},
}

// IsValidPermission reports whether p names a concrete vocabulary entry.
// Globs are patterns, not permissions, and return false.
func IsValidPermission(p string) bool {
	_, ok := Vocabulary[p]
	return ok
}

// ExpandGlob expands a trailing-star pattern ("bucket.*", "cluster.layout.*",
// bare "*") against the vocabulary. Admin-only permissions are never matched
// by globs; they must be held via the synthetic admin subject. Returns nil
// when the pattern matches nothing or has no trailing star.
func ExpandGlob(pattern string) []string {
	if !strings.HasSuffix(pattern, "*") {
		return nil
	}
	prefix := strings.TrimSuffix(pattern, "*")
	var out []string
	for name, spec := range Vocabulary {
		if spec.AdminOnly {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return out
}
