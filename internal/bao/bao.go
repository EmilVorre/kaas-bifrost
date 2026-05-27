// Package bao manage all interactions with OpenBao (secrets engine).
// It handles deployment, auto-unseal configuration, and per-tenant
// secret paths, policies, and Kubernetes auth roles
package bao

import (
	bao "github.com/openbao/openbao/api/v2"
)

// Client wraps the OpenBao API client
type Client struct {
	api *bao.Client
}

// New creates a new OpenBao Client from the given address and token
func New(address, token string) (*Client, error) {
	config := bao.DefaultConfig()
	config.Address = address

	client, err := bao.NewClient(config)
	if err != nil {
		return nil, err
	}

	client.SetToken(token)
	return &Client{api: client}, nil
}

// ProvisionTenant creates an isolated secret path, policy, and
// Kuberetes quth role scoped to the given tenant
// TODO: Write secret path secret/customers/<name>/, 
// 		 write policy, create k8s auth role bound to tenant namespace SA
func (c *Client) ProvisionTenant(name, namespace string) error {
	return nil
}

// DeprovisionTenant removes all OpenBao resources for a tenant
// TODO: Delete secret path, policy, and k8s auth role
func (c *Client) DeprovisionTenant(name string) error {
	return nil
}

// SealStatus returns whether OpenBao is sealed
// TODO: Call /v1/sys/health and return seal status
func (c *Client) SealStatus() (bool, error) {
	return false, nil
}