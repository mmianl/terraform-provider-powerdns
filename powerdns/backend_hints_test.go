package powerdns

import (
	"net/http"
	"strings"
	"testing"
)

func TestViewsHint(t *testing.T) {
	backendFailures := []string{
		"Failed to add example.com. to view internal",
		"Failed to remove example.com. from view internal",
		"Failed to setup view internal for network 192.0.2.0/24",
	}
	for _, message := range backendFailures {
		if hint := viewsHint(http.StatusUnprocessableEntity, message); !strings.Contains(hint, "LMDB") {
			t.Errorf("a backend failure should mention the LMDB requirement, got: %q", hint)
		}
	}

	for _, code := range []int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized} {
		if hint := viewsHint(code, backendFailures[0]); hint != "" {
			t.Errorf("status %d should not be annotated, got: %q", code, hint)
		}
	}

	if hint := viewsHint(http.StatusUnprocessableEntity, "Empty view names are not allowed"); hint != "" {
		t.Errorf("a semantic 422 should not be annotated, got: %q", hint)
	}
}

func TestRecursorHint(t *testing.T) {
	message := `Config Option "api-config-dir" must be set`
	if hint := recursorHint(message); !strings.Contains(hint, "api_dir") {
		t.Errorf("the api-config-dir message should be explained, got: %q", hint)
	}

	if hint := recursorHint("Could not find domain 'example.com.'"); hint != "" {
		t.Errorf("an unrelated message should not be annotated, got: %q", hint)
	}
}
