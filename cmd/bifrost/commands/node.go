package commands

import (
	"fmt"

	"github.com/EmilVorre/Bifrost/internal/k8sclient"
	"github.com/EmilVorre/Bifrost/internal/sshclient"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	nodeIP         string
	nodeSSHUser    string
	nodeSSHKeyPath string
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage cluster nodes",
	Long:  `Drain, reset, and remove nodes from the KaaS Bifrost cluster.`,
}

var nodeRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Drain and remove a node from the cluster",
	Long: `Drains all pods from a node, runs 'kubeadm reset' over SSH, and deletes the node from the cluster.

Example:
  bifrost node remove --ip 10.0.0.3 --ssh-user root --ssh-key ~/.ssh/id_ed25519`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		logger.Info("Starting node removal process", zap.String("ip", nodeIP))

		// 1. Initialize client-go
		fmt.Printf("→ Connecting to cluster via kubeconfig at %s...\n", kubeconfigPath)
		k8s, err := k8sclient.New(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("initialize kubernetes client: %w", err)
		}

		// 2. Find node name by internal IP
		fmt.Printf("→ Locating Kubernetes node with IP %s...\n", nodeIP)
		nodeName, err := k8s.FindNodeNameByIP(ctx, nodeIP)
		if err != nil {
			return fmt.Errorf("locate node by IP: %w", err)
		}
		fmt.Printf("✓ Found node: %s\n", nodeName)

		// 3. Cordon the node
		fmt.Printf("→ Cordoning node %s...\n", nodeName)
		if err := k8s.CordonNode(ctx, nodeName); err != nil {
			return fmt.Errorf("cordon node: %w", err)
		}
		fmt.Printf("✓ Node %s cordoned successfully\n", nodeName)

		// 4. Drain the node
		fmt.Printf("→ Draining pods from node %s (this might take a few minutes)...\n", nodeName)
		if err := k8s.DrainNode(ctx, nodeName); err != nil {
			return fmt.Errorf("drain node: %w", err)
		}
		fmt.Printf("✓ Node %s drained successfully\n", nodeName)

		// 5. SSH into the node and run kubeadm reset
		fmt.Printf("→ Resetting kubeadm on node %s over SSH...\n", nodeIP)
		sshCfg := sshclient.Config{
			Host:       nodeIP,
			User:       nodeSSHUser,
			PrivateKey: nodeSSHKeyPath,
		}
		client, err := sshclient.New(sshCfg)
		if err != nil {
			return fmt.Errorf("initialize ssh client for %s: %w", nodeIP, err)
		}
		defer func() { _ = client.Close() }()

		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("ssh connect to %s: %w", nodeIP, err)
		}

		// Run kubeadm reset
		fmt.Println("  Running 'kubeadm reset --force' on remote node...")
		if _, err := client.Run(ctx, "sudo kubeadm reset --force"); err != nil {
			return fmt.Errorf("run kubeadm reset: %w", err)
		}
		fmt.Println("✓ Remote kubeadm reset complete")

		// 6. Delete the Node object from the cluster
		fmt.Printf("→ Deleting node %s from Kubernetes cluster...\n", nodeName)
		if err := k8s.DeleteNode(ctx, nodeName); err != nil {
			return fmt.Errorf("delete node from cluster: %w", err)
		}
		fmt.Printf("✓ Node %s removed successfully from cluster\n", nodeName)

		return nil
	},
}

func init() {
	nodeRemoveCmd.Flags().StringVar(&nodeIP, "ip", "", "IP address of the node to remove (required)")
	nodeRemoveCmd.Flags().StringVar(&nodeSSHUser, "ssh-user", "root", "SSH user for node access")
	nodeRemoveCmd.Flags().StringVar(&nodeSSHKeyPath, "ssh-key", "~/.ssh/id_ed25519", "Path to SSH private key")

	if err := nodeRemoveCmd.MarkFlagRequired("ip"); err != nil {
		panic("failed to mark flag required: " + err.Error())
	}

	nodeCmd.AddCommand(nodeRemoveCmd)
}
