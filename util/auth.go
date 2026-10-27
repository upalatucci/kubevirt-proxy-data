package util

import (
	"net/http"
	"strings"
)

// ForwardAuthHeaders copies Authorization and Impersonate-* headers to an outbound request.
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
