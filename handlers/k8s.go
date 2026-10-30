package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers/multicluster"
)

const (
	defaultServiceAccountCAPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	defaultK8sTimeout              = 30 * time.Second
	defaultK8sIdleConnTimeout      = 90 * time.Second
	defaultK8sTLSHandshakeTimeout  = 10 * time.Second
)

var (
	k8sHTTPClient              *http.Client
	k8sHTTPClientOnce          sync.Once
	clusterProxyHTTPClient     *http.Client
	clusterProxyHTTPClientOnce sync.Once
)

var APIServerURL = getEnvOrDefault("KUBE_API_SERVER", "kubernetes.default.svc")

func init() {
	multicluster.HubAPIServerHostResolver = func() string { return APIServerURL }
	multicluster.K8sHTTPClientProvider = NewK8sHTTPClient
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func APIServerBaseURL() string {
	return "https://" + APIServerURL
}

// NewK8sHTTPClient returns the shared Kubernetes API HTTP client for this process.
func NewK8sHTTPClient() *http.Client {
	k8sHTTPClientOnce.Do(func() {
		k8sHTTPClient = &http.Client{
			Timeout: defaultK8sTimeout,
			Transport: &http.Transport{
				TLSClientConfig:     k8sTLSConfig(),
				TLSHandshakeTimeout: defaultK8sTLSHandshakeTimeout,
				IdleConnTimeout:     defaultK8sIdleConnTimeout,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	})

	return k8sHTTPClient
}

// NewClusterProxyHTTPClient returns an HTTP client for the ACM cluster-proxy addon.
// The addon serving certificate is not signed by the Kubernetes API CA; TLS verification
// is skipped unless CLUSTER_PROXY_CA_PATH is set and CLUSTER_PROXY_TLS_INSECURE=false.
func NewClusterProxyHTTPClient() *http.Client {
	clusterProxyHTTPClientOnce.Do(func() {
		tlsCfg := &tls.Config{}
		if path := strings.TrimSpace(os.Getenv("CLUSTER_PROXY_CA_PATH")); path != "" {
			pool := x509.NewCertPool()
			if pem, err := os.ReadFile(path); err == nil && pool.AppendCertsFromPEM(pem) {
				tlsCfg.RootCAs = pool
			}
		}
		if tlsCfg.RootCAs == nil || os.Getenv("CLUSTER_PROXY_TLS_INSECURE") != "false" {
			tlsCfg.InsecureSkipVerify = true
		}
		clusterProxyHTTPClient = &http.Client{
			Timeout: defaultK8sTimeout,
			Transport: &http.Transport{
				TLSClientConfig:     tlsCfg,
				TLSHandshakeTimeout: defaultK8sTLSHandshakeTimeout,
				IdleConnTimeout:     defaultK8sIdleConnTimeout,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	})
	return clusterProxyHTTPClient
}

// K8sHTTPClientForCluster uses the cluster-proxy client for managed clusters and the hub client otherwise.
func K8sHTTPClientForCluster(cluster string, req *http.Request) *http.Client {
	if multicluster.UseClusterProxy(cluster, req) {
		return NewClusterProxyHTTPClient()
	}
	return NewK8sHTTPClient()
}

func serviceAccountCAPath() string {
	if path := os.Getenv("SERVICE_ACCOUNT_CA_PATH"); path != "" {
		return path
	}
	return defaultServiceAccountCAPath
}

func k8sTLSConfig() *tls.Config {
	return k8sTLSConfigFromPath(serviceAccountCAPath())
}

func k8sTLSConfigFromPath(caPath string) *tls.Config {
	cfg := &tls.Config{}
	pool := x509.NewCertPool()
	hasRoots := false

	caPEM, caErr := os.ReadFile(caPath)
	if caErr == nil {
		if pool.AppendCertsFromPEM(caPEM) {
			hasRoots = true
		} else {
			log.Printf("Warning: service account CA at %s is present but could not be parsed", caPath)
		}
	}

	if proxyCAPath := strings.TrimSpace(os.Getenv("CLUSTER_PROXY_CA_PATH")); proxyCAPath != "" {
		proxyCAPEM, err := os.ReadFile(proxyCAPath)
		if err != nil {
			log.Printf("Warning: could not read CLUSTER_PROXY_CA_PATH %s: %v", proxyCAPath, err)
		} else if pool.AppendCertsFromPEM(proxyCAPEM) {
			hasRoots = true
		} else {
			log.Printf("Warning: could not parse CLUSTER_PROXY_CA_PATH %s", proxyCAPath)
		}
	}

	if hasRoots {
		cfg.RootCAs = pool
	}

	if caErr != nil {
		if os.Getenv("APP_ENV") == "dev" || os.Getenv("KUBE_TLS_INSECURE") == "true" {
			cfg.InsecureSkipVerify = true
		}
		return cfg
	}

	if !hasRoots {
		if os.Getenv("KUBE_TLS_INSECURE") == "true" {
			cfg.InsecureSkipVerify = true
		}
		return cfg
	}

	return cfg
}

func HasAuthHeader(req *http.Request) bool {
	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if auth == "" {
		return false
	}

	parts := strings.Fields(auth)
	if len(parts) < 2 || parts[1] == "" {
		return false
	}

	return true
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kubernetes API returned status %d: %s", e.StatusCode, e.Message)
}

func StatusMessage(body []byte) string {
	var status struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &status); err == nil && status.Message != "" {
		return status.Message
	}

	return string(body)
}
