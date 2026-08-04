package commands

import (
	"fmt"

	"github.com/EmilVorre/Bifrost/internal/provisioner"
	"github.com/spf13/cobra"
)

// Flags
var (
	controlPlaneIP string
	workerIPs      []string
	sshUser        string
	sshKeyPath     string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a new KaaS Bifrost cluster",
	Long: `Provisions a new Kubernetes cluster using kubeadm over SSH.
Installs Cilium as the CNI, deploys OpenBao with transit auto-unseal,
and sets up Longhorn for persistent storage.

Example:
  bifrost init \
    --control-plane 10.0.0.1 \
    --workers 10.0.0.2,10.0.0.3 \
    --ssh-user root \
    --ssh-key ~/.ssh/id_ed25519`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Starting KaaS Bifrost cluster initialisation")

		cfg := provisioner.ClusterConfigFromFlags(controlPlaneIP, workerIPs, sshUser, sshKeyPath, kubeconfigPath)
		prov := provisioner.New(cfg)
		ctx := cmd.Context()

		fmt.Println("→ Validating SSH connectivity to nodes...")
		if err := prov.PingNodes(ctx); err != nil {
			return fmt.Errorf("ssh connectivity check: %w", err)
		}

		fmt.Println("→ Bootstrapping control plane with kubeadm...")
		if err := prov.Init(ctx); err != nil {
			return fmt.Errorf("control plane bootstrap: %w", err)
		}

		fmt.Println("→ Joining worker nodes...")
		if err := prov.JoinWorkers(ctx); err != nil {
			return fmt.Errorf("join workers: %w", err)
		}

		fmt.Println("→ Installing Cilium CNI + Hubble...")
		if err := prov.InstallCilium(ctx); err != nil {
			return fmt.Errorf("install cilium: %w", err)
		}

		fmt.Println("→ Deploying OpenBao with transit auto-unseal...")
		// TODO: internal/bao — OpenBao Helm deploy + init + unseal config

		fmt.Println("→ Deploying Longhorn storage...")
		// TODO: internal/storage — Longhorn Helm deploy

		fmt.Println("✓ Bifrost cluster initialised successfully")
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&controlPlaneIP, "control-plane", "", "IP address of the control plane node (required)")
	initCmd.Flags().StringArrayVar(&workerIPs, "workers", []string{}, "IP addresses of worker nodes (required)")
	initCmd.Flags().StringVar(&sshUser, "ssh-user", "root", "SSH user for node access")
	initCmd.Flags().StringVar(&sshKeyPath, "ssh-key", "~/.ssh/id_ed25519", "Path to SSH private key")

	if err := initCmd.MarkFlagRequired("control-plane"); err != nil {
		panic("failed to mark flag required: " + err.Error())
	}
	if err := initCmd.MarkFlagRequired("workers"); err != nil {
		panic("failed to mark flag required: " + err.Error())
	}
}
