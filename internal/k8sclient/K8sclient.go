// Package k8sclient provides a wrapper around client-go for interacting
// with the kubernetes API server. All other internal packages use this
// as their single point of contact with the cluster.
package k8sclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client warps the kubernetes clientset
type Client struct {
	Clientset kubernetes.Interface
}

// New creates a new Client from the kubeconfig path
func New(kubeconfigPath string) (*Client, error) {
	expandedPath, err := expandPath(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("expand kubeconfig path: %w", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", expandedPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{Clientset: clientset}, nil
}

// FindNodeNameByIP finds the node name by matching the internal IP address.
func (c *Client) FindNodeNameByIP(ctx context.Context, ip string) (string, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP && addr.Address == ip {
				return node.Name, nil
			}
		}
	}
	return "", fmt.Errorf("node with IP %s not found in cluster", ip)
}

// CordonNode marks a node as unschedulable.
func (c *Client) CordonNode(ctx context.Context, name string) error {
	payload := []byte(`{"spec":{"unschedulable":true}}`)
	_, err := c.Clientset.CoreV1().Nodes().Patch(ctx, name, types.MergePatchType, payload, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patch node %s: %w", name, err)
	}
	return nil
}

// DrainNode deletes all non-daemonset, non-mirror pods on a node and waits for them to terminate.
func (c *Client) DrainNode(ctx context.Context, name string) error {
	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		return fmt.Errorf("list pods on node %s: %w", name, err)
	}

	var podsToDelete []v1.Pod
	for _, pod := range pods.Items {
		if isDaemonSet(pod) || isMirrorPod(pod) {
			continue
		}
		podsToDelete = append(podsToDelete, pod)
	}

	if len(podsToDelete) == 0 {
		return nil
	}

	for _, pod := range podsToDelete {
		err := c.Clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
	}

	// Wait for pods to be completely deleted
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for pods to terminate on node %s", name)
		case <-ticker.C:
			remainingPods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
				FieldSelector: "spec.nodeName=" + name,
			})
			if err != nil {
				return fmt.Errorf("list remaining pods: %w", err)
			}

			activeCount := 0
			for _, pod := range remainingPods.Items {
				if isDaemonSet(pod) || isMirrorPod(pod) {
					continue
				}
				activeCount++
			}

			if activeCount == 0 {
				return nil
			}
		}
	}
}

// DeleteNode deletes a node from the cluster.
func (c *Client) DeleteNode(ctx context.Context, name string) error {
	err := c.Clientset.CoreV1().Nodes().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete node %s: %w", name, err)
	}
	return nil
}

func isDaemonSet(pod v1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func isMirrorPod(pod v1.Pod) bool {
	if pod.Annotations != nil {
		_, ok := pod.Annotations["kubernetes.io/config.mirror"]
		return ok
	}
	return false
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
