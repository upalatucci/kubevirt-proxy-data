package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/handlers"
	"github.com/kubevirt-ui/kubevirt-apiserver-proxy/server"
)

func TestRequestHandler_VMListFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("SERVICE_ACCOUNT_CA_PATH", filepath.Join(t.TempDir(), "missing-ca.crt"))

	const vmListPath = "/apis/kubevirt.io/v1/namespaces/default/virtualmachines"

	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != vmListPath {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"apiVersion":"kubevirt.io/v1",
			"kind":"VirtualMachineList",
			"items":[
				{"metadata":{"name":"healthy-vm"},"status":{"printableStatus":"Running"}},
				{"metadata":{"name":"unhealthy-vm"},"status":{"printableStatus":"CrashLoopBackOff"}}
			]
		}`))
	}))
	defer apiServer.Close()

	originalAPIServerURL := handlers.APIServerURL
	handlers.APIServerURL = strings.TrimPrefix(apiServer.URL, "https://")
	t.Cleanup(func() {
		handlers.APIServerURL = originalAPIServerURL
	})

	router := server.NewRouter()

	runRequest := func(t *testing.T, path string) map[string]interface{} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return payload
	}

	unfiltered := runRequest(t, vmListPath)
	if unfiltered["totalItems"] != float64(2) {
		t.Fatalf("expected totalItems 2, got %v", unfiltered["totalItems"])
	}
	items, ok := unfiltered["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 items without filters, got %v", unfiltered["items"])
	}

	filtered := runRequest(t, vmListPath+"?status.printableStatus=Error")
	if filtered["totalItems"] != float64(2) {
		t.Fatalf("expected totalItems to reflect upstream list size, got %v", filtered["totalItems"])
	}

	filteredItems, ok := filtered["items"].([]interface{})
	if !ok || len(filteredItems) != 1 {
		t.Fatalf("expected 1 filtered item, got %v", filtered["items"])
	}

	first, ok := filteredItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map item, got %T", filteredItems[0])
	}
	metadata, ok := first["metadata"].(map[string]interface{})
	if !ok || metadata["name"] != "unhealthy-vm" {
		t.Fatalf("expected unhealthy-vm in filtered response, got %v", first)
	}
}

func TestRequestHandler_IgnoresClusterQueryParamForListFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("SERVICE_ACCOUNT_CA_PATH", filepath.Join(t.TempDir(), "missing-ca.crt"))

	const vmListPath = "/apis/kubevirt.io/v1/namespaces/default/virtualmachines"

	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != vmListPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"VirtualMachineList","items":[{"metadata":{"name":"vm-a"}}]}`))
	}))
	defer apiServer.Close()

	originalAPIServerURL := handlers.APIServerURL
	handlers.APIServerURL = strings.TrimPrefix(apiServer.URL, "https://")
	t.Cleanup(func() {
		handlers.APIServerURL = originalAPIServerURL
	})

	router := server.NewRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, vmListPath+"?cluster=bm07", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, ok := payload["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 item when cluster query is not a filter key, got %v", payload["items"])
	}
}

func TestRequestHandler_RejectsApisPathWithoutGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "dev")

	router := server.NewRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apis/v1/namespaces/default/persistentvolumeclaims", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestRequestHandler_ProxiesCoreAPIPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("SERVICE_ACCOUNT_CA_PATH", filepath.Join(t.TempDir(), "missing-ca.crt"))

	const pvcListPath = "/api/v1/namespaces/default/persistentvolumeclaims"

	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != pvcListPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"PersistentVolumeClaimList","items":[{"metadata":{"name":"data"}}]}`))
	}))
	defer apiServer.Close()

	originalAPIServerURL := handlers.APIServerURL
	handlers.APIServerURL = strings.TrimPrefix(apiServer.URL, "https://")
	t.Cleanup(func() {
		handlers.APIServerURL = originalAPIServerURL
	})

	router := server.NewRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, pvcListPath, nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}
