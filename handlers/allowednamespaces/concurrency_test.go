package allowednamespaces

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveMaxConcurrencyUsesDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/apis/allowednamespaces", nil)

	maxConcurrency, err := resolveMaxConcurrency(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxConcurrency != defaultMaxConcurrentSSARChecks {
		t.Fatalf("expected default %d, got %d", defaultMaxConcurrentSSARChecks, maxConcurrency)
	}
}

func TestResolveMaxConcurrencyUsesQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/apis/allowednamespaces?maxConcurrency=5", nil)

	maxConcurrency, err := resolveMaxConcurrency(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxConcurrency != 5 {
		t.Fatalf("expected 5, got %d", maxConcurrency)
	}
}

func TestResolveMaxConcurrencyClampsToCap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/apis/allowednamespaces?maxConcurrency=9999", nil)

	maxConcurrency, err := resolveMaxConcurrency(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxConcurrency != maxConcurrentSSARChecksCap {
		t.Fatalf("expected cap %d, got %d", maxConcurrentSSARChecksCap, maxConcurrency)
	}
}

func TestResolveMaxConcurrencyRejectsInvalidQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/apis/allowednamespaces?maxConcurrency=abc", nil)

	_, err := resolveMaxConcurrency(c)
	if err == nil {
		t.Fatal("expected error for invalid maxConcurrency")
	}
}

func TestHandlerRejectsInvalidMaxConcurrencyQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/apis/allowednamespaces", Handler)

	req := httptest.NewRequest(
		http.MethodPost,
		"/apis/allowednamespaces?maxConcurrency=0",
		strings.NewReader(`{"resourceAttributes":{"verb":"get","resource":"pods"}}`),
	)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
