package multicluster

import (
	"net/http/httptest"
	"testing"
)

func TestClusterFromRequestPrefersExplicit(t *testing.T) {
	req := httptest.NewRequest("GET", "/apis/cluster/from-path/kubevirt.io/v1/virtualmachines", nil)
	if ClusterFromRequest(req, "from-body") != "from-body" {
		t.Fatal("expected explicit cluster to win over path")
	}
}

func TestParseProxyPath(t *testing.T) {
	cluster, path := ParseProxyPath("/apis/cluster/bm07/kubevirt.io/v1/namespaces/default/virtualmachines")
	if cluster != "bm07" || path != "/apis/kubevirt.io/v1/namespaces/default/virtualmachines" {
		t.Fatalf("unexpected parse: cluster=%q path=%q", cluster, path)
	}

	cluster, path = ParseProxyPath("/api/cluster/bm07/v1/namespaces/default/persistentvolumeclaims")
	if cluster != "bm07" || path != "/api/v1/namespaces/default/persistentvolumeclaims" {
		t.Fatalf("unexpected core parse: cluster=%q path=%q", cluster, path)
	}

	cluster, path = ParseProxyPath("/apis/kubevirt.io/v1/virtualmachines")
	if cluster != "" || path != "/apis/kubevirt.io/v1/virtualmachines" {
		t.Fatalf("expected hub path unchanged, got cluster=%q path=%q", cluster, path)
	}
}

func TestUseClusterProxyHubClusterName(t *testing.T) {
	HubClusterNameOverride = "fleet-local"
	defer func() { HubClusterNameOverride = "" }()

	t.Setenv("CLUSTER_PROXY_ENABLED", "")
	req := httptest.NewRequest("GET", "/", nil)

	if UseClusterProxy("", req) {
		t.Fatal("expected empty cluster to skip cluster-proxy")
	}
	if UseClusterProxy("fleet-local", req) {
		t.Fatal("expected hub cluster name to skip cluster-proxy")
	}
	if !UseClusterProxy("managed-1", req) {
		t.Fatal("expected managed cluster to use cluster-proxy")
	}
}

func TestRewriteProxyRequestManagedCluster(t *testing.T) {
	HubClusterNameOverride = "fleet-local"
	defer func() { HubClusterNameOverride = "" }()

	t.Setenv("CLUSTER_PROXY_ADDON_HOST", "proxy.test")
	t.Setenv("CLUSTER_PROXY_ADDON_PORT", "9092")
	HubAPIServerHostResolver = func() string { return "hub.example" }

	req := httptest.NewRequest("GET", "/apis/kubevirt.io/v1/virtualmachines", nil)
	RewriteProxyRequest(req, "managed-1")

	if req.URL.Host != "proxy.test:9092" {
		t.Fatalf("unexpected host %q", req.URL.Host)
	}
	if req.URL.Path != "/managed-1/apis/kubevirt.io/v1/virtualmachines" {
		t.Fatalf("unexpected path %q", req.URL.Path)
	}
}

func TestKubernetesURLManagedCluster(t *testing.T) {
	HubClusterNameOverride = "fleet-local"
	defer func() { HubClusterNameOverride = "" }()

	t.Setenv("CLUSTER_PROXY_ADDON_HOST", "proxy.test")
	t.Setenv("CLUSTER_PROXY_ADDON_PORT", "9092")

	req := httptest.NewRequest("GET", "/", nil)
	url := KubernetesURL("managed-1", "/api/v1/namespaces", req)
	expected := "https://proxy.test:9092/managed-1/api/v1/namespaces"
	if url != expected {
		t.Fatalf("got %q want %q", url, expected)
	}
}

func TestClusterProxyDisabled(t *testing.T) {
	t.Setenv("CLUSTER_PROXY_ENABLED", "false")
	req := httptest.NewRequest("GET", "/", nil)
	if UseClusterProxy("managed-1", req) {
		t.Fatal("expected cluster-proxy to be disabled")
	}
}
