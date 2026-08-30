package powerdns

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A zone that was deleted out of band has to surface as ErrNotFound so the
// resources can drop themselves from state rather than failing every plan.
func TestGetZoneNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	_, err := client.GetZone(context.Background(), "missing.example.com.")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetZoneWithRRsetsNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	_, err := client.GetZoneWithRRsets(context.Background(), "missing.example.com.")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetRecordSetByIDZoneNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	_, err := client.GetRecordSetByID(context.Background(), "missing.example.com.", "www.missing.example.com.:::A")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListZoneMetadataNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	_, err := client.ListZoneMetadata(context.Background(), "missing.example.com.")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetZoneMetadataNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	_, err := client.GetZoneMetadata(context.Background(), "missing.example.com.", "ALLOW-AXFR-FROM")
	assert.ErrorIs(t, err, ErrNotFound)
}

// A non-404 failure must stay a real error rather than being mistaken for a
// deleted zone, which would silently drop the resource from state.
func TestGetZoneServerErrorIsNotNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `{"error":"Internal Server Error"}`), nil
	})

	_, err := client.GetZone(context.Background(), "broken.example.com.")
	if assert.Error(t, err) {
		assert.NotErrorIs(t, err, ErrNotFound)
	}
}
