package multicluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultACMConsoleBackendURL = "https://console-chart-console-v2.open-cluster-management.svc:3000"
	defaultHubConfigCacheTTL    = 24 * time.Hour
	defaultHubDiscoveryFailureCacheTTL = time.Minute

	// ClusterQueryParam is a legacy routing query parameter still sent by some clients; it must not
	// be treated as a list filter key.
	ClusterQueryParam = "cluster"

	// commonLocalHubClusterName is the default hub ManagedCluster name on many OCM installs when
	// discovery fails (for example the caller cannot list ManagedCluster objects).
	commonLocalHubClusterName = "local-cluster"

	managedClustersAPIPath = "/apis/cluster.open-cluster-management.io/v1/managedclusters"
	localClusterLabelQuery = "labelSelector=local-cluster%3Dtrue"
)

// K8sHTTPClientProvider supplies the HTTP client used to reach the hub API and ACM console backend.
var K8sHTTPClientProvider = func() *http.Client {
	return http.DefaultClient
}

// HubClusterNameOverride is set in tests to skip ACM hub discovery.
var HubClusterNameOverride string

type hubConfigurationResponse struct {
	LocalHubName string `json:"localHubName"`
}

type hubNameCache struct {
	mu        sync.RWMutex
	name      string
	failed    bool
	expiresAt time.Time
}

var cachedHubName hubNameCache

func resetHubNameCache() {
	cachedHubName.mu.Lock()
	cachedHubName.name = ""
	cachedHubName.failed = false
	cachedHubName.expiresAt = time.Time{}
	cachedHubName.mu.Unlock()
}

// LocalHubName returns the ACM hub ManagedCluster name (same as useHubClusterName / GET /hub localHubName).
func LocalHubName(req *http.Request) string {
	if override := strings.TrimSpace(HubClusterNameOverride); override != "" {
		return override
	}

	now := time.Now()
	cachedHubName.mu.RLock()
	if now.Before(cachedHubName.expiresAt) {
		if cachedHubName.failed {
			cachedHubName.mu.RUnlock()
			return ""
		}
		if cachedHubName.name != "" {
			name := cachedHubName.name
			cachedHubName.mu.RUnlock()
			return name
		}
	}
	cachedHubName.mu.RUnlock()

	name, err := discoverLocalHubName(req)
	if err != nil {
		log.Printf("Failed to discover ACM local hub cluster name: %v", err)
		cacheHubDiscoveryFailure(now)
		return ""
	}
	if name == "" {
		cacheHubDiscoveryFailure(now)
		return ""
	}

	cachedHubName.mu.Lock()
	cachedHubName.name = name
	cachedHubName.failed = false
	cachedHubName.expiresAt = now.Add(hubConfigCacheTTL())
	cachedHubName.mu.Unlock()
	return name
}

func cacheHubDiscoveryFailure(now time.Time) {
	cachedHubName.mu.Lock()
	cachedHubName.name = ""
	cachedHubName.failed = true
	cachedHubName.expiresAt = now.Add(hubDiscoveryFailureCacheTTL())
	cachedHubName.mu.Unlock()
}

// IsHubCluster reports whether the cluster selector targets the hub API server (not cluster-proxy).
func IsHubCluster(cluster string, req *http.Request) bool {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return true
	}
	hub := LocalHubName(req)
	if hub != "" {
		return strings.EqualFold(cluster, hub)
	}
	return strings.EqualFold(cluster, commonLocalHubClusterName)
}

func discoverLocalHubName(req *http.Request) (string, error) {
	ctx := context.Background()
	if req != nil && req.Context() != nil {
		ctx = req.Context()
	}

	// Prefer the hub ManagedCluster CR when developing against a remote API (oc login);
	// ACM console /hub is often only reachable in-cluster.
	if name, err := localHubNameFromManagedClusters(ctx, req); err == nil && name != "" {
		return name, nil
	} else if err != nil {
		log.Printf("ManagedCluster hub lookup failed, trying ACM console /hub: %v", err)
	}

	name, err := localHubNameFromACMBackend(ctx, req)
	if err == nil && name != "" {
		return name, nil
	}
	if err != nil {
		return "", err
	}
	return localHubNameFromManagedClusters(ctx, req)
}

func localHubNameFromACMBackend(ctx context.Context, incomingReq *http.Request) (string, error) {
	url := acmHubConfigurationURL()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	setHubDiscoveryAuth(incomingReq, request)
	request.Header.Set("Accept", "application/json")

	resp, err := K8sHTTPClientProvider().Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ACM console /hub returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded hubConfigurationResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	return strings.TrimSpace(decoded.LocalHubName), nil
}

func localHubNameFromManagedClusters(ctx context.Context, incomingReq *http.Request) (string, error) {
	url := "https://" + hubAPIServerHost() + managedClustersAPIPath + "?" + localClusterLabelQuery
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	setHubDiscoveryAuth(incomingReq, request)
	request.Header.Set("Accept", "application/json")

	resp, err := K8sHTTPClientProvider().Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("managedclusters list returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return "", err
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("no ManagedCluster with local-cluster=true label found")
	}
	return strings.TrimSpace(list.Items[0].Metadata.Name), nil
}

func acmHubConfigurationURL() string {
	if override := strings.TrimSpace(os.Getenv("ACM_CONSOLE_HUB_URL")); override != "" {
		return override
	}
	base := strings.TrimSpace(os.Getenv("ACM_CONSOLE_BACKEND_URL"))
	if base == "" {
		base = defaultACMConsoleBackendURL
	}
	return strings.TrimRight(base, "/") + "/hub"
}

func hubConfigCacheTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ACM_HUB_CONFIG_CACHE_TTL"))
	if raw == "" {
		return defaultHubConfigCacheTTL
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < time.Minute {
		return defaultHubConfigCacheTTL
	}
	return parsed
}

func hubDiscoveryFailureCacheTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ACM_HUB_DISCOVERY_FAILURE_CACHE_TTL"))
	if raw == "" {
		return defaultHubDiscoveryFailureCacheTTL
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < time.Second {
		return defaultHubDiscoveryFailureCacheTTL
	}
	return parsed
}
