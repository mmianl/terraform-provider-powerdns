package powerdns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newStalledServer accepts connections and never answers, which is what a hung
// or firewalled PowerDNS looks like from the client side.
func newStalledServer(t *testing.T) net.Listener {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open without replying.
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()

	return ln
}

// A stalled server must not hold an apply open forever.
func TestRequestTimeoutBoundsAStalledServer(t *testing.T) {
	ln := newStalledServer(t)

	client, err := NewPowerDNSClient(context.Background(), "http://"+ln.Addr().String(), "localhost", "key", nil, false, "0", 0, 1)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, time.Second, client.HTTP.Timeout)

	done := make(chan error, 1)
	go func() {
		_, err := client.ListZones(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		assert.Error(t, err, "a stalled server should surface as an error, not a success")
	case <-time.After(15 * time.Second):
		t.Fatal("request did not time out")
	}
}

// Zero keeps the previous unbounded behaviour, for anyone who relied on it.
func TestRequestTimeoutZeroLeavesClientUnbounded(t *testing.T) {
	client, err := NewPowerDNSClient(context.Background(), "http://127.0.0.1:1", "localhost", "key", nil, false, "0", 0, 0)
	if !assert.NoError(t, err) {
		return
	}
	assert.Zero(t, client.HTTP.Timeout)
}

func TestRecursorRequestTimeoutIsApplied(t *testing.T) {
	client, err := NewRecursorClient(context.Background(), "http://127.0.0.1:1", "key", nil, 5)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, 5*time.Second, client.HTTP.Timeout)
}
