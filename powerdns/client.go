package powerdns

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	freecache "github.com/coocood/freecache"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// DefaultSchema is the value used for the URL in case
// no schema is explicitly defined
var DefaultSchema = "https"

// DefaultCacheSize is client default cache size
var DefaultCacheSize int

// sanitizeURL will output:
// <scheme>://<host>[:port]
// with no trailing /
func sanitizeURL(URL string) (string, error) {
	cleanURL := ""
	host := ""
	schema := ""

	var err error

	if len(URL) == 0 {
		return "", fmt.Errorf("no URL provided")
	}

	parsedURL, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("error while trying to parse URL: %s", err)
	}

	if len(parsedURL.Scheme) == 0 {
		schema = DefaultSchema
	} else {
		if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
			schema = parsedURL.Scheme
		} else {
			schema = DefaultSchema
		}
	}

	if len(parsedURL.Host) == 0 {
		tryout, _ := url.Parse(schema + "://" + URL)

		if len(tryout.Host) == 0 {
			return "", fmt.Errorf("unable to find a hostname in '%s'", URL)
		}

		host = tryout.Host
	} else {
		host = parsedURL.Host
	}

	cleanURL = schema + "://" + host

	return cleanURL, nil
}

// BaseClient contains shared HTTP / auth / cache logic for PowerDNS-style APIs.
type BaseClient struct {
	ServerURL     string // Location of the server to use
	ServerVersion string
	APIKey        string // REST API Static authentication key
	APIVersion    int    // API version to use
	HTTP          *http.Client
	CacheEnable   bool // Enable/Disable cache for REST API requests
	Cache         *freecache.Cache
	CacheTTL      int
}

// NewBaseClient constructs a BaseClient with HTTP, TLS and cache configuration.
func NewBaseClient(ctx context.Context, serverURL string, apiKey string, configTLS *tls.Config, cacheEnable bool, cacheSizeMB string, cacheTTL int) (*BaseClient, error) {
	cleanURL, err := sanitizeURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("error while creating client: %s", err)
	}

	httpClient := cleanhttp.DefaultClient()
	httpClient.Transport.(*http.Transport).TLSClientConfig = configTLS

	if cacheEnable {
		cacheSize, err := strconv.Atoi(cacheSizeMB)
		if err != nil {
			return nil, fmt.Errorf("error while creating client: %s", err)
		}
		DefaultCacheSize = cacheSize * 1024 * 1024
	}

	base := &BaseClient{
		ServerURL:   cleanURL,
		APIKey:      apiKey,
		HTTP:        httpClient,
		APIVersion:  -1,
		CacheEnable: cacheEnable,
		Cache:       freecache.NewCache(DefaultCacheSize),
		CacheTTL:    cacheTTL,
	}

	if err := base.setServerVersion(ctx); err != nil {
		return nil, fmt.Errorf("error while creating client: %s", err)
	}

	return base, nil
}

func (client *BaseClient) setServerVersion(ctx context.Context) error {
	req, err := client.newRequest(ctx, http.MethodGet, "/servers/localhost", nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("invalid response code from server: '%d'. Failed to read response body: %v",
				resp.StatusCode, err)
		}
		return fmt.Errorf("invalid response code from server: '%d'. Response body: %s",
			resp.StatusCode, string(bodyBytes))
	}

	serverInfo := new(serverInfo)
	if err := json.NewDecoder(resp.Body).Decode(serverInfo); err == nil {
		client.ServerVersion = serverInfo.Version
		return nil
	}

	headerServerInfo := strings.SplitN(resp.Header.Get("Server"), "/", 2)
	if len(headerServerInfo) == 2 && strings.EqualFold(headerServerInfo[0], "PowerDNS") {
		client.ServerVersion = headerServerInfo[1]
		return nil
	}

	return fmt.Errorf("unable to get server version")
}

// newRequest creates a new request against the API with necessary headers.
func (client *BaseClient) newRequest(ctx context.Context, method string, endpoint string, body []byte) (*http.Request, error) {
	var err error
	if client.APIVersion < 0 {
		client.APIVersion, err = client.detectAPIVersion(ctx)
	}
	if err != nil {
		return nil, err
	}

	var urlStr string
	if client.APIVersion > 0 {
		urlStr = client.ServerURL + "/api/v" + strconv.Itoa(client.APIVersion) + endpoint
	} else {
		urlStr = client.ServerURL + endpoint
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("error during parsing request URL: %s", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error during creation of request: %s", err)
	}

	req.Header.Add("X-API-Key", client.APIKey)
	req.Header.Add("Accept", "application/json")

	if method != http.MethodGet {
		req.Header.Add("Content-Type", "application/json")
	}

	return req, nil
}

// Detects the API version in use on the server
// Uses int to represent the API version: 0 is the legacy AKA version 3.4 API
// Any other integer correlates with the same API version
func (client *BaseClient) detectAPIVersion(ctx context.Context) (int, error) {
	httpClient := client.HTTP

	u, err := url.Parse(client.ServerURL + "/api/v1/servers")
	if err != nil {
		return -1, fmt.Errorf("error while trying to detect the API version, request URL: %s", err)
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return -1, fmt.Errorf("error during creation of request: %s", err)
	}

	req.Header.Add("X-API-Key", client.APIKey)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return -1, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
			})
		}
	}()

	if resp.StatusCode == http.StatusOK {
		return 1, nil
	}
	return 0, nil
}

// PowerDNSClient is the concrete client used by the provider.
// ZoneInfo represents a PowerDNS zone object
type ZoneInfo struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	URL                string              `json:"url"`
	Kind               string              `json:"kind"`
	DNSSec             bool                `json:"dnsssec"`
	Serial             int64               `json:"serial"`
	Records            []Record            `json:"records,omitempty"`
	ResourceRecordSets []ResourceRecordSet `json:"rrsets,omitempty"`
	Account            string              `json:"account"`
	Nameservers        []string            `json:"nameservers,omitempty"`
	Masters            []string            `json:"masters,omitempty"`
	SoaEditAPI         string              `json:"soa_edit_api"`
}

// ZoneInfoUpd is a limited subset for supported updates
type ZoneInfoUpd struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	SoaEditAPI string `json:"soa_edit_api,omitempty"`
	Account    string `json:"account"`
}

// Record represents a PowerDNS record object
type Record struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"` // For API v0
	Disabled bool   `json:"disabled"`
	SetPtr   bool   `json:"set-ptr"`
}

// ResourceRecordSet represents a PowerDNS RRSet object
type ResourceRecordSet struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	ChangeType string   `json:"changetype"`
	TTL        int      `json:"ttl"` // For API v1
	Records    []Record `json:"records,omitempty"`
}

type zonePatchRequest struct {
	RecordSets []ResourceRecordSet `json:"rrsets"`
}

type errorResponse struct {
	ErrorMsg string `json:"error"`
}

type serverInfo struct {
	ConfigURL  string `json:"config_url"`
	DaemonType string `json:"daemon_type"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	Version    string `json:"version"`
	ZonesURL   string `json:"zones_url"`
}

const idSeparator string = ":::"

// Sentinel error for "not found" scenarios
var (
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = errors.New("not found")
)

// ID returns a record with the ID format
func (record *Record) ID() string {
	return record.Name + idSeparator + record.Type
}

// ID returns a rrSet with the ID format
func (rrSet *ResourceRecordSet) ID() string {
	return rrSet.Name + idSeparator + rrSet.Type
}

// Returns name and type of record or record set based on its ID
func parseID(recID string) (string, string, error) {
	s := strings.Split(recID, idSeparator)
	if len(s) == 2 {
		return s[0], s[1], nil
	}
	return "", "", fmt.Errorf("unknown record ID format")
}

type PowerDNSClient struct {
	*BaseClient
}

// NewPowerDNSClient constructs the derived PowerDNS client used by the provider.
func NewPowerDNSClient(ctx context.Context, serverURL string, apiKey string, configTLS *tls.Config, cacheEnable bool, cacheSizeMB string, cacheTTL int) (*PowerDNSClient, error) {
	base, err := NewBaseClient(ctx, serverURL, apiKey, configTLS, cacheEnable, cacheSizeMB, cacheTTL)
	if err != nil {
		return nil, err
	}
	return &PowerDNSClient{BaseClient: base}, nil
}

// ListZones returns all Zones of server, without records
func (client *PowerDNSClient) ListZones(ctx context.Context) ([]ZoneInfo, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/servers/localhost/zones", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
			})
		}
	}()

	var zoneInfos []ZoneInfo
	if err := json.NewDecoder(resp.Body).Decode(&zoneInfos); err != nil {
		return nil, err
	}

	return zoneInfos, nil
}

// GetZone gets a zone
func (client *PowerDNSClient) GetZone(ctx context.Context, name string) (ZoneInfo, error) {
	req, err := client.newRequest(ctx, http.MethodGet, fmt.Sprintf("/servers/localhost/zones/%s", name), nil)
	if err != nil {
		return ZoneInfo{}, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return ZoneInfo{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   name,
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return ZoneInfo{}, fmt.Errorf("error getting zone: %s", name)
		}
		return ZoneInfo{}, fmt.Errorf("error getting zone: %s, reason: %q", name, errorResp.ErrorMsg)
	}

	var zoneInfo ZoneInfo
	if err := json.NewDecoder(resp.Body).Decode(&zoneInfo); err != nil {
		return ZoneInfo{}, err
	}

	return zoneInfo, nil
}

// ZoneExists checks if requested zone exists
func (client *PowerDNSClient) ZoneExists(ctx context.Context, name string) (bool, error) {
	req, err := client.newRequest(ctx, http.MethodGet, fmt.Sprintf("/servers/localhost/zones/%s", name), nil)
	if err != nil {
		return false, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   name,
			})
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return false, fmt.Errorf("error getting zone: %s", name)
		}
		return false, fmt.Errorf("error getting zone: %s, reason: %q", name, errorResp.ErrorMsg)
	}

	return resp.StatusCode == http.StatusOK, nil
}

// CreateZone creates a zone
func (client *PowerDNSClient) CreateZone(ctx context.Context, zoneInfo ZoneInfo) (ZoneInfo, error) {
	body, err := json.Marshal(zoneInfo)
	if err != nil {
		return ZoneInfo{}, err
	}

	req, err := client.newRequest(ctx, http.MethodPost, "/servers/localhost/zones", body)
	if err != nil {
		return ZoneInfo{}, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return ZoneInfo{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   zoneInfo.Name,
			})
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return ZoneInfo{}, fmt.Errorf("error creating zone: %s", zoneInfo.Name)
		}
		return ZoneInfo{}, fmt.Errorf("error creating zone: %s, reason: %q", zoneInfo.Name, errorResp.ErrorMsg)
	}

	var createdZoneInfo ZoneInfo
	if err := json.NewDecoder(resp.Body).Decode(&createdZoneInfo); err != nil {
		return ZoneInfo{}, err
	}

	return createdZoneInfo, nil
}

// UpdateZone updates a zone
func (client *PowerDNSClient) UpdateZone(ctx context.Context, name string, zoneInfo ZoneInfoUpd) error {
	body, err := json.Marshal(zoneInfo)
	if err != nil {
		return err
	}

	req, err := client.newRequest(ctx, http.MethodPut, fmt.Sprintf("/servers/localhost/zones/%s", name), body)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   name,
			})
		}
	}()

	if resp.StatusCode != http.StatusNoContent {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error updating zone: %s", zoneInfo.Name)
		}
		return fmt.Errorf("error updating zone: %s, reason: %q", zoneInfo.Name, errorResp.ErrorMsg)
	}

	return nil
}

// DeleteZone deletes a zone
func (client *PowerDNSClient) DeleteZone(ctx context.Context, name string) error {
	req, err := client.newRequest(ctx, http.MethodDelete, fmt.Sprintf("/servers/localhost/zones/%s", name), nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   name,
			})
		}
	}()

	if resp.StatusCode != http.StatusNoContent {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error deleting zone: %s", name)
		}
		return fmt.Errorf("error deleting zone: %s, reason: %q", name, errorResp.ErrorMsg)
	}
	return nil
}

// GetZoneInfoFromCache return ZoneInfo struct
func (client *PowerDNSClient) GetZoneInfoFromCache(ctx context.Context, zone string) (*ZoneInfo, error) {
	if client.CacheEnable {
		cacheZoneInfo, err := client.Cache.Get([]byte(zone))
		if err != nil {
			return nil, err
		}

		zoneInfo := new(ZoneInfo)
		if err := json.Unmarshal(cacheZoneInfo, &zoneInfo); err != nil {
			return nil, err
		}

		return zoneInfo, nil
	}

	return nil, nil
}

// ListRecords returns all records in Zone
func (client *PowerDNSClient) ListRecords(ctx context.Context, zone string) ([]Record, error) {
	zoneInfo, err := client.GetZoneInfoFromCache(ctx, zone)
	if err != nil {
		tflog.Warn(ctx, "Cache get failed", map[string]interface{}{
			"zone":  zone,
			"error": err.Error(),
		})
		return nil, err
	}

	if zoneInfo == nil {
		req, err := client.newRequest(ctx, http.MethodGet, fmt.Sprintf("/servers/localhost/zones/%s", zone), nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
					"error":  err.Error(),
					"method": req.Method,
					"url":    req.URL.String(),
					"zone":   zone,
				})
			}
		}()

		zoneInfo = new(ZoneInfo)
		if err := json.NewDecoder(resp.Body).Decode(zoneInfo); err != nil {
			return nil, err
		}

		if client.CacheEnable {
			cacheValue, err := json.Marshal(zoneInfo)
			if err != nil {
				return nil, err
			}

			if err := client.Cache.Set([]byte(zone), cacheValue, client.CacheTTL); err != nil {
				return nil, fmt.Errorf("the cache for REST API requests is enabled but the size isn't enough: cacheSize: %db \n %s",
					DefaultCacheSize, err)
			}
		}
	}

	records := zoneInfo.Records
	// Convert the API v1 response to v0 record structure
	for _, rrs := range zoneInfo.ResourceRecordSets {
		for _, record := range rrs.Records {
			records = append(records, Record{
				Name:    rrs.Name,
				Type:    rrs.Type,
				Content: record.Content,
				TTL:     rrs.TTL,
			})
		}
	}

	return records, nil
}

// ListRecordsInRRSet returns only records of specified name and type
func (client *PowerDNSClient) ListRecordsInRRSet(ctx context.Context, zone string, name string, tpe string) ([]Record, error) {
	allRecords, err := client.ListRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	records := make([]Record, 0, 10)
	for _, r := range allRecords {
		if strings.EqualFold(r.Name, name) && strings.EqualFold(r.Type, tpe) {
			records = append(records, r)
		}
	}

	return records, nil
}

// ListRecordsByID returns all records by IDs
func (client *PowerDNSClient) ListRecordsByID(ctx context.Context, zone string, recID string) ([]Record, error) {
	name, tpe, err := parseID(recID)
	if err != nil {
		return nil, err
	}
	return client.ListRecordsInRRSet(ctx, zone, name, tpe)
}

// RecordExists checks if requested record exists in Zone
func (client *PowerDNSClient) RecordExists(ctx context.Context, zone string, name string, tpe string) (bool, error) {
	allRecords, err := client.ListRecords(ctx, zone)
	if err != nil {
		return false, err
	}

	for _, record := range allRecords {
		if strings.EqualFold(record.Name, name) && strings.EqualFold(record.Type, tpe) {
			return true, nil
		}
	}
	return false, nil
}

// RecordExistsByID checks if requested record exists in Zone by its ID
func (client *PowerDNSClient) RecordExistsByID(ctx context.Context, zone string, recID string) (bool, error) {
	name, tpe, err := parseID(recID)
	if err != nil {
		return false, err
	}
	return client.RecordExists(ctx, zone, name, tpe)
}

// ReplaceRecordSet creates new record set in Zone
func (client *PowerDNSClient) ReplaceRecordSet(ctx context.Context, zone string, rrSet ResourceRecordSet) (string, error) {
	rrSet.ChangeType = "REPLACE"

	reqBody, _ := json.Marshal(zonePatchRequest{
		RecordSets: []ResourceRecordSet{rrSet},
	})

	req, err := client.newRequest(ctx, http.MethodPatch, fmt.Sprintf("/servers/localhost/zones/%s", zone), reqBody)
	if err != nil {
		return "", err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":   err.Error(),
				"method":  req.Method,
				"url":     req.URL.String(),
				"zone":    zone,
				"rrsetId": rrSet.ID(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return "", fmt.Errorf("error creating record set: %s", rrSet.ID())
		}
		return "", fmt.Errorf("error creating record set: %s, reason: %q", rrSet.ID(), errorResp.ErrorMsg)
	}
	return rrSet.ID(), nil
}

// DeleteRecordSet deletes record set from Zone
func (client *PowerDNSClient) DeleteRecordSet(ctx context.Context, zone string, name string, tpe string) error {
	reqBody, _ := json.Marshal(zonePatchRequest{
		RecordSets: []ResourceRecordSet{
			{
				Name:       name,
				Type:       tpe,
				ChangeType: "DELETE",
			},
		},
	})

	req, err := client.newRequest(ctx, http.MethodPatch, fmt.Sprintf("/servers/localhost/zones/%s", zone), reqBody)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   zone,
				"name":   name,
				"type":   tpe,
			})
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error deleting record: %s %s", name, tpe)
		}
		return fmt.Errorf("error deleting record: %s %s, reason: %q", name, tpe, errorResp.ErrorMsg)
	}
	return nil
}

// DeleteRecordSetByID deletes record from Zone by its ID
func (client *PowerDNSClient) DeleteRecordSetByID(ctx context.Context, zone string, recID string) error {
	name, tpe, err := parseID(recID)
	if err != nil {
		return err
	}
	return client.DeleteRecordSet(ctx, zone, name, tpe)
}

// RecursorClient talks to the PowerDNS Recursor API.
type RecursorClient struct {
	*BaseClient
}

// RecursorForwardZone represents a PowerDNS Recursor forward zone.
type RecursorForwardZone struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Kind             string   `json:"kind"`
	Servers          []string `json:"servers"`
	RecursionDesired bool     `json:"recursion_desired"`
}

// RecursorConfigSetting represents a single recursor config entry like:
//
//	{ "name": "allow-from", "value": ["127.0.0.0/8"] }
//
// Only incoming.allow_from and incoming.allow_notify_from can be changed via the API
// as per https://doc.powerdns.com/recursor/http-api/endpoint-servers-config.html
type RecursorConfigSetting struct {
	Name  string   `json:"name"`
	Value []string `json:"value"`
}

// NewRecursorClient builds a client for the recursor server.
func NewRecursorClient(
	ctx context.Context,
	recursorURL string,
	apiKey string,
	configTLS *tls.Config,
) (*RecursorClient, error) {
	base, err := NewBaseClient(ctx, recursorURL, apiKey, configTLS, false, "0", 0)
	if err != nil {
		return nil, err
	}
	return &RecursorClient{BaseClient: base}, nil
}

// GetForwardZone retrieves a specific recursor forward zone definition.
func (client *RecursorClient) GetForwardZone(ctx context.Context, name string) (*RecursorForwardZone, error) {
	endpoint := fmt.Sprintf("/servers/localhost/zones/%s", name)

	req, err := client.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   name,
			})
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var zone RecursorForwardZone
		if err := json.NewDecoder(resp.Body).Decode(&zone); err != nil {
			return nil, err
		}
		return &zone, nil

	case http.StatusNotFound:
		return nil, ErrNotFound

	default:
		var errorResp errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("error getting forward zone %s", name)
		}
		return nil, fmt.Errorf("error getting forward zone %s: %q", name, errorResp.ErrorMsg)
	}
}

// CreateForwardZone creates a recursor forward zone.
func (client *RecursorClient) CreateForwardZone(ctx context.Context, zone *RecursorForwardZone) error {
	body, err := json.Marshal(zone)
	if err != nil {
		return err
	}

	req, err := client.newRequest(ctx, http.MethodPost, "/servers/localhost/zones", body)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   zone.Name,
			})
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		var errorResp errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return fmt.Errorf("error creating forward zone %s", zone.Name)
		}
		return fmt.Errorf("error creating forward zone %s: %q", zone.Name, errorResp.ErrorMsg)
	}

	return nil
}

// DeleteForwardZone deletes a recursor forward zone.
func (client *RecursorClient) DeleteForwardZone(ctx context.Context, name string) error {
	endpoint := fmt.Sprintf("/servers/localhost/zones/%s", name)

	req, err := client.newRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]interface{}{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"zone":   name,
			})
		}
	}()

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil

	case http.StatusNotFound:
		return ErrNotFound

	default:
		var errorResp errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return fmt.Errorf("error deleting forward zone %s", name)
		}
		return fmt.Errorf("error deleting forward zone %s: %q", name, errorResp.ErrorMsg)
	}
}

// GetConfig retrieves a single recursor config setting using
func (client *RecursorClient) GetConfig(ctx context.Context, name string) (*RecursorConfigSetting, error) {
	endpoint := fmt.Sprintf("/servers/localhost/config/%s", name)

	req, err := client.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]any{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"name":   name,
			})
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var setting RecursorConfigSetting
		if err := json.NewDecoder(resp.Body).Decode(&setting); err != nil {
			return nil, err
		}
		return &setting, nil

	case http.StatusNotFound:
		return nil, ErrNotFound

	default:
		var errorResp errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("error getting recursor config %s", name)
		}
		return nil, fmt.Errorf("error getting recursor config %s: %q", name, errorResp.ErrorMsg)
	}
}

// SetConfig changes a single recursor config setting using
func (client *RecursorClient) SetConfig(ctx context.Context, name string, values []string) error {
	setting := RecursorConfigSetting{
		Name:  name,
		Value: values,
	}

	body, err := json.Marshal(&setting)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("/servers/localhost/config/%s", name)

	req, err := client.newRequest(ctx, http.MethodPut, endpoint, body)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			tflog.Warn(ctx, "Error closing response body", map[string]any{
				"error":  err.Error(),
				"method": req.Method,
				"url":    req.URL.String(),
				"name":   name,
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		var errorResp errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return fmt.Errorf("error setting recursor config %s", name)
		}
		return fmt.Errorf("error setting recursor config %s: %q", name, errorResp.ErrorMsg)
	}

	return nil
}
