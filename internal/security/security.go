// Package security manages Falco runtime security and Kyverno admission policies.
// It handles policy application per tenant and alert surfacing for bifrost status.
package security

import "github.com/EmilVorre/Bifrost/internal/k8sclient"

// Manager handles security policy provisioning
type Manager struct {
	client *k8sclient.Client
}

// New creates a new security Manager
func New(client *k8sclient.Client) *Manager {
	return &Manager{client: client}
}

// ApplyTenantPolicies applies Kyverno policies scoped to the tenant namespace
// TODO: Apply PolicyException resources and namespace-scoped Kyverno policies
func (m *Manager) ApplyTenantPolicies(namespace string) error {
	return nil
}

// RemoveTenantPolicies removes Kyverno policies for a tenant namespace
// TODO: Delete namespace-scoped Kyverno policy resources
func (m *Manager) RemoveTenantPolicies(namespace string) error {
	return nil
}

// FalcoAlertSummary returns recent Falco alert counts across all tenant namespaces
// TODO: Query Falco webhook output or Loki for recent alert events
func (m *Manager) FalcoAlertSummary() ([]AlertSummary, error) {
	return []AlertSummary{}, nil
}

// AlertSummary holds a Falco alert count for a single namespace
type AlertSummary struct {
	Namespace string
	Count     int
	Severity  string
}