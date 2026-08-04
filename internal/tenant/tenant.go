// Package tenant manages the lifecycle of tenant namespaces on the cluster.
// It handles namespace creation, RBAC, resource quotas, and network policies.
package tenant

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/EmilVorre/Bifrost/internal/k8sclient"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// QuotaTier defines the resource quota size for a tenant
type QuotaTier string

const (
	QuotaSmall  QuotaTier = "small"
	QuotaMedium QuotaTier = "medium"
	QuotaLarge  QuotaTier = "large"
)

// Tenant represents a single tenant on the platform
type Tenant struct {
	Name      string
	Namespace string
	Quota     QuotaTier
}

// Manager handles tenant provisioning and teardown
type Manager struct {
	client *k8sclient.Client
}

// NewManager creates a new tenant Manager
func NewManager(client *k8sclient.Client) *Manager {
	return &Manager{client: client}
}

type templateData struct {
	Namespace string
}

// Add provisions a new tenant namespace with RBAC, quotas, and network policies
func (m *Manager) Add(ctx context.Context, name string, quota QuotaTier) (*Tenant, error) {
	namespaceName := "tenant-" + name

	// 1. Create Namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				"kaas-bifrost.dev/tenant": name,
				"kaas-bifrost.dev/quota":  string(quota),
			},
		},
	}
	_, err := m.client.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create namespace: %w", err)
	}

	// 2. Create RBAC (ServiceAccount, Role, RoleBinding)
	saName := "tenant-admin"
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespaceName,
		},
	}
	_, err = m.client.Clientset.CoreV1().ServiceAccounts(namespaceName).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create service account: %w", err)
	}

	roleName := "tenant-admin-role"
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespaceName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "apps", "networking.k8s.io", "autoscaling", "batch"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
	_, err = m.client.Clientset.RbacV1().Roles(namespaceName).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create tenant role: %w", err)
	}

	rbName := "tenant-admin-binding"
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: namespaceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespaceName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err = m.client.Clientset.RbacV1().RoleBindings(namespaceName).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create role binding: %w", err)
	}

	// 3. Create ResourceQuota
	limits := map[v1.ResourceName]resource.Quantity{}
	switch quota {
	case QuotaSmall:
		limits[v1.ResourceRequestsCPU] = resource.MustParse("2")
		limits[v1.ResourceRequestsMemory] = resource.MustParse("4Gi")
		limits[v1.ResourceLimitsCPU] = resource.MustParse("2")
		limits[v1.ResourceLimitsMemory] = resource.MustParse("4Gi")
		limits[v1.ResourcePods] = resource.MustParse("20")
	case QuotaLarge:
		limits[v1.ResourceRequestsCPU] = resource.MustParse("8")
		limits[v1.ResourceRequestsMemory] = resource.MustParse("16Gi")
		limits[v1.ResourceLimitsCPU] = resource.MustParse("8")
		limits[v1.ResourceLimitsMemory] = resource.MustParse("16Gi")
		limits[v1.ResourcePods] = resource.MustParse("100")
	default: // QuotaMedium
		limits[v1.ResourceRequestsCPU] = resource.MustParse("4")
		limits[v1.ResourceRequestsMemory] = resource.MustParse("8Gi")
		limits[v1.ResourceLimitsCPU] = resource.MustParse("4")
		limits[v1.ResourceLimitsMemory] = resource.MustParse("8Gi")
		limits[v1.ResourcePods] = resource.MustParse("50")
	}

	rq := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: namespaceName,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: limits,
		},
	}
	_, err = m.client.Clientset.CoreV1().ResourceQuotas(namespaceName).Create(ctx, rq, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create resource quota: %w", err)
	}

	// 4. Create LimitRange
	lr := &v1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-limits",
			Namespace: namespaceName,
		},
		Spec: v1.LimitRangeSpec{
			Limits: []v1.LimitRangeItem{
				{
					Type: v1.LimitTypeContainer,
					Default: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("1"),
						v1.ResourceMemory: resource.MustParse("1Gi"),
					},
					DefaultRequest: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("100m"),
						v1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}
	_, err = m.client.Clientset.CoreV1().LimitRanges(namespaceName).Create(ctx, lr, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create limit range: %w", err)
	}

	// 5. Parse and apply NetworkPolicies
	data := templateData{Namespace: namespaceName}

	defaultDenyPath, err := findTemplatePath("default-deny.yaml")
	if err != nil {
		return nil, err
	}
	defaultDenyPolicy, err := parseNetworkPolicyTemplate(defaultDenyPath, data)
	if err != nil {
		return nil, fmt.Errorf("parse default-deny policy: %w", err)
	}
	_, err = m.client.Clientset.NetworkingV1().NetworkPolicies(namespaceName).Create(ctx, defaultDenyPolicy, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("apply default-deny policy: %w", err)
	}

	tenantAllowPath, err := findTemplatePath("tenant-allow.yaml")
	if err != nil {
		return nil, err
	}
	tenantAllowPolicy, err := parseNetworkPolicyTemplate(tenantAllowPath, data)
	if err != nil {
		return nil, fmt.Errorf("parse tenant-allow policy: %w", err)
	}
	_, err = m.client.Clientset.NetworkingV1().NetworkPolicies(namespaceName).Create(ctx, tenantAllowPolicy, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("apply tenant-allow policy: %w", err)
	}

	// 6. Parse and apply CiliumNetworkPolicy
	ciliumL7Path, err := findTemplatePath("cilium-l7.yaml")
	if err != nil {
		return nil, err
	}
	ciliumPolicyObj, err := parseCiliumNetworkPolicyTemplate(ciliumL7Path, data)
	if err != nil {
		return nil, fmt.Errorf("parse cilium-l7 policy: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "cilium.io",
		Version:  "v2",
		Resource: "ciliumnetworkpolicies",
	}
	_, err = m.client.DynamicClient.Resource(gvr).Namespace(namespaceName).Create(ctx, ciliumPolicyObj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("apply cilium-l7 policy: %w", err)
	}

	return &Tenant{
		Name:      name,
		Namespace: namespaceName,
		Quota:     quota,
	}, nil
}

// Remove tears down all Kubernetes resources for a tenant (cascading namespace delete)
func (m *Manager) Remove(ctx context.Context, name string) error {
	namespaceName := "tenant-" + name
	err := m.client.Clientset.CoreV1().Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete namespace %s: %w", namespaceName, err)
	}
	return nil
}

// List returns all active tenants on the cluster
func (m *Manager) List(ctx context.Context) ([]Tenant, error) {
	namespaces, err := m.client.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: "kaas-bifrost.dev/tenant",
	})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var tenants []Tenant
	for _, ns := range namespaces.Items {
		tenantName := ns.Labels["kaas-bifrost.dev/tenant"]
		quota := QuotaMedium
		if q, ok := ns.Labels["kaas-bifrost.dev/quota"]; ok {
			quota = QuotaTier(q)
		}

		tenants = append(tenants, Tenant{
			Name:      tenantName,
			Namespace: ns.Name,
			Quota:     quota,
		})
	}
	return tenants, nil
}

func findTemplatePath(fileName string) (string, error) {
	paths := []string{
		filepath.Join("configs", "templates", fileName),
		filepath.Join("..", "configs", "templates", fileName),
		filepath.Join("..", "..", "configs", "templates", fileName),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("template file %s not found in search paths", fileName)
}

func parseNetworkPolicyTemplate(path string, data templateData) (*networkingv1.NetworkPolicy, error) {
	tmplBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}

	tmpl, err := template.New("policy").Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	var policy networkingv1.NetworkPolicy
	decoder := yaml.NewYAMLOrJSONDecoder(&buf, 4096)
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("decode network policy yaml: %w", err)
	}

	return &policy, nil
}

func parseCiliumNetworkPolicyTemplate(path string, data templateData) (*unstructured.Unstructured, error) {
	tmplBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}

	tmpl, err := template.New("cilium-policy").Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	var obj unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(&buf, 4096)
	if err := decoder.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode custom resource yaml: %w", err)
	}

	return &obj, nil
}
