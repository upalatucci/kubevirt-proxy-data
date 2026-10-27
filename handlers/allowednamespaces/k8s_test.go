package allowednamespaces

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers"
)

func TestListAccessibleNamespacesFallsBackToOpenShiftProjects(t *testing.T) {
	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case namespacesAPIPath:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"forbidden"}`))
		case openshiftProjectsAPIPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"openshift"}}]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer apiServer.Close()

	originalAPIServerURL := handlers.APIServerURL
	handlers.APIServerURL = strings.TrimPrefix(apiServer.URL, "https://")
	t.Cleanup(func() {
		handlers.APIServerURL = originalAPIServerURL
	})

	client := handlers.NewK8sHTTPClient()
	req := httptest.NewRequest(http.MethodPost, "/apis/allowednamespaces", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	namespaces, err := listAccessibleNamespaces(client, req, "")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	if len(namespaces) != 1 || namespaces[0] != "openshift" {
		t.Fatalf("expected [openshift], got %v", namespaces)
	}
}
