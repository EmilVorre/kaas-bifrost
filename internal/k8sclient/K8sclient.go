// Package k8sclient provides a wrapper around client-go for interacting
// with the kubernetes API server. All other internal packages use this
// as their single point of contact with the cluster.
package k8sclient

import (
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
)

// Client warps the kubernetes clientset
type Client struct {
	Clientset kubernetes.Interface
}

// New creates a new Client from the kubeconfig path
func New(kubeconfigPath string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{Clientset: clientset}, nil
}