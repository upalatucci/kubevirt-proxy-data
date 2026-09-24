package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

var APIServerURL = getEnvOrDefault("KUBE_API_SERVER", "kubernetes.default.svc")

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func APIServerBaseURL() string {
	return "https://" + APIServerURL
}

func NewK8sHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func ForwardAuthHeaders(from *http.Request, to *http.Request) {
	for key, values := range from.Header {
		upperKey := strings.ToUpper(key)
		if upperKey == "AUTHORIZATION" || strings.HasPrefix(upperKey, "IMPERSONATE-") {
			for _, value := range values {
				to.Header.Add(key, value)
			}
		}
	}
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
