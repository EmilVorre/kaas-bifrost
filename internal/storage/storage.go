// Package storage manages persistent storage for the platform.
// It covers Longhorn block storage health and MinIO object storage
// provisioning for tenant buckets.
package storage

import "github.com/EmilVorre/Bifrost/internal/k8sclient"

// Manager handles storage provisioning and health checks
type Manager struct {
	client *k8sclient.Client
}

// New creates a new storage Manager.
func New(client *k8sclient.Client) *Manager {
	return &Manager{client: client}
}

// LonghornHealth returns the health status of Longhorn nodes and volumes
// TODO: Query Longhorn CRDs (nodes.longhorn.io, volumes.longhorn.io)
func (m *Manager) LonghornHealth() ([]LonghornNodeStatus, error) {
	return []LonghornNodeStatus{}, nil
}

// ProvisionTenantBucket creates a MinIO bucket for a tenant and stores
// credentials in OpenBao
// TODO: Use MinIO Go SDK to create bucket, store creds at
//       secret/customers/<name>/minio in OpenBao
func (m *Manager) ProvisionTenantBucket(name string) error {
	return nil
}

// DeprovisionTenantBucket removes the MinIO bucket for a tenant
// TODO: Delete bucket and credentials from OpenBao
func (m *Manager) DeprovisionTenantBucket(name string) error {
	return nil
}

// LonghornNodeStatus holds health info for a single Longhorn node
type LonghornNodeStatus struct {
	Node  string
	Ready bool
}