package powerdns

import (
	"net/http"
	"strings"
	"testing"
)

func TestViewsHint(t *testing.T) {
	if hint := viewsHint(http.StatusUnprocessableEntity); !strings.Contains(hint, "LMDB") {
		t.Errorf("a 422 should mention the LMDB requirement, got: %q", hint)
	}

	for _, code := range []int{http.StatusOK, http.StatusNotFound, http.StatusUnauthorized} {
		if hint := viewsHint(code); hint != "" {
			t.Errorf("status %d should not be annotated, got: %q", code, hint)
		}
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
