// Package bao manages interactions with the OpenBao API.
package bao

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/EmilVorre/Bifrost/internal/k8sclient"
	bao "github.com/openbao/openbao/api/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace        = "kaas-system"
	rootReleaseName  = "openbao-root"
	mainReleaseName  = "openbao"
	transitRootToken = "root-transit-token"
)

// DeployAndConfigure provisions the root transit OpenBao and the cluster OpenBao,
// setting up transit auto-unseal and enabling Kubernetes auth.
func DeployAndConfigure(ctx context.Context, kubeconfigPath string) error {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm binary not found in PATH: %w", err)
	}

	expandedKubeconfig, err := expandPath(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to expand kubeconfig path: %w", err)
	}

	// 1. Add OpenBao Helm Repository
	cmdAdd := exec.CommandContext(ctx, helmPath, "repo", "add", "openbao", "https://openbao.github.io/openbao-helm")
	if output, err := cmdAdd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add openbao helm repo: %w, output: %s", err, string(output))
	}

	// 2. Update Helm Repositories
	cmdUpdate := exec.CommandContext(ctx, helmPath, "repo", "update")
	if output, err := cmdUpdate.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repos: %w, output: %s", err, string(output))
	}

	// 3. Deploy OpenBao Root (Transit Unsealer)
	fmt.Println("  Deploying root unsealer OpenBao...")
	rootArgs := []string{
		"upgrade", "--install", rootReleaseName, "openbao/openbao",
		"--namespace", namespace,
		"--create-namespace",
		"--set", "server.dev.enabled=true",
		"--set", "server.dev.devRootToken=" + transitRootToken,
		"--kubeconfig", expandedKubeconfig,
	}
	cmdRootInstall := exec.CommandContext(ctx, helmPath, rootArgs...)
	if output, err := cmdRootInstall.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install root openbao: %w, output: %s", err, string(output))
	}

	// Initialize Kubernetes client
	k8s, err := k8sclient.New(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to initialize kubernetes client: %w", err)
	}

	// 4. Wait for root unsealer pod to be ready
	rootPodName := fmt.Sprintf("%s-0", rootReleaseName)
	fmt.Printf("  Waiting for root unsealer pod (%s) to be ready...\n", rootPodName)
	if err := k8s.WaitForPodReady(ctx, namespace, rootPodName, 5*time.Minute); err != nil {
		return fmt.Errorf("root unsealer pod not ready: %w", err)
	}

	// 5. Establish port-forward to root unsealer
	fmt.Println("  Establishing secure port-forward to root unsealer...")
	stopRootPF, _, err := k8s.PortForward(ctx, namespace, rootPodName, 8201, 8200)
	if err != nil {
		return fmt.Errorf("failed to port-forward to root unsealer: %w", err)
	}
	defer close(stopRootPF)

	// 6. Connect to root unsealer and configure Transit engine
	fmt.Println("  Configuring Transit secret engine on root unsealer...")
	rootClient, err := New("http://127.0.0.1:8201", transitRootToken)
	if err != nil {
		return fmt.Errorf("failed to connect to root unsealer: %w", err)
	}

	// Mount transit engine
	_ = rootClient.api.Sys().Mount("transit", &bao.MountInput{
		Type:        "transit",
		Description: "Transit secrets engine for auto-unseal",
	})

	// Create unseal key
	_, err = rootClient.api.Logical().Write("transit/keys/autounseal", nil)
	if err != nil {
		return fmt.Errorf("failed to create transit auto-unseal key: %w", err)
	}

	// Create policy for cluster unsealing
	policyRules := `
path "transit/encrypt/autounseal" {
  capabilities = ["update"]
}
path "transit/decrypt/autounseal" {
  capabilities = ["update"]
}
`
	err = rootClient.api.Sys().PutPolicy("autounseal-policy", policyRules)
	if err != nil {
		return fmt.Errorf("failed to write autounseal policy: %w", err)
	}

	// Generate a periodic unseal token
	tokenSecret, err := rootClient.api.Auth().Token().Create(&bao.TokenCreateRequest{
		Policies: []string{"autounseal-policy"},
		Period:   "24h",
		NoParent: true,
	})
	if err != nil {
		return fmt.Errorf("failed to generate transit unseal token: %w", err)
	}
	transitToken := tokenSecret.Auth.ClientToken

	// 7. Store transit token in Kubernetes Secret for cluster OpenBao to consume
	secretName := "openbao-transit-unseal"
	fmt.Printf("  Creating Kubernetes Secret %s...\n", secretName)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"token": transitToken,
		},
	}
	_ = k8s.Clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	_, err = k8s.Clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to store transit unseal token in secret: %w", err)
	}

	// 8. Deploy Main Cluster OpenBao
	fmt.Println("  Deploying cluster OpenBao with transit unseal config...")
	valuesContent := `server:
  enabled: true
  extraSecretEnvironmentVars:
    - envName: VAULT_TOKEN
      secretName: openbao-transit-unseal
      secretKey: token
    - envName: BAO_TOKEN
      secretName: openbao-transit-unseal
      secretKey: token
  standalone:
    enabled: true
    config: |
      ui = true
      
      listener "tcp" {
        tls_disable = 1
        address     = "[::]:8200"
        cluster_address = "[::]:8201"
      }
      
      storage "file" {
        path = "/openbao/data"
      }
      
      seal "transit" {
        address         = "http://openbao-root.kaas-system.svc.cluster.local:8200"
        key_name        = "autounseal"
        mount_path      = "transit/"
        tls_skip_verify = "true"
      }
`
	tmpFile, err := os.CreateTemp("", "openbao-values-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp values file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(valuesContent); err != nil {
		return fmt.Errorf("failed to write values file: %w", err)
	}
	_ = tmpFile.Close()

	mainArgs := []string{
		"upgrade", "--install", mainReleaseName, "openbao/openbao",
		"--namespace", namespace,
		"-f", tmpFile.Name(),
		"--kubeconfig", expandedKubeconfig,
	}
	cmdMainInstall := exec.CommandContext(ctx, helmPath, mainArgs...)
	if output, err := cmdMainInstall.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install cluster openbao: %w, output: %s", err, string(output))
	}

	// 9. Wait for cluster OpenBao pod to be ready
	mainPodName := fmt.Sprintf("%s-0", mainReleaseName)
	fmt.Printf("  Waiting for cluster OpenBao pod (%s) to be ready...\n", mainPodName)
	if err := k8s.WaitForPodReady(ctx, namespace, mainPodName, 5*time.Minute); err != nil {
		return fmt.Errorf("cluster openbao pod not ready: %w", err)
	}

	// 10. Establish port-forward to cluster OpenBao
	fmt.Println("  Establishing secure port-forward to cluster OpenBao...")
	stopMainPF, _, err := k8s.PortForward(ctx, namespace, mainPodName, 8202, 8200)
	if err != nil {
		return fmt.Errorf("failed to port-forward to cluster OpenBao: %w", err)
	}
	defer close(stopMainPF)

	// 11. Connect to cluster OpenBao and initialize
	fmt.Println("  Checking initialization status of cluster OpenBao...")
	mainAddress := "http://127.0.0.1:8202"
	config := bao.DefaultConfig()
	config.Address = mainAddress
	mainApiClient, err := bao.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create cluster openbao API client: %w", err)
	}

	initStatus, err := mainApiClient.Sys().InitStatus()
	if err != nil {
		return fmt.Errorf("failed to check cluster openbao init status: %w", err)
	}

	var rootToken string
	homeDir, _ := os.UserHomeDir()
	bifrostDir := filepath.Join(homeDir, ".bifrost")

	if !initStatus {
		fmt.Println("  Initializing cluster OpenBao (transit auto-unsealing active)...")
		initResponse, err := mainApiClient.Sys().Init(&bao.InitRequest{
			RecoveryShares:    5,
			RecoveryThreshold: 3,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize cluster openbao: %w", err)
		}
		rootToken = initResponse.RootToken

		// Persist credentials locally
		_ = os.MkdirAll(bifrostDir, 0700)
		_ = os.WriteFile(filepath.Join(bifrostDir, "bao-token"), []byte(rootToken), 0600)

		var keysStr strings.Builder
		for _, key := range initResponse.Keys {
			keysStr.WriteString(key + "\n")
		}
		_ = os.WriteFile(filepath.Join(bifrostDir, "bao-recovery-keys"), []byte(keysStr.String()), 0600)

		// Store credentials in a secure Kubernetes Secret for fallback retrieval
		credSecretName := "openbao-init-credentials"
		credSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      credSecretName,
				Namespace: namespace,
			},
			StringData: map[string]string{
				"root-token":    rootToken,
				"recovery-keys": keysStr.String(),
			},
		}
		_ = k8s.Clientset.CoreV1().Secrets(namespace).Delete(ctx, credSecretName, metav1.DeleteOptions{})
		_, err = k8s.Clientset.CoreV1().Secrets(namespace).Create(ctx, credSecret, metav1.CreateOptions{})
		if err != nil {
			fmt.Printf("  Warning: failed to persist credentials to secret %s: %v\n", credSecretName, err)
		}
	} else {
		fmt.Println("  Cluster OpenBao is already initialized.")
		// Retrieve persisted root token
		tokenBytes, err := os.ReadFile(filepath.Join(bifrostDir, "bao-token"))
		if err == nil {
			rootToken = string(tokenBytes)
		} else {
			credSecret, err := k8s.Clientset.CoreV1().Secrets(namespace).Get(ctx, "openbao-init-credentials", metav1.GetOptions{})
			if err == nil {
				rootToken = string(credSecret.Data["root-token"])
			}
		}
	}

	if rootToken == "" {
		return fmt.Errorf("failed to retrieve root token for cluster openbao")
	}

	mainApiClient.SetToken(rootToken)

	// 12. Enable and configure Kubernetes authentication
	fmt.Println("  Enabling and configuring Kubernetes auth method on cluster OpenBao...")
	auths, err := mainApiClient.Sys().ListAuth()
	if err != nil {
		return fmt.Errorf("failed to list auth methods: %w", err)
	}

	if _, ok := auths["kubernetes/"]; !ok {
		err = mainApiClient.Sys().EnableAuthWithOptions("kubernetes", &bao.EnableAuthOptions{
			Type: "kubernetes",
		})
		if err != nil {
			return fmt.Errorf("failed to enable kubernetes auth method: %w", err)
		}
	}

	_, err = mainApiClient.Logical().Write("auth/kubernetes/config", map[string]interface{}{
		"kubernetes_host": "https://kubernetes.default.svc",
	})
	if err != nil {
		return fmt.Errorf("failed to configure kubernetes auth: %w", err)
	}

	fmt.Println("  OpenBao transit auto-unseal setup and initialization successfully completed!")
	return nil
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
