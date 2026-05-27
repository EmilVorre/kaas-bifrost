// Package harbor manages the Harbor image registry.
// It handles per-tenant project creation, reboot accounts,
// and imagePullSecret provisioning into tenant namespaces.
package harbor

// Client wraps the Harbor API
type Client struct {
	baseURL		string
	username	string
	password	string
}

// New creates a new Harbor Client
func New(baseURL, username, password string) *Client {
	return &Client{
		baseURL: 	baseURL,
		username: 	username,
		password: 	password,
	}
}

// ProvisionTenant creates a Harbor project and robot acvcount for a tenant,
// storing credentials in OpenBao and creating an imagePullSecret
// TODO: POST /api/v2.0/projects, create robot account, store in Openbao,
//		 create k8s imagePullSecret in tenant namespace
func (c *Client) ProvisionTenant(name string) error {
	return nil
}

// DeprovisionTenant removes the Harbor project and robot account for a tenant
// TODO: DELETE /api/v2.0/projects/<name> , DELETE robot account
func (c *Client) DeprovisionTenant(name string) error {
	return nil
}

// Health checks whether Harbor is reachable and healthy
// TODO: GET /api/v2.0/health and return status
func (c *Client) Health() (bool, error) {
	return true, nil
}
