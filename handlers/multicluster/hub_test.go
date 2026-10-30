package multicluster

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLocalHubNameFromACMBackend(t *testing.T) {
	resetHubNameCache()
	t.Setenv("ACM_CONSOLE_HUB_URL", "")
	HubClusterNameOverride = ""
	HubAPIServerHostResolver = func() string { return "kubernetes.default.svc" }

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hub" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"localHubName": "my-hub"})
	}))
	defer backend.Close()

	t.Setenv("ACM_CONSOLE_BACKEND_URL", backend.URL)
	t.Setenv("ACM_HUB_CONFIG_CACHE_TTL", "1h")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := LocalHubName(req); got != "my-hub" {
		t.Fatalf("expected my-hub, got %q", got)
	}

	// Cached: stop backend and read again.
	backend.Close()
	if got := LocalHubName(req); got != "my-hub" {
		t.Fatalf("expected cached my-hub, got %q", got)
	}
}

func TestIsHubClusterUsesDiscoveredName(t *testing.T) {
	HubClusterNameOverride = "fleet-local"
	defer func() { HubClusterNameOverride = "" }()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !IsHubCluster("", req) {
		t.Fatal("empty cluster should be hub")
	}
	if !IsHubCluster("fleet-local", req) {
		t.Fatal("fleet-local should match hub")
	}
	if IsHubCluster("spoke-1", req) {
		t.Fatal("spoke-1 should not be hub")
	}
}

func TestHubConfigCacheTTLDefault(t *testing.T) {
	t.Setenv("ACM_HUB_CONFIG_CACHE_TTL", "")
	if hubConfigCacheTTL() != defaultHubConfigCacheTTL {
		t.Fatalf("expected default %v", defaultHubConfigCacheTTL)
	}
}

func TestHubConfigCacheTTLCustom(t *testing.T) {
	t.Setenv("ACM_HUB_CONFIG_CACHE_TTL", "2h")
	if hubConfigCacheTTL() != 2*time.Hour {
		t.Fatalf("expected 2h")
	}
}

func TestIsHubClusterFallsBackToLocalClusterWhenDiscoveryFails(t *testing.T) {
	resetHubNameCache()
	HubClusterNameOverride = ""
	t.Setenv("ACM_LOCAL_HUB_CLUSTER_NAME", "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !IsHubCluster("local-cluster", req) {
		t.Fatal("local-cluster should route to hub when hub name is unknown")
	}
	if IsHubCluster("bm07", req) {
		t.Fatal("bm07 should not be treated as hub when discovery fails")
	}
}

func TestLocalHubNameCachesDiscoveryFailure(t *testing.T) {
	resetHubNameCache()
	HubClusterNameOverride = ""
	t.Setenv("ACM_LOCAL_HUB_CLUSTER_NAME", "")
	t.Setenv("ACM_CONSOLE_HUB_URL", "http://127.0.0.1:1/hub")
	t.Setenv("ACM_HUB_DISCOVERY_FAILURE_CACHE_TTL", "10m")
	HubAPIServerHostResolver = func() string { return "127.0.0.1:1" }
	K8sHTTPClientProvider = func() *http.Client {
		return &http.Client{Timeout: 200 * time.Millisecond}
	}
	defer func() { K8sHTTPClientProvider = func() *http.Client { return http.DefaultClient } }()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := LocalHubName(req); got != "" {
		t.Fatalf("expected empty hub name on failure, got %q", got)
	}
	if got := LocalHubName(req); got != "" {
		t.Fatalf("expected cached failure, got %q", got)
	}
}

func TestConfiguredHubClusterNameFromEnv(t *testing.T) {
	resetHubNameCache()
	t.Setenv("ACM_LOCAL_HUB_CLUSTER_NAME", "static-hub")
	defer t.Setenv("ACM_LOCAL_HUB_CLUSTER_NAME", "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := LocalHubName(req); got != "static-hub" {
		t.Fatalf("expected static-hub, got %q", got)
	}
	if !IsHubCluster("static-hub", req) {
		t.Fatal("static-hub should be hub")
	}
}
