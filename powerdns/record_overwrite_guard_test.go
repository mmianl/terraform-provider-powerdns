package powerdns

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const guardZoneWithRecord = `{"name":"example.com.","kind":"Native","rrsets":[{"name":"www.example.com.","type":"A","ttl":300,"records":[{"content":"192.0.2.1","disabled":false}]}]}`

// Creating over a record set that is already on the server would destroy it,
// so the guard has to refuse and point at import instead.
func TestGuardRecordOverwriteBlocksExisting(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, guardZoneWithRecord), nil
	})

	err := guardRecordOverwrite(context.Background(), client, "powerdns_record", "example.com.", "www.example.com.", "A")
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "already exists")
	// The message has to carry a command the user can actually run.
	assert.Contains(t, err.Error(), "terraform import powerdns_record")
	assert.Contains(t, err.Error(), `"id": "www.example.com.:::A"`)
}

func TestGuardRecordOverwriteAllowsNewName(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, guardZoneWithRecord), nil
	})

	err := guardRecordOverwrite(context.Background(), client, "powerdns_record", "example.com.", "fresh.example.com.", "A")
	assert.NoError(t, err)
}

// A different type at the same name is a separate record set in PowerDNS.
func TestGuardRecordOverwriteAllowsDifferentType(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, guardZoneWithRecord), nil
	})

	err := guardRecordOverwrite(context.Background(), client, "powerdns_record", "example.com.", "www.example.com.", "TXT")
	assert.NoError(t, err)
}

// A missing zone is reported by the zone resource, so the guard must not turn
// it into a second confusing error on every record in that zone.
func TestGuardRecordOverwriteIgnoresMissingZone(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	err := guardRecordOverwrite(context.Background(), client, "powerdns_record", "missing.example.com.", "www.missing.example.com.", "A")
	assert.NoError(t, err)
}

// Any other failure has to surface rather than being read as "does not exist",
// which would let the apply through and overwrite the record after all.
func TestGuardRecordOverwriteSurfacesOtherErrors(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`), nil
	})

	err := guardRecordOverwrite(context.Background(), client, "powerdns_record", "example.com.", "www.example.com.", "A")
	if assert.Error(t, err) {
		assert.True(t, strings.Contains(err.Error(), "couldn't check"), "got: %v", err)
	}
}

// Record names are case-insensitive in DNS, so a differently-cased spelling of
// an existing record must still be caught.
func TestGuardRecordOverwriteIsCaseInsensitive(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, guardZoneWithRecord), nil
	})

	err := guardRecordOverwrite(context.Background(), client, "powerdns_record", "example.com.", "WWW.Example.COM.", "a")
	assert.Error(t, err)
}

// lockRecordCreate has to actually exclude a second holder of the same key,
// or two resources racing to create the same record set could both pass the
// guard's existence check before either one writes.
func TestLockRecordCreateExcludesSameKey(t *testing.T) {
	var active int32
	var sawOverlap bool
	var mu sync.Mutex

	hold := func() {
		unlock := lockRecordCreate("example.com.", "www.example.com.", "A")
		defer unlock()

		mu.Lock()
		active++
		if active > 1 {
			sawOverlap = true
		}
		mu.Unlock()

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hold()
		}()
	}
	wg.Wait()

	assert.False(t, sawOverlap, "lockRecordCreate let two holders of the same key run concurrently")
}

// A different key must not be blocked by an unrelated key's lock.
func TestLockRecordCreateAllowsDifferentKeys(t *testing.T) {
	done := make(chan struct{})

	unlockA := lockRecordCreate("a.example.com.", "www.a.example.com.", "A")
	go func() {
		unlockB := lockRecordCreate("b.example.com.", "www.b.example.com.", "A")
		unlockB()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lockRecordCreate blocked an unrelated key")
	}
	unlockA()
}
