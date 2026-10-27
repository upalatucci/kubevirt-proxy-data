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

	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/util"
)

const (
	defaultACMConsoleBackendURL = "http://multicluster-console-chart-v2.open-cluster-management.svc.cluster.local:9000"
	defaultHubConfigCacheTTL    = 24 * time.Hour

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
	expiresAt time.Time
}

var cachedHubName hubNameCache

func resetHubNameCache() {
	cachedHubName.mu.Lock()
	cachedHubName.name = ""
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
	if cachedHubName.name != "" && now.Before(cachedHubName.expiresAt) {
		name := cachedHubName.name
		cachedHubName.mu.RUnlock()
		return name
	}
	cachedHubName.mu.RUnlock()

	name, err := discoverLocalHubName(req)
	if err != nil {
		log.Printf("Failed to discover ACM local hub cluster name: %v", err)
		return ""
	}
	if name == "" {
		return ""
	}

	cachedHubName.mu.Lock()
	cachedHubName.name = name
	cachedHubName.expiresAt = now.Add(hubConfigCacheTTL())
	cachedHubName.mu.Unlock()
	return name
}

// IsHubCluster reports whether the cluster selector targets the hub API server (not cluster-proxy).
func IsHubCluster(cluster string, req *http.Request) bool {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return true
	}
	hub := LocalHubName(req)
	if hub == "" {
		return false
	}
	return strings.EqualFold(cluster, hub)
}

func discoverLocalHubName(req *http.Request) (string, error) {
	ctx := context.Background()
	if req != nil && req.Context() != nil {
		ctx = req.Context()
	}

	name, err := localHubNameFromACMBackend(ctx, req)
	if err == nil && name != "" {
		return name, nil
	}
	if err != nil {
		log.Printf("ACM console /hub lookup failed, falling back to ManagedCluster list: %v", err)
	}
	return localHubNameFromManagedClusters(ctx, req)
}

func localHubNameFromACMBackend(ctx context.Context, incomingReq *http.Request) (string, error) {
	url := acmHubConfigurationURL()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if incomingReq != nil {
		util.ForwardAuthHeaders(incomingReq, request)
	}
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
	if incomingReq != nil {
		util.ForwardAuthHeaders(incomingReq, request)
	}
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
