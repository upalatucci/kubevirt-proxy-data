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
	k8sHTTPClient     *http.Client
	k8sHTTPClientOnce sync.Once
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

	caPEM, caErr := os.ReadFile(caPath)
	if caErr == nil {
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM(caPEM) {
			cfg.RootCAs = pool
			return cfg
		}

		log.Printf("Warning: service account CA at %s is present but could not be parsed", caPath)
		if os.Getenv("KUBE_TLS_INSECURE") == "true" {
			cfg.InsecureSkipVerify = true
		}
		return cfg
	}

	if os.Getenv("APP_ENV") == "dev" || os.Getenv("KUBE_TLS_INSECURE") == "true" {
		cfg.InsecureSkipVerify = true
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
