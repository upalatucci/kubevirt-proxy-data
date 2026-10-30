package multicluster

import (
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	ClusterPathSegment = "cluster"

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

// ParseProxyPath extracts an optional /api/cluster/{name}/ or /apis/cluster/{name}/ prefix.
// It returns the cluster name and the Kubernetes API path forwarded upstream (/api/... or /apis/...).
func ParseProxyPath(path string) (cluster string, kubernetesPath string) {
	if path == "" {
		return "", path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", "/"
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[1] != ClusterPathSegment {
		return "", path
	}

	cluster = strings.TrimSpace(parts[2])
	rest := append([]string{parts[0]}, parts[3:]...)
	if len(rest) == 1 && rest[0] == parts[0] {
		return cluster, "/" + parts[0]
	}
	return cluster, "/" + strings.Join(rest, "/")
}

// ClusterFromRequest resolves the target cluster from an explicit value or the /cluster/{name}/ path prefix.
func ClusterFromRequest(r *http.Request, explicitCluster string) string {
	if cluster := strings.TrimSpace(explicitCluster); cluster != "" {
		return cluster
	}
	cluster, _ := ParseProxyPath(r.URL.Path)
	return cluster
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
