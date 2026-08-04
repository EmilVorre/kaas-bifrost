package provisioner

import (
	"bytes"
	"testing"
)

func TestReplaceLoopback(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		controlPlaneIP string
		want           string
	}{
		{
			name:           "replace 127.0.0.1",
			content:        "server: https://127.0.0.1:6443",
			controlPlaneIP: "10.0.0.5",
			want:           "server: https://10.0.0.5:6443",
		},
		{
			name:           "replace localhost",
			content:        "server: https://localhost:6443",
			controlPlaneIP: "10.0.0.5",
			want:           "server: https://10.0.0.5:6443",
		},
		{
			name:           "no change for external IP",
			content:        "server: https://192.168.1.100:6443",
			controlPlaneIP: "10.0.0.5",
			want:           "server: https://192.168.1.100:6443",
		},
		{
			name: "multi-line config",
			content: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0t...
    server: https://127.0.0.1:6443
  name: kubernetes`,
			controlPlaneIP: "10.0.0.5",
			want: `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0t...
    server: https://10.0.0.5:6443
  name: kubernetes`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceLoopback([]byte(tt.content), tt.controlPlaneIP)
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("ReplaceLoopback() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

func TestClusterConfigFromFlags(t *testing.T) {
	controlPlaneIP := "10.0.0.1"
	workerIPs := []string{"10.0.0.2", "10.0.0.3"}
	sshUser := "root"
	sshKeyPath := "~/.ssh/id_ed25519"
	kubeconfigPath := "~/.bifrost/kubeconfig"

	cfg := ClusterConfigFromFlags(controlPlaneIP, workerIPs, sshUser, sshKeyPath, kubeconfigPath)

	if cfg.ControlPlane.IP != controlPlaneIP {
		t.Errorf("expected control plane IP %s, got %s", controlPlaneIP, cfg.ControlPlane.IP)
	}
	if len(cfg.Worker) != 2 {
		t.Errorf("expected 2 workers, got %d", len(cfg.Worker))
	}
	if cfg.Worker[0].IP != "10.0.0.2" {
		t.Errorf("expected first worker IP 10.0.0.2, got %s", cfg.Worker[0].IP)
	}
	if cfg.KubeconfigPath != kubeconfigPath {
		t.Errorf("expected kubeconfig path %s, got %s", kubeconfigPath, cfg.KubeconfigPath)
	}
}

func TestExpandPath_NoTilde(t *testing.T) {
	path := "/etc/kubernetes/admin.conf"
	got, err := expandPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != path {
		t.Errorf("expandPath() = %q, want %q", got, path)
	}
}
