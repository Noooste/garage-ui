package models

import "time"

// ====================================
// Access Key Models
// ====================================

// GarageKeyInfo represents detailed information about a Garage access key
type GarageKeyInfo struct {
	AccessKeyID     string          `json:"accessKeyId"`
	Name            string          `json:"name"`
	Expired         bool            `json:"expired"`
	SecretAccessKey *string         `json:"secretAccessKey,omitempty"`
	Permissions     KeyPermissions  `json:"permissions"`
	Buckets         []KeyBucketInfo `json:"buckets"`
	Created         *time.Time      `json:"created,omitempty"`
	Expiration      *time.Time      `json:"expiration,omitempty"`
}

// KeyPermissions represents permissions for an access key
type KeyPermissions struct {
	CreateBucket bool `json:"createBucket"`
}

// KeyBucketInfo represents bucket information associated with a key
type KeyBucketInfo struct {
	ID            string              `json:"id"`
	GlobalAliases []string            `json:"globalAliases"`
	LocalAliases  []string            `json:"localAliases"`
	Permissions   BucketKeyPermission `json:"permissions"`
}

// BucketKeyPermission represents permissions a key has on a specific bucket
type BucketKeyPermission struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
	Owner bool `json:"owner"`
}

// CreateKeyRequest represents the request to create a new access key
type CreateKeyRequest struct {
	Name         *string         `json:"name,omitempty"`
	Expiration   *time.Time      `json:"expiration,omitempty"`
	NeverExpires bool            `json:"neverExpires,omitempty"`
	Allow        *KeyPermissions `json:"allow,omitempty"`
	Deny         *KeyPermissions `json:"deny,omitempty"`
}

// UpdateKeyRequest represents the request to update an access key
type UpdateKeyRequest struct {
	Name         *string         `json:"name,omitempty"`
	Expiration   *time.Time      `json:"expiration,omitempty"`
	NeverExpires bool            `json:"neverExpires,omitempty"`
	Allow        *KeyPermissions `json:"allow,omitempty"`
	Deny         *KeyPermissions `json:"deny,omitempty"`
}

// ImportKeyRequest represents the request to import an existing key
type ImportKeyRequest struct {
	AccessKeyID     string  `json:"accessKeyId"`
	SecretAccessKey string  `json:"secretAccessKey"`
	Name            *string `json:"name,omitempty"`
}

// ListKeysResponseItem represents a single key in the list response
type ListKeysResponseItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Expired    bool       `json:"expired"`
	Created    *time.Time `json:"created,omitempty"`
	Expiration *time.Time `json:"expiration,omitempty"`
}

// ====================================
// Bucket Models (Admin API)
// ====================================

// GarageBucketInfo represents detailed information about a bucket from Admin API
type GarageBucketInfo struct {
	ID                             string               `json:"id"`
	Created                        time.Time            `json:"created"`
	GlobalAliases                  []string             `json:"globalAliases"`
	WebsiteAccess                  bool                 `json:"websiteAccess"`
	WebsiteConfig                  *BucketWebsiteConfig `json:"websiteConfig,omitempty"`
	Keys                           []BucketKeyInfo      `json:"keys"`
	Objects                        int64                `json:"objects"`
	Bytes                          int64                `json:"bytes"`
	UnfinishedUploads              int64                `json:"unfinishedUploads"`
	UnfinishedMultipartUploads     int64                `json:"unfinishedMultipartUploads"`
	UnfinishedMultipartUploadParts int64                `json:"unfinishedMultipartUploadParts"`
	UnfinishedMultipartUploadBytes int64                `json:"unfinishedMultipartUploadBytes"`
	Quotas                         *BucketQuotas        `json:"quotas,omitempty"`
}

// BucketWebsiteConfig represents website configuration for a bucket
type BucketWebsiteConfig struct {
	IndexDocument string  `json:"indexDocument"`
	ErrorDocument *string `json:"errorDocument,omitempty"`
}

// BucketQuotas represents quota settings for a bucket
type BucketQuotas struct {
	MaxSize    *int64 `json:"maxSize,omitempty"`
	MaxObjects *int64 `json:"maxObjects,omitempty"`
}

// BucketKeyInfo represents key information associated with a bucket
type BucketKeyInfo struct {
	AccessKeyID        string              `json:"accessKeyId"`
	Name               string              `json:"name"`
	Permissions        BucketKeyPermission `json:"permissions"`
	BucketLocalAliases []string            `json:"bucketLocalAliases"`
}

// CreateBucketAdminRequest represents the request to create a bucket via Admin API
type CreateBucketAdminRequest struct {
	GlobalAlias *string                 `json:"globalAlias,omitempty"`
	LocalAlias  *CreateBucketLocalAlias `json:"localAlias,omitempty"`
}

// CreateBucketLocalAlias represents local alias configuration when creating a bucket
type CreateBucketLocalAlias struct {
	AccessKeyID string               `json:"accessKeyId"`
	Alias       string               `json:"alias"`
	Allow       *BucketKeyPermission `json:"allow,omitempty"`
}

// UpdateBucketRequest represents the request to update bucket settings
type UpdateBucketRequest struct {
	WebsiteAccess *UpdateBucketWebsiteAccess `json:"websiteAccess,omitempty"`
	Quotas        *BucketQuotas              `json:"quotas,omitempty"`
}

// UpdateBucketWebsiteAccess represents website access settings update
type UpdateBucketWebsiteAccess struct {
	Enabled       bool    `json:"enabled"`
	IndexDocument *string `json:"indexDocument,omitempty"`
	ErrorDocument *string `json:"errorDocument,omitempty"`
}

// ListBucketsResponseItem represents a single bucket in the list response
type ListBucketsResponseItem struct {
	ID            string             `json:"id"`
	Created       time.Time          `json:"created"`
	GlobalAliases []string           `json:"globalAliases"`
	LocalAliases  []BucketLocalAlias `json:"localAliases"`
}

// BucketLocalAlias represents a local alias for a bucket
type BucketLocalAlias struct {
	AccessKeyID string `json:"accessKeyId"`
	Alias       string `json:"alias"`
}

// ====================================
// Bucket Alias Models
// ====================================

// AddBucketAliasRequest represents the request to add a bucket alias
type AddBucketAliasRequest struct {
	BucketID    string  `json:"bucketId"`
	GlobalAlias *string `json:"globalAlias,omitempty"`
	LocalAlias  *string `json:"localAlias,omitempty"`
	AccessKeyID *string `json:"accessKeyId,omitempty"`
}

// RemoveBucketAliasRequest represents the request to remove a bucket alias
type RemoveBucketAliasRequest struct {
	BucketID    string  `json:"bucketId"`
	GlobalAlias *string `json:"globalAlias,omitempty"`
	LocalAlias  *string `json:"localAlias,omitempty"`
	AccessKeyID *string `json:"accessKeyId,omitempty"`
}

// ====================================
// Permission Models
// ====================================

// BucketKeyPermRequest represents a request to change bucket-key permissions
type BucketKeyPermRequest struct {
	BucketID    string              `json:"bucketId"`
	AccessKeyID string              `json:"accessKeyId"`
	Permissions BucketKeyPermission `json:"permissions"`
}

// ====================================
// Cluster Models
// ====================================

// ClusterHealth represents the health status of the cluster
type ClusterHealth struct {
	Status           string `json:"status"`
	KnownNodes       int    `json:"knownNodes"`
	ConnectedNodes   int    `json:"connectedNodes"`
	StorageNodes     int    `json:"storageNodes"`
	StorageNodesUp   int    `json:"storageNodesUp"`
	Partitions       int    `json:"partitions"`
	PartitionsQuorum int    `json:"partitionsQuorum"`
	PartitionsAllOk  int    `json:"partitionsAllOk"`
}

// ClusterStatus represents the current status of the cluster
type ClusterStatus struct {
	LayoutVersion int        `json:"layoutVersion"`
	Nodes         []NodeInfo `json:"nodes"`
}

// ClusterStatistics represents global cluster statistics
type ClusterStatistics struct {
	Freeform string `json:"freeform"`
}

// ====================================
// Node Models
// ====================================

// NodeInfo represents information about a cluster node
type NodeInfo struct {
	ID                string         `json:"id"`
	IsUp              bool           `json:"isUp"`
	LastSeenSecsAgo   *int64         `json:"lastSeenSecsAgo,omitempty"`
	Hostname          *string        `json:"hostname,omitempty"`
	Addr              *string        `json:"addr,omitempty"`
	GarageVersion     *string        `json:"garageVersion,omitempty"`
	Role              *NodeRole      `json:"role,omitempty"`
	Draining          bool           `json:"draining"`
	DataPartition     *FreeSpaceInfo `json:"dataPartition,omitempty"`
	MetadataPartition *FreeSpaceInfo `json:"metadataPartition,omitempty"`
}

// NodeRole represents the role assigned to a node
type NodeRole struct {
	Zone     string   `json:"zone"`
	Capacity *int64   `json:"capacity,omitempty"`
	Tags     []string `json:"tags"`
}

// FreeSpaceInfo represents disk space information
type FreeSpaceInfo struct {
	Available int64 `json:"available"`
	Total     int64 `json:"total"`
}

// NodeInfoResponse represents the response for GetNodeInfo
type NodeInfoResponse struct {
	NodeID         string   `json:"nodeId"`
	GarageVersion  string   `json:"garageVersion"`
	RustVersion    string   `json:"rustVersion"`
	DBEngine       string   `json:"dbEngine"`
	GarageFeatures []string `json:"garageFeatures,omitempty"`
}

// NodeStatisticsResponse represents the response for GetNodeStatistics
type NodeStatisticsResponse struct {
	Freeform string `json:"freeform"`
}

// MultiNodeResponse represents responses from multiple nodes
type MultiNodeResponse struct {
	Success map[string]interface{} `json:"success"`
	Error   map[string]string      `json:"error"`
}
