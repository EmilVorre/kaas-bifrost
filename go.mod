module github.com/<your-handle>/bifrost

go 1.22

require (
	// CLI
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.18.2

	// Kubernetes
	k8s.io/api v0.29.3
	k8s.io/apimachinery v0.29.3
	k8s.io/client-go v0.29.3
	sigs.k8s.io/controller-runtime v0.17.2

	// Cilium
	github.com/cilium/cilium v1.15.3

	// OpenBao
	github.com/openbao/openbao/api v1.12.0

	// Harbor
	github.com/goharbor/go-client v0.26.2

	// SSH (for kubeadm node provisioning)
	golang.org/x/crypto v0.21.0

	// Helm (for deploying platform components)
	helm.sh/helm/v3 v3.14.3

	// HTTP server (dashboard backend)
	github.com/go-chi/chi/v5 v5.0.12

	// Logging
	go.uber.org/zap v1.27.0

	// Testing
	github.com/stretchr/testify v1.9.0
)