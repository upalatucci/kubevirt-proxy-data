package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/config"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers/allowednamespaces"
)

const (
	healthCacheTime = 30 * time.Second
	apiCacheTime    = 15 * time.Second
)

func main() {
	flag.Parse()

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	router := gin.Default()

	memoryStore := persist.NewMemoryStore(1 * time.Minute)

	router.Use(gzip.Gzip(gzip.DefaultCompression))

	router.GET("/health", cache.CacheByRequestURI(memoryStore, healthCacheTime), handlers.HealthHandler)
	router.POST("/apis/allowednamespaces", allowednamespaces.Handler)
	router.GET("/apis/*path", cache.CacheByRequestURI(memoryStore, apiCacheTime), handlers.RequestHandler)

	server := &http.Server{
		Addr:      ":8080",
		Handler:   router,
		TLSConfig: &tls.Config{},
	}

	minTLSVer := cfg.GetMinTLSVersion()
	if minTLSVer != 0 {
		server.TLSConfig.MinVersion = minTLSVer
	}

	if minTLSVer < tls.VersionTLS13 {
		if ciphers := cfg.GetTLSCipherSuites(); len(ciphers) > 0 {
			server.TLSConfig.CipherSuites = ciphers
		}
	}

	server.TLSConfig.CurvePreferences = cfg.GetTLSCurveIDs()

	server.TLSConfig.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
		return server.TLSConfig.Clone(), nil
	}

	log.Printf("listening for server 8080 - v0.0.10 - API cache time: %v", apiCacheTime)

	if os.Getenv("APP_ENV") == "dev" {
		err = server.ListenAndServe()
	} else {
		err = server.ListenAndServeTLS("./cert/tls.crt", "./cert/tls.key")
	}

	if err != nil {
		log.Println("Failed to start server: ", err.Error())
	}
}
