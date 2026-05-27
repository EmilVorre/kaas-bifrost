// Package tentant manages the lifecycle of tenant namespaces on the cluster.
// Its handles namespaces creation, RBAC, resource quotas, and network policies.
package tenant

import (
	"github.com/EmilVorre/Bifrost/internal/k8sclient"
)

// QuotaTier defines the resource quota size for a tentant
type QuotaTier string

const (
	QuotaSmall	QuotaTier = "small"
	QuotaMedium	QuotaTier = "medium"
	QuotaLarge	QuotaTier = "large"
)

// Tenant represents a single tenant on the platform
type Tenant struct {
	Name		string
	Namespace	string
	Quota		QuotaTier
}

// Manager handles tenant provisioning and teardown
type Manager struct {
	client *k8sclient.Client
}

// NewManager creates a new tenant Manager
func NewManager(client *k8sclient.Client) *Manager {
	return &Manager{client: client}
}

// Add provisions a new tenant namespace with RBAC, quotas, and network policies
// TODO: Create namespace, Role, RoleBinding, ResourceQuota, LimitRange,
//		 default-deny NetworkPolicy, CiliumNetworkPolicy allow rules
func (m *Manager) Add(name string, quota QuotaTier) (*Tenant, error) {
	return &Tenant{
		Name:		name,
		Namespace: 	"tenant-" + name,
		Quota:		quota,
	}, nil
}

// Remove tears down all Kubernetes resources for a tentant
// TODO: Delete namespace (cascades all  k8s resources)
func (m *Manager) Remove(name string) error {
	return nil
}

// List returns all active tenants on the cluster
// TODO: List namespaces with the kaas-bifrost.dev/tenant label
func (m *Manager) List() ([]Tenant, error) {
	return []Tenant{}, nil 
}