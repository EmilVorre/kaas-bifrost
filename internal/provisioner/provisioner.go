// Package provisioner handles bootstrapping Kubernetes nodes over SSH.
// It manages kubeadm init/join, CNI installation, and kubeconfig retrieval.
package provisioner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/EmilVorre/Bifrost/internal/k8sclient"
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
	ControlPlane   NodeConfig
	Worker         []NodeConfig
	KubeconfigPath string
}

// Provisioner manages the cluster bootstrap lifecycle.
type Provisioner struct {
	cfg     ClusterConfig
	joinCmd string
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

		// Check if already initialized
		var isInitialized bool
		if _, err := c.Run(ctx, "test -f /etc/kubernetes/admin.conf"); err == nil {
			isInitialized = true
		}

		if !isInitialized {
			// Run kubeadm init
			initCmd := fmt.Sprintf("sudo kubeadm init --apiserver-advertise-address=%s --pod-network-cidr=10.244.0.0/16", p.cfg.ControlPlane.IP)
			if _, err := c.Run(ctx, initCmd); err != nil {
				return fmt.Errorf("run kubeadm init: %w", err)
			}
		}

		// Retrieve kubeconfig
		result, err := c.Run(ctx, "sudo cat /etc/kubernetes/admin.conf")
		if err != nil {
			return fmt.Errorf("retrieve admin.conf: %w", err)
		}

		// Replace loopback IP with external control plane IP
		modifiedKubeconfig := ReplaceLoopback([]byte(result.Stdout), p.cfg.ControlPlane.IP)

		// Securely store the kubeconfig locally
		expandedKubeconfigPath, err := expandPath(p.cfg.KubeconfigPath)
		if err != nil {
			return fmt.Errorf("expand local kubeconfig path: %w", err)
		}

		// Create parent directory
		parentDir := filepath.Dir(expandedKubeconfigPath)
		if err := os.MkdirAll(parentDir, 0700); err != nil {
			return fmt.Errorf("create local kubeconfig directory %s: %w", parentDir, err)
		}

		// Write kubeconfig file with secure 0600 permissions
		if err := os.WriteFile(expandedKubeconfigPath, modifiedKubeconfig, 0600); err != nil {
			return fmt.Errorf("write local kubeconfig to %s: %w", expandedKubeconfigPath, err)
		}

		// Generate join command for worker nodes
		joinResult, err := c.Run(ctx, "sudo kubeadm token create --print-join-command")
		if err != nil {
			return fmt.Errorf("generate join command: %w", err)
		}

		p.joinCmd = strings.TrimSpace(joinResult.Stdout)
		return nil
	})
}

// JoinWorkers joins all worker nodes to the cluster via kubeadm join.
func (p *Provisioner) JoinWorkers(ctx context.Context) error {
	if p.joinCmd == "" {
		// If Init wasn't called in this invocation, generate the join command on the fly
		err := p.withNode(ctx, p.cfg.ControlPlane, func(c *sshclient.Client) error {
			joinResult, err := c.Run(ctx, "sudo kubeadm token create --print-join-command")
			if err != nil {
				return fmt.Errorf("generate join command: %w", err)
			}
			p.joinCmd = strings.TrimSpace(joinResult.Stdout)
			return nil
		})
		if err != nil {
			return fmt.Errorf("retrieve join command from control plane: %w", err)
		}
	}

	for _, worker := range p.cfg.Worker {
		if err := p.withNode(ctx, worker, func(c *sshclient.Client) error {
			if _, err := c.Run(ctx, "command -v kubeadm >/dev/null"); err != nil {
				return fmt.Errorf("kubeadm not found: %w", err)
			}

			// Check if already joined
			if _, err := c.Run(ctx, "test -f /etc/kubernetes/kubelet.conf"); err == nil {
				// Already joined, skip
				return nil
			}

			// Run kubeadm join
			joinCmd := "sudo " + p.joinCmd
			if _, err := c.Run(ctx, joinCmd); err != nil {
				return fmt.Errorf("run kubeadm join: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// InstallCilium deploys Cilium CNI and Hubble via helm and verifies health.
func (p *Provisioner) InstallCilium(ctx context.Context) error {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm binary not found in PATH: %w", err)
	}

	// 1. Add Cilium Helm Repo
	cmdAdd := exec.CommandContext(ctx, helmPath, "repo", "add", "cilium", "https://helm.cilium.io/")
	if output, err := cmdAdd.CombinedOutput(); err != nil {
		return fmt.Errorf("add cilium helm repo: %w, output: %s", err, string(output))
	}

	// 2. Repo Update
	cmdUpdate := exec.CommandContext(ctx, helmPath, "repo", "update")
	if output, err := cmdUpdate.CombinedOutput(); err != nil {
		return fmt.Errorf("update helm repos: %w, output: %s", err, string(output))
	}

	// 3. Expand local kubeconfig path
	expandedKubeconfigPath, err := expandPath(p.cfg.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("expand local kubeconfig path: %w", err)
	}

	// 4. Install Cilium
	args := []string{
		"upgrade", "--install", "cilium", "cilium/cilium",
		"--namespace", "kube-system",
		"--set", "hubble.enabled=true",
		"--set", "hubble.relay.enabled=true",
		"--set", "hubble.ui.enabled=true",
		"--kubeconfig", expandedKubeconfigPath,
	}
	cmdInstall := exec.CommandContext(ctx, helmPath, args...)
	if output, err := cmdInstall.CombinedOutput(); err != nil {
		return fmt.Errorf("install cilium helm chart: %w, output: %s", err, string(output))
	}

	// 5. Verify CNI health using k8sclient
	k8s, err := k8sclient.New(expandedKubeconfigPath)
	if err != nil {
		return fmt.Errorf("initialize kubernetes client: %w", err)
	}

	fmt.Println("  Waiting for Cilium DaemonSet to be ready...")
	if err := k8s.WaitForDaemonSetReady(ctx, "kube-system", "cilium", 5*time.Minute); err != nil {
		return fmt.Errorf("cilium daemonset health check: %w", err)
	}

	fmt.Println("  Waiting for Hubble Relay to be ready...")
	if err := k8s.WaitForDeploymentReady(ctx, "kube-system", "hubble-relay", 5*time.Minute); err != nil {
		return fmt.Errorf("hubble relay deployment health check: %w", err)
	}

	fmt.Println("  Waiting for Hubble UI to be ready...")
	if err := k8s.WaitForDeploymentReady(ctx, "kube-system", "hubble-ui", 5*time.Minute); err != nil {
		return fmt.Errorf("hubble ui deployment health check: %w", err)
	}

	return nil
}

// ClusterConfigFromFlags builds cluster topology from CLI flag values.
func ClusterConfigFromFlags(controlPlaneIP string, workerIPs []string, sshUser, sshKeyPath, kubeconfigPath string) ClusterConfig {
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
		Worker:         workers,
		KubeconfigPath: kubeconfigPath,
	}
}

// ReplaceLoopback replaces loopback server IPs/hosts in kubeconfig with control plane IP.
func ReplaceLoopback(content []byte, controlPlaneIP string) []byte {
	s := string(content)
	s = strings.ReplaceAll(s, "server: https://127.0.0.1:", "server: https://"+controlPlaneIP+":")
	s = strings.ReplaceAll(s, "server: https://localhost:", "server: https://"+controlPlaneIP+":")
	return []byte(s)
}

func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
