package powerdns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Terraform sends requests in parallel, and the first of them detects the API
// version. Run with -race to catch unsynchronized access.
func TestNewRequestDetectsAPIVersionOnce(t *testing.T) {
	var detections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		detections.Add(1)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := NewBaseClient(server.URL, "key", nil, false, "", 0, 0)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := client.newRequest(context.Background(), http.MethodGet, "/servers/localhost", nil)
			assert.NoError(t, err)
			assert.Contains(t, req.URL.Path, "/api/v1/servers/localhost")
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), detections.Load(), "the version must be detected once")
}
