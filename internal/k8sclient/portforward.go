package k8sclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForward forwards localPort to podPort of the specified pod in namespace.
// It returns a stop channel (to close the connection), a ready channel, and any startup error.
func (c *Client) PortForward(ctx context.Context, namespace, podName string, localPort, podPort int) (chan struct{}, chan struct{}, error) {
	// Build path
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimPrefix(c.RestConfig.Host, "https://")
	hostIP = strings.TrimPrefix(hostIP, "http://")

	serverURL := &url.URL{
		Scheme: "https",
		Path:   path,
		Host:   hostIP,
	}

	// SPDY upgrader
	transport, upgrader, err := spdy.RoundTripperFor(c.RestConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create spdy round tripper: %w", err)
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		serverURL,
	)

	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})

	ports := []string{fmt.Sprintf("%d:%d", localPort, podPort)}
	addresses := []string{"127.0.0.1"}

	pf, err := portforward.NewOnAddresses(
		dialer,
		addresses,
		ports,
		stopCh,
		readyCh,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := pf.ForwardPorts(); err != nil {
			errCh <- err
		}
	}()

	// Wait for ready or error or context timeout
	select {
	case <-readyCh:
		return stopCh, readyCh, nil
	case err := <-errCh:
		return nil, nil, fmt.Errorf("port forward failed: %w", err)
	case <-ctx.Done():
		close(stopCh)
		return nil, nil, ctx.Err()
	}
}
