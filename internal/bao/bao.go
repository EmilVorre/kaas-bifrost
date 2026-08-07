// Package bao manages interactions with the OpenBao API.
// It handles tenant provisioning, deprovisioning, and health status.
package bao

import (
	"fmt"

	bao "github.com/openbao/openbao/api/v2"
)

// Client wraps the OpenBao API client.
type Client struct {
	api *bao.Client
}

// New creates a new OpenBao Client.
func New(address, token string) (*Client, error) {
	config := bao.DefaultConfig()
	config.Address = address

	client, err := bao.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize openbao client: %w", err)
	}

	client.SetToken(token)
	return &Client{api: client}, nil
}

// ProvisionTenant creates a tenant secret path, policy, and Kubernetes auth role.
func (c *Client) ProvisionTenant(name, namespace string) error {
	policyName := fmt.Sprintf("tenant-%s", name)

	// Define policy rules for Key-Value v2 secret engine
	policyRules := fmt.Sprintf(`
path "secret/data/customers/%s/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "secret/metadata/customers/%s/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
`, name, name)

	// Write policy
	err := c.api.Sys().PutPolicy(policyName, policyRules)
	if err != nil {
		return fmt.Errorf("failed to create policy %s: %w", policyName, err)
	}

	// Create Kubernetes auth role bound to tenant-admin service account in the tenant namespace
	rolePath := fmt.Sprintf("auth/kubernetes/role/tenant-%s", name)
	_, err = c.api.Logical().Write(rolePath, map[string]interface{}{
		"bound_service_account_names":      []string{"tenant-admin"},
		"bound_service_account_namespaces": []string{namespace},
		"token_policies":                   []string{policyName},
		"token_ttl":                        "24h",
	})
	if err != nil {
		return fmt.Errorf("failed to configure kubernetes auth role %s: %w", rolePath, err)
	}

	return nil
}

// DeprovisionTenant removes all OpenBao resources for a tenant.
func (c *Client) DeprovisionTenant(name string) error {
	// Delete Kubernetes auth role
	rolePath := fmt.Sprintf("auth/kubernetes/role/tenant-%s", name)
	_, err := c.api.Logical().Delete(rolePath)
	if err != nil {
		return fmt.Errorf("failed to delete kubernetes auth role %s: %w", rolePath, err)
	}

	// Delete policy
	policyName := fmt.Sprintf("tenant-%s", name)
	err = c.api.Sys().DeletePolicy(policyName)
	if err != nil {
		return fmt.Errorf("failed to delete policy %s: %w", policyName, err)
	}

	// Delete Key-Value v2 metadata to purge secrets under customers/name
	metadataPath := fmt.Sprintf("secret/metadata/customers/%s", name)
	_, err = c.api.Logical().Delete(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to delete tenant secret metadata %s: %w", metadataPath, err)
	}

	return nil
}

// SealStatus returns whether OpenBao is currently sealed.
func (c *Client) SealStatus() (bool, error) {
	status, err := c.api.Sys().SealStatus()
	if err != nil {
		return false, fmt.Errorf("failed to retrieve seal status: %w", err)
	}
	return status.Sealed, nil
}
