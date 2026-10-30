package handlers

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers/multicluster"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/proxy"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/util"
)

var PROTOCOL string = "https"
var ORIGIN = "http://localhost"

func RequestHandler(c *gin.Context) {
	cluster, kubernetesPath, err := NormalizeProxyPath(c.Request.URL.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Request.URL.Path = kubernetesPath

	if c.Request.URL.Scheme == "" {
		c.Request.URL.Scheme = "https"
	}

	multicluster.RewriteProxyRequest(c.Request, cluster)

	tlsConf := &tls.Config{InsecureSkipVerify: true}

	proxy := &proxy.Proxy{
		Config: &proxy.Config{
			TLSClientConfig: tlsConf,
			Endpoint:        c.Request.URL,
			Origin:          ORIGIN,
		},
	}

	c.Request.Header.Set("Origin", ORIGIN)
	c.Request.Header.Set("Accept-Encoding", "*")
	defer c.Request.Body.Close()

	if c.IsWebsocket() {

		proxy.ServeHTTP(c.Writer, c.Request)

	} else {

		httpClient := K8sHTTPClientForCluster(cluster, c.Request)

		c.Request.RequestURI = ""
		resp, err := httpClient.Do(c.Request)
		if err != nil {
			log.Println("Failed to initiate call to kube api server: ", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to Kubernetes API server"})
			return
		}

		bodyBytes, err := io.ReadAll(resp.Body)

		if err != nil {
			log.Println("Failed to read response body: ", err.Error())
		}

		defer resp.Body.Close()
		filterQuery := c.Request.URL.Query()
		filterQuery.Del(multicluster.ClusterQueryParam)
		filteredJson := util.FilterResponseQuery(bodyBytes, filterQuery)

		if err != nil {
			log.Println("Unable to transform response body to json) ", err.Error())
		}

		c.JSON(resp.StatusCode, filteredJson)
	}
}
