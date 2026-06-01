// Package provisioner handles bootstrapping Kubernetes nodes over SSH.
// It manages kubeadm init/join, CNI installation, and kubeconfig retrieval.
package provisioner

import (
	"context"
	"fmt"

	"github.com/EmilVorre/Bifrost/internal/sshclient"
)

// NodeConfig holds SSH connection details for a single node.
type NodeConfig struct {
	IP         string
	SSHUser    string
	SSHKeyPath string
}

// ClusterConfig holds the full cluster topology for provisioning.
type ClusterConfig struct {
	ControlPlane NodeConfig
	Worker       []NodeConfig
}

// Provisioner manages the cluster bootstrap lifecycle.
type Provisioner struct {
	cfg ClusterConfig
}

// New creates a new provisioner with the given cluster config.
func New(cfg ClusterConfig) *Provisioner {
	return &Provisioner{cfg: cfg}
}

func (p *Provisioner) allNodes() []NodeConfig {
	nodes := make([]NodeConfig, 0, 1+len(p.cfg.Worker))
	nodes = append(nodes, p.cfg.ControlPlane)
	nodes = append(nodes, p.cfg.Worker...)
	return nodes
}

func nodeToSSH(nc NodeConfig) sshclient.Config {
	return sshclient.Config{
		Host:       nc.IP,
		User:       nc.SSHUser,
		PrivateKey: nc.SSHKeyPath,
	}
}

func (p *Provisioner) withNode(ctx context.Context, nc NodeConfig, fn func(*sshclient.Client) error) error {
	client, err := sshclient.New(nodeToSSH(nc))
	if err != nil {
		return fmt.Errorf("node %s: %w", nc.IP, err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("node %s: ssh connect: %w", nc.IP, err)
	}

	if err := fn(client); err != nil {
		return fmt.Errorf("node %s: %w", nc.IP, err)
	}
	return nil
}

// PingNodes verifies SSH connectivity to the control plane and all workers.
func (p *Provisioner) PingNodes(ctx context.Context) error {
	for _, node := range p.allNodes() {
		if err := p.withNode(ctx, node, func(c *sshclient.Client) error {
			_, err := c.Run(ctx, "true")
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// Init bootstraps the control plane via kubeadm over SSH.
func (p *Provisioner) Init(ctx context.Context) error {
	return p.withNode(ctx, p.cfg.ControlPlane, func(c *sshclient.Client) error {
		if _, err := c.Run(ctx, "command -v kubeadm >/dev/null"); err != nil {
			return fmt.Errorf("kubeadm not found on control plane: %w", err)
		}
		// TODO: run kubeadm init, retrieve admin.conf kubeconfig
		return fmt.Errorf("kubeadm init: not implemented")
	})
}

// JoinWorkers joins all worker nodes to the cluster via kubeadm join.
func (p *Provisioner) JoinWorkers(ctx context.Context) error {
	for _, worker := range p.cfg.Worker {
		if err := p.withNode(ctx, worker, func(c *sshclient.Client) error {
			if _, err := c.Run(ctx, "command -v kubeadm >/dev/null"); err != nil {
				return fmt.Errorf("kubeadm not found: %w", err)
			}
			// TODO: run kubeadm join with token from Init
			return fmt.Errorf("kubeadm join: not implemented")
		}); err != nil {
			return err
		}
	}
	return nil
}

// InstallCilium deploys Cilium CNI and Hubble via helm.
// TODO: Apply Cilium helm chart to the bootstrapped cluster.
func (p *Provisioner) InstallCilium() error {
	return nil
}

// ClusterConfigFromFlags builds cluster topology from CLI flag values.
func ClusterConfigFromFlags(controlPlaneIP string, workerIPs []string, sshUser, sshKeyPath string) ClusterConfig {
	workers := make([]NodeConfig, len(workerIPs))
	for i, ip := range workerIPs {
		workers[i] = NodeConfig{
			IP:         ip,
			SSHUser:    sshUser,
			SSHKeyPath: sshKeyPath,
		}
	}
	return ClusterConfig{
		ControlPlane: NodeConfig{
			IP:         controlPlaneIP,
			SSHUser:    sshUser,
			SSHKeyPath: sshKeyPath,
		},
		Worker: workers,
	}
}
