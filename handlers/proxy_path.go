package handlers

import (
	"fmt"
	"strings"

	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers/multicluster"
)

// NormalizeProxyPath strips an optional /api/cluster/{name}/ or /apis/cluster/{name}/ prefix,
// validates the Kubernetes API path, and returns the cluster name and upstream path.
func NormalizeProxyPath(path string) (cluster string, kubernetesPath string, err error) {
	cluster, kubernetesPath = multicluster.ParseProxyPath(path)
	if hasClusterPathPrefix(path) && cluster == "" {
		return "", "", fmt.Errorf("cluster name is required in /cluster/{name}/ path prefix")
	}
	if err := validateKubernetesAPIPath(kubernetesPath); err != nil {
		return "", "", err
	}
	return cluster, kubernetesPath, nil
}

// ValidateKubernetesProxyPath validates a proxy path, including an optional /cluster/{name}/ segment.
func ValidateKubernetesProxyPath(path string) error {
	_, _, err := NormalizeProxyPath(path)
	return err
}

func hasClusterPathPrefix(path string) bool {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return false
	}
	parts := strings.Split(trimmed, "/")
	return len(parts) >= 3 && parts[1] == multicluster.ClusterPathSegment
}

func validateKubernetesAPIPath(path string) error {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return fmt.Errorf("Kubernetes API path is required")
	}

	parts := strings.Split(trimmed, "/")
	switch parts[0] {
	case "api":
		versionIndex := 1
		if len(parts) >= 3 && parts[1] == multicluster.ClusterPathSegment {
			return fmt.Errorf("cluster prefix must appear immediately after /api/ or /apis/ (for example /apis/cluster/bm07/kubevirt.io/v1/...)")
		}
		if len(parts) < versionIndex+1 || !isKubernetesAPIVersion(parts[versionIndex]) {
			return fmt.Errorf("core API paths must be /api/{version}/... (for example /api/v1/namespaces/default/pods)")
		}
	case "apis":
		if len(parts) >= 2 && parts[1] == multicluster.ClusterPathSegment {
			return fmt.Errorf("cluster prefix must appear immediately after /api/ or /apis/ (for example /apis/cluster/bm07/kubevirt.io/v1/...)")
		}
		if len(parts) < 3 {
			return fmt.Errorf("named API paths must be /apis/{group}/{version}/... (for example /apis/kubevirt.io/v1/virtualmachines)")
		}
		if isKubernetesAPIVersion(parts[1]) {
			return fmt.Errorf("use /api/%s/... for core resources; /apis requires an API group before the version", parts[1])
		}
		if !isKubernetesAPIVersion(parts[2]) {
			return fmt.Errorf("named API paths must be /apis/{group}/{version}/... (for example /apis/kubevirt.io/v1/virtualmachines)")
		}
	default:
		return fmt.Errorf("proxy only supports /api/... and /apis/... Kubernetes paths")
	}
	return nil
}

func isKubernetesAPIVersion(segment string) bool {
	return strings.HasPrefix(segment, "v") && len(segment) >= 2
}
