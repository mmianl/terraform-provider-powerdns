package powerdns

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	freecache "github.com/coocood/freecache"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
)

// DefaultSchema is the value used for the URL in case
// no schema is explicitly defined
var DefaultSchema = "https"

// DefaultCacheSize is client default cache size
var DefaultCacheSize int

// Client is a PowerDNS client representation
type Client struct {
	ServerURL         string // Location of PowerDNS authoritative server to use
	RecursorServerURL string // Location of PowerDNS recursor server to use
	ServerVersion     string
	APIKey            string // REST API Static authentication key
	APIVersion        int    // API version to use
	HTTP              *http.Client
	CacheEnable       bool // Enable/Disable chache for REST API requests
	Cache             *freecache.Cache
	CacheTTL          int
}

// NewClient returns a new PowerDNS client
func NewClient(serverURL string, recursorServerURL string, apiKey string, configTLS *tls.Config, cacheEnable bool, cacheSizeMB string, cacheTTL int) (*Client, error) {

	cleanURL, err := sanitizeURL(serverURL)

	httpClient := cleanhttp.DefaultClient()
	httpClient.Transport.(*http.Transport).TLSClientConfig = configTLS

	if err != nil {
		return nil, fmt.Errorf("error while creating client: %s", err)
	}

	if cacheEnable {
		cacheSize, err := strconv.Atoi(cacheSizeMB)
		if err != nil {
			return nil, fmt.Errorf("error while creating client: %s", err)
		}
		DefaultCacheSize = cacheSize * 1024 * 1024
	}

	client := Client{
		ServerURL:         cleanURL,
		RecursorServerURL: recursorServerURL,
		APIKey:            apiKey,
		HTTP:              httpClient,
		APIVersion:        -1,
		CacheEnable:       cacheEnable,
		Cache:             freecache.NewCache(DefaultCacheSize),
		CacheTTL:          cacheTTL,
	}

	if err := client.setServerVersion(); err != nil {
		return nil, fmt.Errorf("error while creating client: %s", err)
	}

	return &client, nil
}

// sanitizeURL will output:
// <scheme>://<host>[:port]
// with no trailing /
// For details on the implementation be familiar with the behavior or url.Parse
// specifically: https://go-review.googlesource.com/c/go/+/81436/
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
		// this is necessary because when using `<host>:<port>` (without schema)
		// url.Parse will contain Scheme = host.
		if (parsedURL.Scheme == "http") || (parsedURL.Scheme == "https") {
			schema = parsedURL.Scheme
		} else {
			schema = DefaultSchema
		}
	}

	if len(parsedURL.Host) == 0 {
		// url.Parse will return an empty host when the value passed
		// contains no schema, so we add a default schema and force parsing
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

// Creates a new request with necessary headers
func (client *Client) newRequest(method string, endpoint string, body []byte) (*http.Request, error) {

	var err error
	if client.APIVersion < 0 {
		client.APIVersion, err = client.detectAPIVersion()
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
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("error during parsing request URL: %s", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error during creation of request: %s", err)
	}

	req.Header.Add("X-API-Key", client.APIKey)
	req.Header.Add("Accept", "application/json")

	if method != "GET" {
		req.Header.Add("Content-Type", "application/json")
	}

	return req, nil
}

// Creates a new request for recursor API
func (client *Client) newRequestRecursor(method string, endpoint string, body []byte) (*http.Request, error) {
	var err error
	if client.APIVersion < 0 {
		client.APIVersion, err = client.detectAPIVersion()
	}

	if err != nil {
		return nil, err
	}

	var urlStr string
	if client.APIVersion > 0 {
		urlStr = client.RecursorServerURL + "/api/v" + strconv.Itoa(client.APIVersion) + endpoint
	} else {
		urlStr = client.RecursorServerURL + endpoint
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

	if method != "GET" {
		req.Header.Add("Content-Type", "application/json")
	}

	return req, nil
}

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

// Detects the API version in use on the server
// Uses int to represent the API version: 0 is the legacy AKA version 3.4 API
// Any other integer correlates with the same API version
func (client *Client) detectAPIVersion() (int, error) {

	httpClient := client.HTTP

	url, err := url.Parse(client.ServerURL + "/api/v1/servers")
	if err != nil {
		return -1, fmt.Errorf("error while trying to detect the API version, request URL: %s", err)
	}

	req, err := http.NewRequest("GET", url.String(), nil)
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
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()
	if resp.StatusCode == 200 {
		return 1, nil
	}
	return 0, nil
}

// ListZones returns all Zones of server, without records
func (client *Client) ListZones() ([]ZoneInfo, error) {
	req, err := client.newRequest("GET", "/servers/localhost/zones", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	var zoneInfos []ZoneInfo

	err = json.NewDecoder(resp.Body).Decode(&zoneInfos)
	if err != nil {
		return nil, err
	}

	return zoneInfos, nil
}

// GetZone gets a zone
func (client *Client) GetZone(name string) (ZoneInfo, error) {
	req, err := client.newRequest("GET", fmt.Sprintf("/servers/localhost/zones/%s", name), nil)
	if err != nil {
		return ZoneInfo{}, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return ZoneInfo{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
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
	err = json.NewDecoder(resp.Body).Decode(&zoneInfo)
	if err != nil {
		return ZoneInfo{}, err
	}

	return zoneInfo, nil
}

// ZoneExists checks if requested zone exists
func (client *Client) ZoneExists(name string) (bool, error) {
	req, err := client.newRequest("GET", fmt.Sprintf("/servers/localhost/zones/%s", name), nil)
	if err != nil {
		return false, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
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
func (client *Client) CreateZone(zoneInfo ZoneInfo) (ZoneInfo, error) {
	body, err := json.Marshal(zoneInfo)
	if err != nil {
		return ZoneInfo{}, err
	}

	req, err := client.newRequest("POST", "/servers/localhost/zones", body)
	if err != nil {
		return ZoneInfo{}, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return ZoneInfo{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
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
	err = json.NewDecoder(resp.Body).Decode(&createdZoneInfo)
	if err != nil {
		return ZoneInfo{}, err
	}

	return createdZoneInfo, nil
}

// UpdateZone updates a zone
func (client *Client) UpdateZone(name string, zoneInfo ZoneInfoUpd) error {
	body, err := json.Marshal(zoneInfo)
	if err != nil {
		return err
	}

	req, err := client.newRequest("PUT", fmt.Sprintf("/servers/localhost/zones/%s", name), body)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
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
func (client *Client) DeleteZone(name string) error {
	req, err := client.newRequest("DELETE", fmt.Sprintf("/servers/localhost/zones/%s", name), nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	if resp.StatusCode != 204 {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error deleting zone: %s", name)
		}
		return fmt.Errorf("error deleting zone: %s, reason: %q", name, errorResp.ErrorMsg)
	}
	return nil
}

// GetZoneInfoFromCache return ZoneInfo struct
func (client *Client) GetZoneInfoFromCache(zone string) (*ZoneInfo, error) {
	if client.CacheEnable {
		cacheZoneInfo, err := client.Cache.Get([]byte(zone))
		if err != nil {
			return nil, err
		}

		zoneInfo := new(ZoneInfo)
		err = json.Unmarshal(cacheZoneInfo, &zoneInfo)
		if err != nil {
			return nil, err
		}

		return zoneInfo, err
	}
	return nil, nil
}

// ListRecords returns all records in Zone
func (client *Client) ListRecords(zone string) ([]Record, error) {
	zoneInfo, err := client.GetZoneInfoFromCache(zone)
	if err != nil {
		log.Printf("[WARN] module.freecache: %s: %s", zone, err)
	}

	if zoneInfo == nil {
		req, err := client.newRequest("GET", fmt.Sprintf("/servers/localhost/zones/%s", zone), nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("[WARN] Error closing response body: %s", err)
			}
		}()

		zoneInfo = new(ZoneInfo)
		err = json.NewDecoder(resp.Body).Decode(zoneInfo)
		if err != nil {
			return nil, err
		}

		if client.CacheEnable {
			cacheValue, err := json.Marshal(zoneInfo)
			if err != nil {
				return nil, err
			}

			err = client.Cache.Set([]byte(zone), cacheValue, client.CacheTTL)
			if err != nil {
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
func (client *Client) ListRecordsInRRSet(zone string, name string, tpe string) ([]Record, error) {
	allRecords, err := client.ListRecords(zone)
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
func (client *Client) ListRecordsByID(zone string, recID string) ([]Record, error) {
	name, tpe, err := parseID(recID)
	if err != nil {
		return nil, err
	}
	return client.ListRecordsInRRSet(zone, name, tpe)
}

// RecordExists checks if requested record exists in Zone
func (client *Client) RecordExists(zone string, name string, tpe string) (bool, error) {
	allRecords, err := client.ListRecords(zone)
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

// RecordExistsByID checks if requested record exists in Zone by it's ID
func (client *Client) RecordExistsByID(zone string, recID string) (bool, error) {
	name, tpe, err := parseID(recID)
	if err != nil {
		return false, err
	}
	return client.RecordExists(zone, name, tpe)
}

// ReplaceRecordSet creates new record set in Zone
func (client *Client) ReplaceRecordSet(zone string, rrSet ResourceRecordSet) (string, error) {
	rrSet.ChangeType = "REPLACE"

	reqBody, _ := json.Marshal(zonePatchRequest{
		RecordSets: []ResourceRecordSet{rrSet},
	})

	req, err := client.newRequest("PATCH", fmt.Sprintf("/servers/localhost/zones/%s", zone), reqBody)
	if err != nil {
		return "", err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return "", fmt.Errorf("error creating record set: %s", rrSet.ID())
		}
		return "", fmt.Errorf("error creating record set: %s, reason: %q", rrSet.ID(), errorResp.ErrorMsg)
	}
	return rrSet.ID(), nil
}

// DeleteRecordSet deletes record set from Zone
func (client *Client) DeleteRecordSet(zone string, name string, tpe string) error {
	reqBody, _ := json.Marshal(zonePatchRequest{
		RecordSets: []ResourceRecordSet{
			{
				Name:       name,
				Type:       tpe,
				ChangeType: "DELETE",
			},
		},
	})

	req, err := client.newRequest("PATCH", fmt.Sprintf("/servers/localhost/zones/%s", zone), reqBody)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error deleting record: %s %s", name, tpe)
		}
		return fmt.Errorf("error deleting record: %s %s, reason: %q", name, tpe, errorResp.ErrorMsg)
	}
	return nil
}

// DeleteRecordSetByID deletes record from Zone by its ID
func (client *Client) DeleteRecordSetByID(zone string, recID string) error {
	name, tpe, err := parseID(recID)
	if err != nil {
		return err
	}
	return client.DeleteRecordSet(zone, name, tpe)
}

func (client *Client) setServerVersion() error {
	req, err := client.newRequest("GET", "/servers/localhost", nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	if resp.StatusCode != 200 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("invalid response code from server: '%d'. Failed to read response body: %v",
				resp.StatusCode, err)
		}
		return fmt.Errorf("invalid response code from server: '%d'. Response body: %s",
			resp.StatusCode, string(bodyBytes))
	}

	serverInfo := new(serverInfo)
	err = json.NewDecoder(resp.Body).Decode(serverInfo)
	if err == nil {
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

// GetRecursorConfig gets all recursor config
func (client *Client) GetRecursorConfig() (map[string]string, error) {
	req, err := client.newRequestRecursor("GET", "/servers/localhost/config", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var config map[string]string
		err = json.NewDecoder(resp.Body).Decode(&config)
		if err != nil {
			return nil, err
		}

		return config, nil

	case http.StatusNotFound:
		// Map 404 to sentinel error
		return nil, ErrNotFound

	default:
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return nil, fmt.Errorf("error getting recursor config")
		}
		return nil, fmt.Errorf("error getting recursor config: %q", errorResp.ErrorMsg)
	}
}

// GetRecursorConfigValue gets a specific recursor config value
func (client *Client) GetRecursorConfigValue(name string) (string, error) {
	req, err := client.newRequestRecursor("GET", fmt.Sprintf("/servers/localhost/config/%s", name), nil)
	if err != nil {
		return "", err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var value string
		err = json.NewDecoder(resp.Body).Decode(&value)
		if err != nil {
			return "", err
		}

		return value, nil

	case http.StatusNotFound:
		// Map 404 to sentinel error
		return "", ErrNotFound

	default:
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return "", fmt.Errorf("error getting recursor config %s", name)
		}
		return "", fmt.Errorf("error getting recursor config %s: %q", name, errorResp.ErrorMsg)
	}
}

// SetRecursorConfigValue sets a recursor config value
func (client *Client) SetRecursorConfigValue(name string, value string) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}

	req, err := client.newRequestRecursor("PUT", fmt.Sprintf("/servers/localhost/config/%s", name), body)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error setting recursor config %s", name)
		}
		return fmt.Errorf("error setting recursor config %s: %q", name, errorResp.ErrorMsg)
	}

	return nil
}

// DeleteRecursorConfigValue deletes a recursor config value
func (client *Client) DeleteRecursorConfigValue(name string) error {
	req, err := client.newRequestRecursor("DELETE", fmt.Sprintf("/servers/localhost/config/%s", name), nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("[WARN] Error closing response body: %s", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil

	case http.StatusNotFound:
		// Map 404 to sentinel error
		return ErrNotFound

	default:
		errorResp := new(errorResponse)
		if err = json.NewDecoder(resp.Body).Decode(errorResp); err != nil {
			return fmt.Errorf("error deleting recursor config %s", name)
		}
		return fmt.Errorf("error deleting recursor config %s: %q", name, errorResp.ErrorMsg)
	}
}
