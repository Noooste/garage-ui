package models

import (
	"encoding/json"
	"testing"
)

func TestClusterHealth_StorageNodesOk_BackCompat(t *testing.T) {
	raw := `{
		"status": "healthy",
		"knownNodes": 3,
		"connectedNodes": 3,
		"storageNodes": 3,
		"storageNodesOk": 3,
		"partitions": 256,
		"partitionsQuorum": 256,
		"partitionsAllOk": 256
	}`
	var h ClusterHealth
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.StorageNodesUp != 3 {
		t.Fatalf("expected StorageNodesUp=3, got %d", h.StorageNodesUp)
	}
}

func TestClusterHealth_StorageNodesUp(t *testing.T) {
	raw := `{
		"status": "healthy",
		"knownNodes": 3,
		"connectedNodes": 3,
		"storageNodes": 3,
		"storageNodesUp": 3,
		"partitions": 256,
		"partitionsQuorum": 256,
		"partitionsAllOk": 256
	}`
	var h ClusterHealth
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.StorageNodesUp != 3 {
		t.Fatalf("expected StorageNodesUp=3, got %d", h.StorageNodesUp)
	}
}
