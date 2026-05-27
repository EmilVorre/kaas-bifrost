package commands
 
import (
	"fmt"
 
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
    --ssh-user ubuntu \
    --ssh-key ~/.ssh/id_ed25519`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Starting KaaS Bifrost cluster initialisation")
 
		fmt.Println("→ Validating SSH connectivity to nodes...")
		// TODO: internal/provisioner — SSH health check on all nodes
 
		fmt.Println("→ Bootstrapping control plane with kubeadm...")
		// TODO: internal/provisioner — kubeadm init on control plane
 
		fmt.Println("→ Joining worker nodes...")
		// TODO: internal/provisioner — kubeadm join on each worker
 
		fmt.Println("→ Installing Cilium CNI + Hubble...")
		// TODO: internal/provisioner — Cilium Helm deploy
 
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
	initCmd.Flags().StringVar(&sshUser, "ssh-user", "ubuntu", "SSH user for node access")
	initCmd.Flags().StringVar(&sshKeyPath, "ssh-key", "~/.ssh/id_ed25519", "Path to SSH private key")
 
	if err := initCmd.MarkFlagRequired("control-plane"); err != nil {
		panic("failed to mark flag required: " + err.Error())
	}
	if err := initCmd.MarkFlagRequired("workers"); err != nil {
		panic("failed to mark flag required: " + err.Error())
	}
}
