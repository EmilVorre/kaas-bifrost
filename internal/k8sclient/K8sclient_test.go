package k8sclient

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIsDaemonSet(t *testing.T) {
	tests := []struct {
		name string
		pod  v1.Pod
		want bool
	}{
		{
			name: "is a daemonset",
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "DaemonSet",
							Name: "cilium-agent",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "is not a daemonset",
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "ReplicaSet",
							Name: "nginx-deployment-abc",
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDaemonSet(tt.pod)
			if got != tt.want {
				t.Errorf("isDaemonSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMirrorPod(t *testing.T) {
	tests := []struct {
		name string
		pod  v1.Pod
		want bool
	}{
		{
			name: "is a mirror pod",
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubernetes.io/config.mirror": "some-value",
					},
				},
			},
			want: true,
		},
		{
			name: "is not a mirror pod",
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"other-annotation": "value",
					},
				},
			},
			want: false,
		},
		{
			name: "no annotations",
			pod:  v1.Pod{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMirrorPod(tt.pod)
			if got != tt.want {
				t.Errorf("isMirrorPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "expand tilde prefix",
			path: "~/.bifrost/config",
			want: filepath.Join(home, ".bifrost", "config"),
		},
		{
			name: "only tilde",
			path: "~",
			want: home,
		},
		{
			name: "no tilde",
			path: "/etc/kubernetes/admin.conf",
			want: "/etc/kubernetes/admin.conf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("expandPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientMethodsWithFake(t *testing.T) {
	ctx := context.Background()

	// 1. Prepare fake objects
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Status: v1.NodeStatus{
			Addresses: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "10.0.0.10",
				},
			},
		},
	}

	podToDelete := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-pod",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node-1",
		},
	}

	dsPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "DaemonSet",
					Name: "kube-proxy",
				},
			},
		},
		Spec: v1.PodSpec{
			NodeName: "node-1",
		},
	}

	// 2. Initialize fake client
	clientset := fake.NewSimpleClientset(node, podToDelete, dsPod)
	client := &Client{Clientset: clientset}

	// 3. Test FindNodeNameByIP
	nodeName, err := client.FindNodeNameByIP(ctx, "10.0.0.10")
	if err != nil {
		t.Fatalf("unexpected error finding node by IP: %v", err)
	}
	if nodeName != "node-1" {
		t.Errorf("expected node-1, got %s", nodeName)
	}

	// Test FindNodeNameByIP with non-existent IP
	_, err = client.FindNodeNameByIP(ctx, "10.0.0.99")
	if err == nil {
		t.Fatal("expected error for non-existent IP, got nil")
	}

	// 4. Test CordonNode
	err = client.CordonNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("unexpected error cordoning node: %v", err)
	}

	updatedNode, err := clientset.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if !updatedNode.Spec.Unschedulable {
		t.Error("expected node to be unschedulable")
	}

	// 5. Test DrainNode
	err = client.DrainNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("unexpected error draining node: %v", err)
	}

	// The regular pod should be deleted, but daemonset pod should remain
	_, err = clientset.CoreV1().Pods("default").Get(ctx, "nginx-pod", metav1.GetOptions{})
	if err == nil {
		t.Error("expected podToDelete to be deleted, but it exists")
	}

	_, err = clientset.CoreV1().Pods("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected daemonset pod kube-proxy to remain, but got error: %v", err)
	}

	// 6. Test DeleteNode
	err = client.DeleteNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("unexpected error deleting node: %v", err)
	}

	_, err = clientset.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err == nil {
		t.Error("expected node to be deleted, but it exists")
	}
}
