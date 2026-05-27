// Package ingress manages Ingress resources, TLS certificates via Cert-Manager,
// and DNS records via external-DNS for each tenant.
package ingress

import (
	"github.com/EmilVorre/Bifrost/internal/k8sclient"
)

// Manager handles ingress provisioning for tenants
type Manager struct {
	client		*k8sclient.Client
	baseDomain	string
	issuerName	string
} 

// New creates a new ingress Manager
// baseDomain is the root domain for tenant ingresses (e.g. bifrost.example.com)
// issuerName is the Cert-Manager ClusterIssuer to use (e.g. letsencrypt-prod)
func New(client *k8sclient.Client, baseDomain, issuerName string) *Manager {
	return &Manager{
		client:		client,
		baseDomain: baseDomain,
		issuerName: issuerName,
	}
}

// ProvisionTenant creates an Ingress resource with TLS annotation for a tenant
// External-DNS will pick up the Ingress and create the DNS record automatically
// TODO: Create Ingress with cert-manager.io/cluster-issuer annotation,
//       Cert-Manager will provision the TLS cert, External-DNS creates DNS record
func (m *Manager) ProvisionTenant(name, namespace string) error {
	return nil
}
 
// DeprovisionTenant removes the Ingress resource for a tenant
// External-DNS will clean up the DNS record when the Ingress is deleted
// TODO: Delete Ingress resource from tenant namespace
func (m *Manager) DeprovisionTenant(name, namespace string) error {
	return nil
}
 
// ListCertificates returns certificate expiry info for all tenant ingresses
// TODO: List Certificate resources across all tenant namespaces
func (m *Manager) ListCertificates() ([]CertInfo, error) {
	return []CertInfo{}, nil
}
 
// CertInfo holds certificate status for a single tenant
type CertInfo struct {
	Tenant    string
	Domain    string
	ExpiresAt string
	Ready     bool
}
