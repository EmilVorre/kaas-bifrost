// Package provisioner handles bootstrappign Kubernetes nodes over SSH.
// It maneges kubeamd init/join, CNI instilation, and kubeconfig retrieval.
package provisioner

// NodeConfig holds SSH connection details for a single node
type NodeConfig struct {
	IP			string
	SSHUser		string
	SShKeyPath	string
}

// ClusterConfig holds the full cluster topology  for provisioning
type ClusterConfig struct {
	ControlPlane	NodeConfig
	Worker			[]NodeConfig
}

// Provisioner manages the cluster bootstrap lifecycle
type Provisioner struct {
	cfg ClusterConfig
}

// New creates a new provisioner with teh given cluster config
func New(cfg ClusterConfig) *Provisioner {
	return &Provisioner{cfg: cfg}
}

// Init bootstraps the control plane via kubeadm over SSH
// TODO: SSH into control plane, run kubeadm init, retrieve kubeconfig
func (p *Provisioner) Init() error {
	return nil
}

// JoinWorkers joins all worker nodes to the cluster via kubeadm join
// TODO: SSH into each worker, run kubeadm join with the token from Init
func (p *Provisioner)JoinWorkers() error {
	return nil
}

// InstallCilium deploys Cilium CNI and Hubble via helm
// TODO: Apply Cilium heml chart to the bootstrapped cluster
func (p *Provisioner) InstallCilium() error {
	return nil
}