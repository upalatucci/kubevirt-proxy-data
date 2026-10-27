package multicluster

import (
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	ClusterQueryParam = "cluster"

	defaultClusterProxyHost = "cluster-proxy-addon-user.multicluster-engine.svc.cluster.local"
	defaultClusterProxyPort = "9092"
)

// HubAPIServerHostResolver returns the hub Kubernetes API host (no scheme).
// handlers sets this to read APIServerURL so tests and runtime share one source of truth.
var HubAPIServerHostResolver = func() string {
	host := os.Getenv("KUBE_API_SERVER")
	if host == "" {
		host = "kubernetes.default.svc"
	}
	return host
}

func hubAPIServerHost() string {
	return HubAPIServerHostResolver()
}

// ClusterFromRequest resolves the target managed cluster from the query string or request field.
func ClusterFromRequest(r *http.Request, cluster string) string {
	if cluster != "" {
		return cluster
	}
	return strings.TrimSpace(r.URL.Query().Get(ClusterQueryParam))
}

// UseClusterProxy reports whether outbound calls should use the cluster-proxy addon.
func UseClusterProxy(cluster string, req *http.Request) bool {
	if os.Getenv("CLUSTER_PROXY_ENABLED") == "false" {
		return false
	}
	return strings.TrimSpace(cluster) != "" && !IsHubCluster(cluster, req)
}

// RewriteProxyRequest points a pass-through Kubernetes API request at the hub API server or cluster-proxy.
func RewriteProxyRequest(req *http.Request, cluster string) {
	req.URL.Scheme = "https"
	if !UseClusterProxy(cluster, req) {
		if req.URL.Host == "" {
			req.URL.Host = hubAPIServerHost()
		}
		return
	}

	host, port := clusterProxyHostPort()
	req.URL.Host = net.JoinHostPort(host, port)
	req.URL.Path = "/" + cluster + req.URL.Path
	req.Header.Set("Origin", "https://"+host)
}

// KubernetesURL builds an absolute URL for a Kubernetes API path on the hub or a managed cluster.
func KubernetesURL(cluster string, apiPath string, req *http.Request) string {
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	if !UseClusterProxy(cluster, req) {
		return "https://" + hubAPIServerHost() + apiPath
	}
	host, port := clusterProxyHostPort()
	return "https://" + net.JoinHostPort(host, port) + "/" + cluster + apiPath
}

func clusterProxyHostPort() (string, string) {
	host := os.Getenv("CLUSTER_PROXY_ADDON_HOST")
	if host == "" {
		host = defaultClusterProxyHost
	}
	port := os.Getenv("CLUSTER_PROXY_ADDON_PORT")
	if port == "" {
		port = defaultClusterProxyPort
	}
	return host, port
}
