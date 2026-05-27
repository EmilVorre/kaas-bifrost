// Package telemetry manages the Grafana LGTM observability stack.
// It handles health checks for Prometheus, Loki, Tempo, and Grafana,
// and provisions per-tenant Grafana dashboards.
package telemetry

import "github.com/EmilVorre/Bifrost/internal/k8sclient"

// Manager handles telemetry stack provisioning and health
type Manager struct {
	client      *k8sclient.Client
	grafanaURL  string
	grafanaUser string
	grafanaPass string
}

// New creates a new telemetry Manager.
func New(client *k8sclient.Client, grafanaURL, grafanaUser, grafanaPass string) *Manager {
	return &Manager{
		client:      client,
		grafanaURL:  grafanaURL,
		grafanaUser: grafanaUser,
		grafanaPass: grafanaPass,
	}
}

// ProvisionTenantDashboard creates a Grafana dashboard scoped to the tenant namespace
// TODO: POST to Grafana API to create a dashboard with namespace label selector
func (m *Manager) ProvisionTenantDashboard(name, namespace string) error {
	return nil
}

// DeprovisionTenantDashboard removes the Grafana dashboard for a tenant
// TODO: DELETE dashboard from Grafana API
func (m *Manager) DeprovisionTenantDashboard(name string) error {
	return nil
}

// StackHealth returns the health status of all LGTM components
// TODO: Check Prometheus, Loki, Tempo, Grafana pod readiness
func (m *Manager) StackHealth() (StackStatus, error) {
	return StackStatus{}, nil
}

// StackStatus holds health info for the full LGTM stack
type StackStatus struct {
	Prometheus bool
	Loki       bool
	Tempo      bool
	Grafana    bool
}