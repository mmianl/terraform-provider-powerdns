package powerdns

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestNotifyZone(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com./notify", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"result":"Notification queued"}`), nil
	})

	assert.NoError(t, client.NotifyZone(context.Background(), "example.com."))
}

func TestRectifyZone(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/v1/servers/localhost/zones/example.com./rectify", r.URL.Path)
		return jsonResponse(http.StatusOK, `{"result":"Rectified"}`), nil
	})

	assert.NoError(t, client.RectifyZone(context.Background(), "example.com."))
}

// A missing zone has to be distinguishable so the resource can say so plainly.
func TestZoneActionNotFound(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":"Not Found"}`), nil
	})

	assert.ErrorIs(t, client.NotifyZone(context.Background(), "missing.example.com."), ErrNotFound)
	assert.ErrorIs(t, client.RectifyZone(context.Background(), "missing.example.com."), ErrNotFound)
}

func TestZoneActionSurfacesServerError(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnprocessableEntity, `{"error":"cannot rectify"}`), nil
	})

	err := client.RectifyZone(context.Background(), "example.com.")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "cannot rectify")
	}
}

func TestAccPDNSZoneNotify(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneNotifyConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone_notify.test", "zone", "notify-action.sysa.xyz."),
					resource.TestCheckResourceAttrSet("powerdns_zone_notify.test", "id"),
				),
			},
			{
				// Nothing is tracked on the server, so a second plan has to be
				// empty rather than wanting to run the operation again.
				Config:   testPDNSZoneNotifyConfig,
				PlanOnly: true,
			},
		},
	})
}

func TestAccPDNSZoneRectify(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckPDNSZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: testPDNSZoneRectifyConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("powerdns_zone_rectify.test", "zone", "rectify-action.sysa.xyz."),
				),
			},
			{
				Config:   testPDNSZoneRectifyConfig,
				PlanOnly: true,
			},
		},
	})
}

func TestAccPDNSZoneActionMissingZone(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config:      testPDNSZoneNotifyMissingConfig,
				ExpectError: regexp.MustCompile(`does not exist`),
			},
		},
	})
}

const testPDNSZoneNotifyConfig = `
resource "powerdns_zone" "notify-action" {
	name = "notify-action.sysa.xyz."
	kind = "Master"
}

resource "powerdns_zone_notify" "test" {
	zone = powerdns_zone.notify-action.name
}`

const testPDNSZoneRectifyConfig = `
resource "powerdns_zone" "rectify-action" {
	name = "rectify-action.sysa.xyz."
	kind = "Master"
}

resource "powerdns_zone_rectify" "test" {
	zone = powerdns_zone.rectify-action.name
}`

const testPDNSZoneNotifyMissingConfig = `
resource "powerdns_zone_notify" "missing" {
	zone = "no-such-zone.sysa.xyz."
}`
