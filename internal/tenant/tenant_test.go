package tenant

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/EmilVorre/Bifrost/internal/k8sclient"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestTenantManager(t *testing.T) {
	ctx := context.Background()

	// Create test templates directory and template files for unit tests
	tmpDir, err := os.MkdirTemp("", "bifrost-templates-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create subdirectories configs/templates inside the tmp directory
	configsDir := filepath.Join(tmpDir, "configs", "templates")
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write mock templates
	defaultDenyContent := `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: {{ .Namespace }}
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress`

	tenantAllowContent := `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tenant-allow-rules
  namespace: {{ .Namespace }}
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress`

	ciliumL7Content := `apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: tenant-l7-rules
  namespace: {{ .Namespace }}
spec:
  endpointSelector:
    matchLabels:
      app: app-with-l7-rules`

	if err := os.WriteFile(filepath.Join(configsDir, "default-deny.yaml"), []byte(defaultDenyContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configsDir, "tenant-allow.yaml"), []byte(tenantAllowContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configsDir, "cilium-l7.yaml"), []byte(ciliumL7Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Change current working directory to tmpDir during test execution
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// 1. Initialize fake client-go and dynamic client
	clientset := fake.NewSimpleClientset()
	scheme := runtime.NewScheme()

	gvrToListKind := map[schema.GroupVersionResource]string{
		{
			Group:    "cilium.io",
			Version:  "v2",
			Resource: "ciliumnetworkpolicies",
		}: "CiliumNetworkPolicyList",
	}
	dynamicClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	k8s := &k8sclient.Client{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
	}

	mgr := NewManager(k8s)

	// 2. Test Add tenant
	tenantName := "acme"
	quota := QuotaSmall
	namespaceName := "tenant-" + tenantName

	ten, err := mgr.Add(ctx, tenantName, quota)
	if err != nil {
		t.Fatalf("Add tenant returned error: %v", err)
	}

	if ten.Name != tenantName {
		t.Errorf("expected tenant name %s, got %s", tenantName, ten.Name)
	}
	if ten.Namespace != namespaceName {
		t.Errorf("expected namespace %s, got %s", namespaceName, ten.Namespace)
	}
	if ten.Quota != quota {
		t.Errorf("expected quota %s, got %s", quota, ten.Quota)
	}

	// 3. Verify namespace was created with correct labels
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get created namespace: %v", err)
	}
	if ns.Labels["kaas-bifrost.dev/tenant"] != tenantName {
		t.Errorf("expected label kaas-bifrost.dev/tenant to be %s, got %s", tenantName, ns.Labels["kaas-bifrost.dev/tenant"])
	}
	if ns.Labels["kaas-bifrost.dev/quota"] != string(quota) {
		t.Errorf("expected label kaas-bifrost.dev/quota to be %s, got %s", quota, ns.Labels["kaas-bifrost.dev/quota"])
	}

	// 4. Verify ServiceAccount, Role, RoleBinding were created
	_, err = clientset.CoreV1().ServiceAccounts(namespaceName).Get(ctx, "tenant-admin", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected ServiceAccount tenant-admin to exist, got error: %v", err)
	}

	_, err = clientset.RbacV1().Roles(namespaceName).Get(ctx, "tenant-admin-role", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected Role tenant-admin-role to exist, got error: %v", err)
	}

	_, err = clientset.RbacV1().RoleBindings(namespaceName).Get(ctx, "tenant-admin-binding", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected RoleBinding tenant-admin-binding to exist, got error: %v", err)
	}

	// 5. Verify ResourceQuota and LimitRange were created
	rq, err := clientset.CoreV1().ResourceQuotas(namespaceName).Get(ctx, "tenant-quota", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected ResourceQuota tenant-quota to exist, got error: %v", err)
	}
	podsLimit := rq.Spec.Hard[v1.ResourcePods]
	if podsLimit.Value() != 20 {
		t.Errorf("expected ResourceQuota pods limit to be 20, got %d", podsLimit.Value())
	}

	_, err = clientset.CoreV1().LimitRanges(namespaceName).Get(ctx, "tenant-limits", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected LimitRange tenant-limits to exist, got error: %v", err)
	}

	// 6. Verify standard NetworkPolicies were created
	npList, err := clientset.NetworkingV1().NetworkPolicies(namespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list network policies: %v", err)
	}
	if len(npList.Items) != 2 {
		t.Errorf("expected 2 network policies, got %d", len(npList.Items))
	}

	// 7. Verify CiliumNetworkPolicy was created using dynamic client
	gvr := schema.GroupVersionResource{
		Group:    "cilium.io",
		Version:  "v2",
		Resource: "ciliumnetworkpolicies",
	}
	cnpList, err := dynamicClient.Resource(gvr).Namespace(namespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list cilium network policies: %v", err)
	}
	if len(cnpList.Items) != 1 {
		t.Errorf("expected 1 cilium network policy, got %d", len(cnpList.Items))
	}

	// 8. Test List tenants
	tenants, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List tenants returned error: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("expected 1 tenant listed, got %d", len(tenants))
	}
	if tenants[0].Name != tenantName {
		t.Errorf("expected listed tenant name %s, got %s", tenantName, tenants[0].Name)
	}

	// 9. Test Remove tenant
	err = mgr.Remove(ctx, tenantName)
	if err != nil {
		t.Fatalf("Remove tenant returned error: %v", err)
	}

	// Verify namespace deletion was requested
	_, err = clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err == nil {
		t.Errorf("expected namespace %s to be deleted, but it still exists", namespaceName)
	}
}

func TestFindTemplatePath_InTests(t *testing.T) {
	// Let's create a temporary file and see if findTemplatePath can find it
	tmpFile, err := os.CreateTemp("", "bifrost-find-path-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// We are looking for something in standard paths, but let's test expandPath / findTemplatePath
	_, err = findTemplatePath("non-existent-template-xyz.yaml")
	if err == nil {
		t.Error("expected error for non-existent template, got nil")
	}
}
