package bao

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProvisionTenant(t *testing.T) {
	tenantName := "example"
	tenantNamespace := "tenant-example"

	// Track API calls
	var policyCreated, roleCreated bool

	// Start mock OpenBao HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && r.URL.Path == "/v1/sys/policies/acl/tenant-"+tenantName:
			policyCreated = true
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}
			var policyReq map[string]interface{}
			if err := json.Unmarshal(body, &policyReq); err != nil {
				t.Fatalf("failed to unmarshal policy request: %v", err)
			}
			policyContent, ok := policyReq["policy"].(string)
			if !ok || !strings.Contains(policyContent, "secret/data/customers/"+tenantName+"/*") {
				t.Errorf("policy content does not target correct paths, got: %s", policyContent)
			}
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "PUT" && r.URL.Path == "/v1/auth/kubernetes/role/tenant-"+tenantName:
			roleCreated = true
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}
			var roleReq map[string]interface{}
			if err := json.Unmarshal(body, &roleReq); err != nil {
				t.Fatalf("failed to unmarshal role request: %v", err)
			}

			// Verify bound service accounts and namespaces
			saNames, _ := roleReq["bound_service_account_names"].([]interface{})
			saNamespaces, _ := roleReq["bound_service_account_namespaces"].([]interface{})

			if len(saNames) != 1 || saNames[0] != "tenant-admin" {
				t.Errorf("unexpected bound service accounts: %v", saNames)
			}
			if len(saNamespaces) != 1 || saNamespaces[0] != tenantNamespace {
				t.Errorf("unexpected bound service account namespaces: %v", saNamespaces)
			}
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Instantiate our wrapper pointing to mock server
	client, err := New(server.URL, "test-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.ProvisionTenant(tenantName, tenantNamespace)
	if err != nil {
		t.Fatalf("ProvisionTenant failed: %v", err)
	}

	if !policyCreated {
		t.Error("expected policy creation request, but none occurred")
	}
	if !roleCreated {
		t.Error("expected kubernetes auth role creation request, but none occurred")
	}
}

func TestDeprovisionTenant(t *testing.T) {
	tenantName := "example"

	// Track API calls
	var roleDeleted, policyDeleted, secretsDeleted bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE" && r.URL.Path == "/v1/auth/kubernetes/role/tenant-"+tenantName:
			roleDeleted = true
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "DELETE" && r.URL.Path == "/v1/sys/policies/acl/tenant-"+tenantName:
			policyDeleted = true
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "DELETE" && r.URL.Path == "/v1/secret/metadata/customers/"+tenantName:
			secretsDeleted = true
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = client.DeprovisionTenant(tenantName)
	if err != nil {
		t.Fatalf("DeprovisionTenant failed: %v", err)
	}

	if !roleDeleted {
		t.Error("expected role deletion request, but none occurred")
	}
	if !policyDeleted {
		t.Error("expected policy deletion request, but none occurred")
	}
	if !secretsDeleted {
		t.Error("expected secrets metadata deletion request, but none occurred")
	}
}

func TestSealStatus(t *testing.T) {
	tests := []struct {
		name         string
		sealedResult bool
		responseJSON string
	}{
		{
			name:         "Sealed",
			sealedResult: true,
			responseJSON: `{"sealed": true, "threshold": 3, "shares": 5, "progress": 0}`,
		},
		{
			name:         "Unsealed",
			sealedResult: false,
			responseJSON: `{"sealed": false, "threshold": 3, "shares": 5, "progress": 0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/v1/sys/seal-status" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(tt.responseJSON))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			client, err := New(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			isSealed, err := client.SealStatus()
			if err != nil {
				t.Fatalf("SealStatus failed: %v", err)
			}

			if isSealed != tt.sealedResult {
				t.Errorf("expected SealStatus to be %v, got: %v", tt.sealedResult, isSealed)
			}
		})
	}
}
