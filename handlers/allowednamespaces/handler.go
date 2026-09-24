package allowednamespaces

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers"
)

// Handler returns the namespaces where the caller is allowed to perform
// the action described in resourceAttributes.
func Handler(c *gin.Context) {
	defer c.Request.Body.Close()

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var request AllowedNamespacesRequest
	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body must contain resourceAttributes"})
		return
	}

	if request.ResourceAttributes.Verb == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resourceAttributes.verb is required"})
		return
	}

	if !handlers.HasAuthHeader(c.Request) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header with a bearer token is required"})
		return
	}

	maxConcurrency, err := resolveMaxConcurrency(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client := handlers.NewK8sHTTPClient()
	namespaces, err := listAccessibleNamespaces(client, c.Request)
	if err != nil {
		log.Println("Failed to list namespaces: ", err.Error())

		if apiErr, ok := err.(*handlers.APIError); ok {
			c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.Message})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list namespaces from Kubernetes API server"})
		return
	}

	allowedNamespaces, failures := filterAllowedNamespaces(client, c.Request, request.ResourceAttributes, namespaces, maxConcurrency)

	c.JSON(http.StatusOK, AllowedNamespacesResponse{
		AllowedNamespaces: allowedNamespaces,
		Failures:          failures,
	})
}
