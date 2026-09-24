package allowednamespaces

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	maxConcurrencyQueryParam = "maxConcurrency"
	maxConcurrencyEnvVar       = "ALLOWED_NS_MAX_CONCURRENCY"
	maxConcurrencyCapEnvVar    = "ALLOWED_NS_MAX_CONCURRENCY_CAP"

	fallbackMaxConcurrentSSARChecks = 20
	fallbackMaxConcurrentSSARCap    = 100
)

var (
	defaultMaxConcurrentSSARChecks = envInt(maxConcurrencyEnvVar, fallbackMaxConcurrentSSARChecks)
	maxConcurrentSSARChecksCap       = envInt(maxConcurrencyCapEnvVar, fallbackMaxConcurrentSSARCap)
)

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}

	return parsed
}

func resolveMaxConcurrency(c *gin.Context) (int, error) {
	queryValue := c.Query(maxConcurrencyQueryParam)
	if queryValue == "" {
		return defaultMaxConcurrentSSARChecks, nil
	}

	maxConcurrency, err := strconv.Atoi(queryValue)
	if err != nil || maxConcurrency < 1 {
		return 0, fmt.Errorf("maxConcurrency must be a positive integer")
	}

	if maxConcurrency > maxConcurrentSSARChecksCap {
		maxConcurrency = maxConcurrentSSARChecksCap
	}

	return maxConcurrency, nil
}
