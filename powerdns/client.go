package powerdns

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	freecache "github.com/coocood/freecache"
	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// DefaultSchema is the value used for the URL in case
// no schema is explicitly defined
var DefaultSchema = "https"

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
	ServerURL   string // Location of the server to use
	APIKey      string // REST API Static authentication key
	APIVersion  int    // API version to use; -1 until detected
	HTTP        *http.Client
	CacheEnable bool // Enable/Disable cache for REST API requests
	Cache       *freecache.Cache
	CacheSize   int
	CacheTTL    int

	// A single client is shared by every resource, and Terraform walks the
	// graph in parallel, so version detection must happen exactly once.
	apiVersionOnce sync.Once
	apiVersionErr  error
}

// NewBaseClient constructs a BaseClient with HTTP, TLS and cache configuration.
func NewBaseClient(serverURL string, apiKey string, configTLS *tls.Config, cacheEnable bool, cacheSizeMB string, cacheTTL int) (*BaseClient, error) {
	cleanURL, err := sanitizeURL(serverURL)
	if err != nil {
		return nil, fmt.Errorf("error while creating client: %s", err)
	}

	httpClient := cleanhttp.DefaultClient()
	httpClient.Transport.(*http.Transport).TLSClientConfig = configTLS

	base := &BaseClient{
		ServerURL: cleanURL,
		APIKey:    apiKey,
		HTTP:      httpClient,
		// -1 marks the version as not yet detected; see BaseClient.apiVersion.
		APIVersion:  -1,
		CacheEnable: cacheEnable,
		CacheTTL:    cacheTTL,
	}

	// Only allocate the cache when it will actually be read.
	if cacheEnable {
		cacheSize, err := strconv.Atoi(cacheSizeMB)
		if err != nil {
			return nil, fmt.Errorf("error while creating client: %s", err)
		}
		base.CacheSize = cacheSize * 1024 * 1024
		base.Cache = freecache.NewCache(base.CacheSize)
	}

	return base, nil
}

// PowerDNSClient is the concrete client used by the provider.
// ZoneInfo represents a PowerDNS zone object
type ZoneInfo struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	URL                string              `json:"url"`
	Kind               string              `json:"kind"`
	Catalog            string              `json:"catalog,omitempty"`
	DNSSec             bool                `json:"dnssec"`
	Serial             int64               `json:"serial"`
	Records            []Record            `json:"records,omitempty"`
	ResourceRecordSets []ResourceRecordSet `json:"rrsets,omitempty"`
	Account            string              `json:"account"`
	Nameservers        []string            `json:"nameservers,omitempty"`
	Masters            []string            `json:"masters,omitempty"`
	SoaEditAPI         string              `json:"soa_edit_api,omitempty"`
}

// ZoneInfoUpd is a limited subset for supported updates
type ZoneInfoUpd struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Catalog    string   `json:"catalog,omitempty"`
	SoaEditAPI string   `json:"soa_edit_api,omitempty"`
	Account    string   `json:"account"`
	Masters    []string `json:"masters,omitempty"`
}

// View represents a PowerDNS view object.
type View struct {
	ID    string   `json:"id,omitempty"`
	Name  string   `json:"name,omitempty"`
	Zones []string `json:"zones,omitempty"`
}

// Network represents a PowerDNS network object.
type Network struct {
	Network string `json:"network"`
	View    string `json:"view"`
}

// ZoneMetadata represents a single metadata kind with all configured values.
type ZoneMetadata struct {
	Kind     string   `json:"kind"`
	Metadata []string `json:"metadata"`
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
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	ChangeType string     `json:"changetype"`
	TTL        int        `json:"ttl"` // For API v1
	Records    []Record   `json:"records,omitempty"`
	Comments   *[]Comment `json:"comments,omitempty"`
}

type zonePatchRequest struct {
	RecordSets []ResourceRecordSet `json:"rrsets"`
}

// Comment represents a PowerDNS RRset comment.
type Comment struct {
	Content    string `json:"content"`
	Account    string `json:"account"`
	ModifiedAt int64  `json:"modified_at,omitempty"`
}

type errorResponse struct {
	ErrorMsg string `json:"error"`
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

func recordsFromRRSet(rrSet *ResourceRecordSet) []Record {
	if rrSet == nil {
		return nil
	}

	records := make([]Record, 0, len(rrSet.Records))
	for _, record := range rrSet.Records {
		records = append(records, Record{
			Name:     rrSet.Name,
			Type:     rrSet.Type,
			Content:  record.Content,
			TTL:      rrSet.TTL,
			Disabled: record.Disabled,
			SetPtr:   record.SetPtr,
		})
	}

	return records
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
	serverID string
}

// NewPowerDNSClient constructs the derived PowerDNS client used by the provider.
func NewPowerDNSClient(ctx context.Context, serverURL string, serverID string, apiKey string, configTLS *tls.Config, cacheEnable bool, cacheSizeMB string, cacheTTL int) (*PowerDNSClient, error) {
	base, err := NewBaseClient(serverURL, apiKey, configTLS, cacheEnable, cacheSizeMB, cacheTTL)
	if err != nil {
		return nil, err
	}
	if serverID == "" {
		serverID = "localhost"
	}
	return &PowerDNSClient{BaseClient: base, serverID: serverID}, nil
}

// serverEndpoint returns an API path scoped to the configured authoritative server.
func (client *PowerDNSClient) serverEndpoint(path string) string {
	return "/servers/" + pathEscape(client.serverID) + path
}

// zoneEndpoint returns the API path for a single zone.
func (client *PowerDNSClient) zoneEndpoint(zone string) string {
	return client.serverEndpoint("/zones/" + pathEscape(zone))
}

// zoneMetadataEndpoint returns the API path for one metadata kind of a zone.
func (client *PowerDNSClient) zoneMetadataEndpoint(zone, kind string) string {
	return client.zoneEndpoint(zone) + "/metadata/" + pathEscape(kind)
}

// viewEndpoint returns the API path for a single view.
func (client *PowerDNSClient) viewEndpoint(view string) string {
	return client.serverEndpoint("/views/" + pathEscape(view))
}

// networkEndpoint returns the API path for a single network.
func (client *PowerDNSClient) networkEndpoint(ip, prefixlen string) string {
	return client.serverEndpoint("/networks/" + pathEscape(ip) + "/" + pathEscape(prefixlen))
}

// ListZones returns all Zones of server, without records
func (client *PowerDNSClient) ListZones(ctx context.Context) ([]ZoneInfo, error) {
	var zoneInfos []ZoneInfo
	_, err := client.do(ctx, requestOptions{
		endpoint: client.serverEndpoint("/zones"),
		out:      &zoneInfos,
		describe: "listing zones",
	})
	if err != nil {
		return nil, err
	}

	return zoneInfos, nil
}

// GetZone gets a zone
func (client *PowerDNSClient) GetZone(ctx context.Context, name string) (ZoneInfo, error) {
	return client.getZone(ctx, name, false)
}

// GetZoneWithRRsets gets a zone including its RRsets.
func (client *PowerDNSClient) GetZoneWithRRsets(ctx context.Context, name string) (ZoneInfo, error) {
	return client.getZone(ctx, name, true)
}

func (client *PowerDNSClient) getZone(ctx context.Context, name string, includeRRsets bool) (ZoneInfo, error) {
	endpoint := client.zoneEndpoint(name)
	if includeRRsets {
		endpoint += "?rrsets=true"
	}

	var zoneInfo ZoneInfo
	_, err := client.do(ctx, requestOptions{
		endpoint:  endpoint,
		out:       &zoneInfo,
		describe:  fmt.Sprintf("getting zone: %s", name),
		logFields: map[string]any{"zone": name},
	})
	if err != nil {
		return ZoneInfo{}, err
	}

	return zoneInfo, nil
}

// ZoneExists checks if requested zone exists
func (client *PowerDNSClient) ZoneExists(ctx context.Context, name string) (bool, error) {
	statusCode, err := client.do(ctx, requestOptions{
		endpoint:  client.zoneEndpoint(name),
		okCodes:   []int{http.StatusOK, http.StatusNotFound},
		describe:  fmt.Sprintf("getting zone: %s", name),
		logFields: map[string]any{"zone": name},
	})
	if err != nil {
		return false, err
	}

	return statusCode == http.StatusOK, nil
}

// CreateZone creates a zone
func (client *PowerDNSClient) CreateZone(ctx context.Context, zoneInfo ZoneInfo) (ZoneInfo, error) {
	var createdZoneInfo ZoneInfo
	_, err := client.do(ctx, requestOptions{
		method:    http.MethodPost,
		endpoint:  client.serverEndpoint("/zones"),
		body:      zoneInfo,
		out:       &createdZoneInfo,
		okCodes:   []int{http.StatusCreated},
		describe:  fmt.Sprintf("creating zone: %s", zoneInfo.Name),
		logFields: map[string]any{"zone": zoneInfo.Name},
	})
	if err != nil {
		return ZoneInfo{}, err
	}

	return createdZoneInfo, nil
}

// UpdateZone updates a zone
func (client *PowerDNSClient) UpdateZone(ctx context.Context, name string, zoneInfo ZoneInfoUpd) error {
	_, err := client.do(ctx, requestOptions{
		method:    http.MethodPut,
		endpoint:  client.zoneEndpoint(name),
		body:      zoneInfo,
		okCodes:   []int{http.StatusNoContent},
		describe:  fmt.Sprintf("updating zone: %s", name),
		logFields: map[string]any{"zone": name},
	})
	if err != nil {
		return err
	}

	client.invalidateZoneCache(ctx, name)
	return nil
}

// DeleteZone deletes a zone
func (client *PowerDNSClient) DeleteZone(ctx context.Context, name string) error {
	_, err := client.do(ctx, requestOptions{
		method:    http.MethodDelete,
		endpoint:  client.zoneEndpoint(name),
		okCodes:   []int{http.StatusNoContent},
		describe:  fmt.Sprintf("deleting zone: %s", name),
		logFields: map[string]any{"zone": name},
	})
	if err != nil {
		return err
	}

	client.invalidateZoneCache(ctx, name)
	return nil
}

// ListZoneMetadata returns all domain metadata entries for a zone.
func (client *PowerDNSClient) ListZoneMetadata(ctx context.Context, zone string) ([]ZoneMetadata, error) {
	var metadata []ZoneMetadata
	_, err := client.do(ctx, requestOptions{
		endpoint:  client.zoneEndpoint(zone) + "/metadata",
		out:       &metadata,
		describe:  fmt.Sprintf("reading zone metadata: %s", zone),
		logFields: map[string]any{"zone": zone},
	})
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// GetZoneMetadata returns one metadata kind for a zone.
func (client *PowerDNSClient) GetZoneMetadata(ctx context.Context, zone string, kind string) (ZoneMetadata, error) {
	var metadata ZoneMetadata
	_, err := client.do(ctx, requestOptions{
		endpoint:  client.zoneMetadataEndpoint(zone, kind),
		out:       &metadata,
		describe:  fmt.Sprintf("reading zone metadata: %s (%s)", zone, kind),
		logFields: map[string]any{"zone": zone, "kind": kind},
	})
	if err != nil {
		return ZoneMetadata{}, err
	}

	return metadata, nil
}

// ReplaceZoneMetadata replaces all values for a metadata kind in a zone.
func (client *PowerDNSClient) ReplaceZoneMetadata(ctx context.Context, zone string, kind string, values []string) error {
	_, err := client.do(ctx, requestOptions{
		method:   http.MethodPut,
		endpoint: client.zoneMetadataEndpoint(zone, kind),
		body: ZoneMetadata{
			Kind:     kind,
			Metadata: values,
		},
		okCodes:   []int{http.StatusOK, http.StatusNoContent},
		describe:  fmt.Sprintf("replacing zone metadata: %s (%s)", zone, kind),
		logFields: map[string]any{"zone": zone, "kind": kind},
	})

	return err
}

// DeleteZoneMetadata deletes all values for a metadata kind in a zone.
func (client *PowerDNSClient) DeleteZoneMetadata(ctx context.Context, zone string, kind string) error {
	_, err := client.do(ctx, requestOptions{
		method:    http.MethodDelete,
		endpoint:  client.zoneMetadataEndpoint(zone, kind),
		okCodes:   []int{http.StatusNoContent},
		describe:  fmt.Sprintf("deleting zone metadata: %s (%s)", zone, kind),
		logFields: map[string]any{"zone": zone, "kind": kind},
	})

	return err
}

// getCachedZoneInfo returns the cached ZoneInfo for a zone. found is false when
// caching is disabled or the zone is simply not cached yet; only a malformed
// cache entry is reported as an error.
func (client *PowerDNSClient) getCachedZoneInfo(ctx context.Context, zone string) (info *ZoneInfo, found bool, err error) {
	if !client.CacheEnable || client.Cache == nil {
		return nil, false, nil
	}

	cached, err := client.Cache.Get([]byte(zone))
	if err != nil {
		// freecache reports an ordinary miss as an error; that is not a failure.
		return nil, false, nil
	}

	zoneInfo := new(ZoneInfo)
	if err := json.Unmarshal(cached, zoneInfo); err != nil {
		tflog.Warn(ctx, "Discarding malformed zone cache entry", map[string]any{
			"zone":  zone,
			"error": err.Error(),
		})
		client.Cache.Del([]byte(zone))
		return nil, false, nil
	}

	return zoneInfo, true, nil
}

// invalidateZoneCache drops a zone's cached records. Every write path calls
// this: without it a read issued later in the same apply can still be served
// the pre-write zone and be recorded as drift.
func (client *PowerDNSClient) invalidateZoneCache(ctx context.Context, zone string) {
	if !client.CacheEnable || client.Cache == nil {
		return
	}

	if client.Cache.Del([]byte(zone)) {
		tflog.Debug(ctx, "Invalidated zone cache", map[string]any{"zone": zone})
	}
}

// ListRecords returns all records in Zone
func (client *PowerDNSClient) ListRecords(ctx context.Context, zone string) ([]Record, error) {
	zoneInfo, found, err := client.getCachedZoneInfo(ctx, zone)
	if err != nil {
		return nil, err
	}

	if !found {
		zoneInfo = new(ZoneInfo)
		// A missing zone has no records, and callers rely on that: the
		// CheckDestroy helpers ask for the records of a zone Terraform has
		// just deleted. Anything other than 404 is a real failure and must not
		// be decoded into an empty result - a 401 would otherwise read as "the
		// zone is empty".
		statusCode, err := client.do(ctx, requestOptions{
			endpoint:  client.zoneEndpoint(zone) + "?rrsets=true",
			out:       zoneInfo,
			okCodes:   []int{http.StatusOK, http.StatusNotFound},
			describe:  fmt.Sprintf("listing records for zone: %s", zone),
			logFields: map[string]any{"zone": zone},
		})
		if err != nil {
			return nil, err
		}
		if statusCode == http.StatusNotFound {
			return []Record{}, nil
		}

		if client.CacheEnable && client.Cache != nil {
			cacheValue, err := json.Marshal(zoneInfo)
			if err != nil {
				return nil, err
			}

			if err := client.Cache.Set([]byte(zone), cacheValue, client.CacheTTL); err != nil {
				return nil, fmt.Errorf("the cache for REST API requests is enabled but the size isn't enough: cacheSize: %db: %w", client.CacheSize, err)
			}
		}
	}

	records := zoneInfo.Records
	// Convert the API v1 response to v0 record structure
	for i := range zoneInfo.ResourceRecordSets {
		records = append(records, recordsFromRRSet(&zoneInfo.ResourceRecordSets[i])...)
	}

	return records, nil
}

// ListRecordsInRRSet returns only records of specified name and type
func (client *PowerDNSClient) ListRecordsInRRSet(ctx context.Context, zone string, name string, tpe string) ([]Record, error) {
	allRecords, err := client.ListRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	records := make([]Record, 0, len(allRecords))
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

// GetRecordSetByID returns the full RRset (including comments) for the given ID.
func (client *PowerDNSClient) GetRecordSetByID(ctx context.Context, zone string, recID string) (*ResourceRecordSet, error) {
	name, tpe, err := parseID(recID)
	if err != nil {
		return nil, err
	}

	zoneInfo, err := client.GetZoneWithRRsets(ctx, zone)
	if err != nil {
		return nil, err
	}

	for i := range zoneInfo.ResourceRecordSets {
		rrSet := &zoneInfo.ResourceRecordSets[i]
		if strings.EqualFold(rrSet.Name, name) && strings.EqualFold(rrSet.Type, tpe) {
			return rrSet, nil
		}
	}

	return nil, nil
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

	_, err := client.do(ctx, requestOptions{
		method:   http.MethodPatch,
		endpoint: client.zoneEndpoint(zone),
		body: zonePatchRequest{
			RecordSets: []ResourceRecordSet{rrSet},
		},
		okCodes:   []int{http.StatusOK, http.StatusNoContent},
		describe:  fmt.Sprintf("creating record set: %s", rrSet.ID()),
		logFields: map[string]any{"zone": zone, "rrsetId": rrSet.ID()},
	})
	if err != nil {
		return "", err
	}

	client.invalidateZoneCache(ctx, zone)
	return rrSet.ID(), nil
}

// DeleteRecordSet deletes record set from Zone
func (client *PowerDNSClient) DeleteRecordSet(ctx context.Context, zone string, name string, tpe string) error {
	_, err := client.do(ctx, requestOptions{
		method:   http.MethodPatch,
		endpoint: client.zoneEndpoint(zone),
		body: zonePatchRequest{
			RecordSets: []ResourceRecordSet{
				{
					Name:       name,
					Type:       tpe,
					ChangeType: "DELETE",
				},
			},
		},
		okCodes:   []int{http.StatusOK, http.StatusNoContent},
		describe:  fmt.Sprintf("deleting record: %s %s", name, tpe),
		logFields: map[string]any{"zone": zone, "name": name, "type": tpe},
	})
	if err != nil {
		return err
	}

	client.invalidateZoneCache(ctx, zone)
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

// ListViews returns all configured views.
func (client *PowerDNSClient) ListViews(ctx context.Context) ([]string, error) {
	// GET /views answers with {"views": [...]}, not a bare array.
	var wrapper struct {
		Views []string `json:"views"`
	}
	_, err := client.do(ctx, requestOptions{
		endpoint: client.serverEndpoint("/views"),
		out:      &wrapper,
		describe: "listing views",
	})
	if err != nil {
		return nil, err
	}

	return wrapper.Views, nil
}

// GetView retrieves a specific view.
func (client *PowerDNSClient) GetView(ctx context.Context, viewName string) (*View, error) {
	var view View
	_, err := client.do(ctx, requestOptions{
		endpoint:    client.viewEndpoint(viewName),
		out:         &view,
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("getting view %s", viewName),
		logFields:   map[string]any{"view": viewName},
	})
	if err != nil {
		return nil, err
	}

	if view.Name == "" {
		view.Name = viewName
	}
	return &view, nil
}

// AddZoneToView associates a zone with a view.
func (client *PowerDNSClient) AddZoneToView(ctx context.Context, viewName, zoneName string) error {
	_, err := client.do(ctx, requestOptions{
		method:   http.MethodPost,
		endpoint: client.viewEndpoint(viewName),
		body: struct {
			Name string `json:"name"`
		}{Name: zoneName},
		okCodes:   []int{http.StatusOK, http.StatusCreated, http.StatusNoContent},
		describe:  fmt.Sprintf("adding zone %s to view %s", zoneName, viewName),
		hint:      viewsHint,
		logFields: map[string]any{"view": viewName, "zone": zoneName},
	})

	return err
}

// RemoveZoneFromView removes a zone from a view.
func (client *PowerDNSClient) RemoveZoneFromView(ctx context.Context, viewName, zoneName string) error {
	_, err := client.do(ctx, requestOptions{
		method:      http.MethodDelete,
		endpoint:    client.viewEndpoint(viewName) + "/" + pathEscape(zoneName),
		okCodes:     []int{http.StatusOK, http.StatusNoContent},
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("removing zone %s from view %s", zoneName, viewName),
		hint:        viewsHint,
		logFields:   map[string]any{"view": viewName, "zone": zoneName},
	})

	return err
}

// ListNetworks returns all configured networks.
func (client *PowerDNSClient) ListNetworks(ctx context.Context) ([]Network, error) {
	// GET /networks answers with {"networks": [...]}, not a bare array.
	var wrapper struct {
		Networks []Network `json:"networks"`
	}
	_, err := client.do(ctx, requestOptions{
		endpoint: client.serverEndpoint("/networks"),
		out:      &wrapper,
		describe: "listing networks",
	})
	if err != nil {
		return nil, err
	}

	return wrapper.Networks, nil
}

// GetNetwork retrieves a specific network definition.
func (client *PowerDNSClient) GetNetwork(ctx context.Context, ip, prefixlen string) (*Network, error) {
	var network Network
	_, err := client.do(ctx, requestOptions{
		endpoint:    client.networkEndpoint(ip, prefixlen),
		out:         &network,
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("getting network %s/%s", ip, prefixlen),
		logFields:   map[string]any{"ip": ip, "prefix_len": prefixlen},
	})
	if err != nil {
		return nil, err
	}

	return &network, nil
}

// SetNetwork creates or updates a network definition.
func (client *PowerDNSClient) SetNetwork(ctx context.Context, ip, prefixlen, view string) error {
	_, err := client.do(ctx, requestOptions{
		method:    http.MethodPut,
		endpoint:  client.networkEndpoint(ip, prefixlen),
		body:      Network{View: view},
		okCodes:   []int{http.StatusOK, http.StatusCreated, http.StatusNoContent},
		describe:  fmt.Sprintf("setting network %s/%s", ip, prefixlen),
		hint:      viewsHint,
		logFields: map[string]any{"ip": ip, "prefix_len": prefixlen, "view": view},
	})

	return err
}

// DeleteNetwork deletes a network definition by clearing its view.
func (client *PowerDNSClient) DeleteNetwork(ctx context.Context, ip, prefixlen string) error {
	_, err := client.do(ctx, requestOptions{
		method:      http.MethodPut,
		endpoint:    client.networkEndpoint(ip, prefixlen),
		body:        Network{View: ""},
		okCodes:     []int{http.StatusOK, http.StatusCreated, http.StatusNoContent},
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("deleting network %s/%s", ip, prefixlen),
		hint:        viewsHint,
		logFields:   map[string]any{"ip": ip, "prefix_len": prefixlen},
	})

	return err
}

// RecursorClient talks to the PowerDNS Recursor API.
type RecursorClient struct {
	*BaseClient
}

// serverEndpoint returns an API path scoped to the recursor server. The
// recursor exposes only "localhost", unlike the authoritative server.
func (client *RecursorClient) serverEndpoint(path string) string {
	return "/servers/localhost" + path
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
	base, err := NewBaseClient(recursorURL, apiKey, configTLS, false, "0", 0)
	if err != nil {
		return nil, err
	}
	return &RecursorClient{BaseClient: base}, nil
}

// GetForwardZone retrieves a specific recursor forward zone definition.
func (client *RecursorClient) GetForwardZone(ctx context.Context, name string) (*RecursorForwardZone, error) {
	var zone RecursorForwardZone
	_, err := client.do(ctx, requestOptions{
		endpoint:    client.serverEndpoint("/zones/" + pathEscape(name)),
		out:         &zone,
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("getting forward zone %s", name),
		logFields:   map[string]any{"zone": name},
	})
	if err != nil {
		// The recursor answers an unknown zone with a 422 rather than a 404,
		// so the absence has to be recognised from the message itself.
		if strings.Contains(err.Error(), "Could not find domain") {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &zone, nil
}

// CreateForwardZone creates a recursor forward zone.
func (client *RecursorClient) CreateForwardZone(ctx context.Context, zone *RecursorForwardZone) error {
	_, err := client.do(ctx, requestOptions{
		method:    http.MethodPost,
		endpoint:  client.serverEndpoint("/zones"),
		body:      zone,
		out:       nil,
		okCodes:   []int{http.StatusCreated},
		describe:  fmt.Sprintf("creating forward zone %s", zone.Name),
		hint:      recursorMessageHint,
		logFields: map[string]any{"zone": zone.Name},
	})

	return err
}

// DeleteForwardZone deletes a recursor forward zone.
func (client *RecursorClient) DeleteForwardZone(ctx context.Context, name string) error {
	_, err := client.do(ctx, requestOptions{
		method:      http.MethodDelete,
		endpoint:    client.serverEndpoint("/zones/" + pathEscape(name)),
		okCodes:     []int{http.StatusNoContent, http.StatusOK},
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("deleting forward zone %s", name),
		hint:        recursorMessageHint,
		logFields:   map[string]any{"zone": name},
	})

	return err
}

// GetConfig retrieves a single recursor config setting.
func (client *RecursorClient) GetConfig(ctx context.Context, name string) (*RecursorConfigSetting, error) {
	var setting RecursorConfigSetting
	_, err := client.do(ctx, requestOptions{
		endpoint:    client.serverEndpoint("/config/" + pathEscape(name)),
		out:         &setting,
		notFoundErr: ErrNotFound,
		describe:    fmt.Sprintf("getting recursor config %s", name),
		logFields:   map[string]any{"name": name},
	})
	if err != nil {
		return nil, err
	}

	return &setting, nil
}

// SetConfig changes a single recursor config setting.
func (client *RecursorClient) SetConfig(ctx context.Context, name string, values []string) error {
	_, err := client.do(ctx, requestOptions{
		method:   http.MethodPut,
		endpoint: client.serverEndpoint("/config/" + pathEscape(name)),
		body: RecursorConfigSetting{
			Name:  name,
			Value: values,
		},
		describe:  fmt.Sprintf("setting recursor config %s", name),
		hint:      recursorMessageHint,
		logFields: map[string]any{"name": name},
	})

	return err
}
