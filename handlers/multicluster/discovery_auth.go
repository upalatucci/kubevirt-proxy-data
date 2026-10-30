package multicluster

import (
	"net/http"
	"os"
	"strings"

	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/util"
)

const defaultServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

func serviceAccountTokenPath() string {
	if path := strings.TrimSpace(os.Getenv("SERVICE_ACCOUNT_TOKEN_PATH")); path != "" {
		return path
	}
	return defaultServiceAccountTokenPath
}

func serviceAccountBearerToken() (string, bool) {
	tokenBytes, err := os.ReadFile(serviceAccountTokenPath())
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(string(tokenBytes))
	return token, token != ""
}

// setHubDiscoveryAuth prefers the pod service account for hub discovery so callers
// without ManagedCluster list permission still get correct hub-vs-spoke routing.
func setHubDiscoveryAuth(incomingReq *http.Request, outboundReq *http.Request) {
	if token, ok := serviceAccountBearerToken(); ok {
		outboundReq.Header.Set("Authorization", "Bearer "+token)
		return
	}
	if incomingReq != nil {
		util.ForwardAuthHeaders(incomingReq, outboundReq)
	}
}
