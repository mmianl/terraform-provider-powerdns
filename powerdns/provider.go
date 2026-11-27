package powerdns

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type ProviderClients struct {
	PDNS     *PowerDNSClient
	Recursor *RecursorClient
}

var _ provider.Provider = &PowerDNSProvider{}

// PowerDNSProvider implements the provider on the Terraform Plugin Framework
// (protocol version 6).
type PowerDNSProvider struct {
	version string

	mu      sync.RWMutex
	clients *ProviderClients
}

// New returns a provider factory for the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PowerDNSProvider{version: version}
	}
}

// Meta returns the clients created by the last Configure call.
func (p *PowerDNSProvider) Meta() any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.clients
}

type providerModel struct {
	APIKey            types.String `tfsdk:"api_key"`
	ClientCertFile    types.String `tfsdk:"client_cert_file"`
	ClientCertKeyFile types.String `tfsdk:"client_cert_key_file"`
	ServerURL         types.String `tfsdk:"server_url"`
	ServerID          types.String `tfsdk:"server_id"`
	InsecureHTTPS     types.Bool   `tfsdk:"insecure_https"`
	CACertificate     types.String `tfsdk:"ca_certificate"`
	CacheRequests     types.Bool   `tfsdk:"cache_requests"`
	CacheMemSize      types.String `tfsdk:"cache_mem_size"`
	CacheTTL          types.Int64  `tfsdk:"cache_ttl"`
	RequestTimeout    types.Int64  `tfsdk:"request_timeout"`
	RecursorServerURL types.String `tfsdk:"recursor_server_url"`
	RecursorServerID  types.String `tfsdk:"recursor_server_id"`
}

func (p *PowerDNSProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "powerdns"
	resp.Version = p.version
}

func (p *PowerDNSProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "REST API authentication API key. Can also be set via PDNS_API_KEY.",
			},
			"client_cert_file": schema.StringAttribute{
				Optional:    true,
				Description: "REST API authentication client certificate file path (.crt). Can also be set via PDNS_CLIENT_CERT_FILE.",
			},
			"client_cert_key_file": schema.StringAttribute{
				Optional:    true,
				Description: "REST API authentication client certificate key file path (.key). Can also be set via PDNS_CLIENT_CERT_KEY_FILE.",
			},
			"server_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the PowerDNS server (e.g., https://pdns.example.com). Can also be set via PDNS_SERVER_URL.",
			},
			"server_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of the PowerDNS Authoritative Server to manage. Defaults to localhost. Can also be set via PDNS_SERVER_ID.",
			},
			"insecure_https": schema.BoolAttribute{
				Optional:    true,
				Description: "Disable verification of the PowerDNS server's TLS certificate. Also via PDNS_INSECURE_HTTPS.",
			},
			"ca_certificate": schema.StringAttribute{
				Optional:    true,
				Description: "Content or path of a Root CA to verify the server certificate. Also via PDNS_CACERT.",
			},
			"cache_requests": schema.BoolAttribute{
				Optional:    true,
				Description: "Enable caching of REST API requests. Also via PDNS_CACHE_REQUESTS.",
			},
			"cache_mem_size": schema.StringAttribute{
				Optional:    true,
				Description: "Cache memory size in MB. Defaults to 100. Also via PDNS_CACHE_MEM_SIZE.",
			},
			"cache_ttl": schema.Int64Attribute{
				Optional:    true,
				Description: "Cache TTL in seconds. Defaults to 30. Also via PDNS_CACHE_TTL.",
			},
			"request_timeout": schema.Int64Attribute{
				Optional:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(0)},
				Description: "Timeout in seconds for a single API request. 0 disables the timeout. Defaults to 60. Also via PDNS_REQUEST_TIMEOUT.",
			},
			"recursor_server_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the PowerDNS recursor server. Also via PDNS_RECURSOR_SERVER_URL.",
			},
			"recursor_server_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of the PowerDNS Recursor to manage. Defaults to localhost. Can also be set via PDNS_RECURSOR_SERVER_ID.",
			},
		},
	}
}

// envString returns the configured value, falling back to the environment
// variable and then to def.
func envString(v types.String, env, def string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	if e, ok := os.LookupEnv(env); ok && e != "" {
		return e
	}
	return def
}

func envBool(v types.Bool, env string, def bool) (bool, error) {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool(), nil
	}
	if e, ok := os.LookupEnv(env); ok && e != "" {
		b, err := strconv.ParseBool(e)
		if err != nil {
			return false, fmt.Errorf("%s: %w", env, err)
		}
		return b, nil
	}
	return def, nil
}

func envInt(v types.Int64, env string, def int) (int, error) {
	if !v.IsNull() && !v.IsUnknown() {
		return int(v.ValueInt64()), nil
	}
	if e, ok := os.LookupEnv(env); ok && e != "" {
		n, err := strconv.Atoi(e)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", env, err)
		}
		return n, nil
	}
	return def, nil
}

// resolveConfig merges the provider block with environment variables and
// defaults.
func resolveConfig(m providerModel) (Config, error) {
	var err error
	cfg := Config{
		APIKey:            envString(m.APIKey, "PDNS_API_KEY", ""),
		ClientCertFile:    envString(m.ClientCertFile, "PDNS_CLIENT_CERT_FILE", ""),
		ClientCertKeyFile: envString(m.ClientCertKeyFile, "PDNS_CLIENT_CERT_KEY_FILE", ""),
		ServerURL:         envString(m.ServerURL, "PDNS_SERVER_URL", ""),
		ServerID:          envString(m.ServerID, "PDNS_SERVER_ID", "localhost"),
		RecursorServerURL: envString(m.RecursorServerURL, "PDNS_RECURSOR_SERVER_URL", ""),
		RecursorServerID:  envString(m.RecursorServerID, "PDNS_RECURSOR_SERVER_ID", "localhost"),
		CACertificate:     envString(m.CACertificate, "PDNS_CACERT", ""),
		CacheMemorySize:   envString(m.CacheMemSize, "PDNS_CACHE_MEM_SIZE", "100"),
	}
	if cfg.InsecureHTTPS, err = envBool(m.InsecureHTTPS, "PDNS_INSECURE_HTTPS", false); err != nil {
		return cfg, err
	}
	if cfg.CacheEnable, err = envBool(m.CacheRequests, "PDNS_CACHE_REQUESTS", false); err != nil {
		return cfg, err
	}
	if cfg.CacheTTL, err = envInt(m.CacheTTL, "PDNS_CACHE_TTL", 30); err != nil {
		return cfg, err
	}
	if cfg.RequestTimeout, err = envInt(m.RequestTimeout, "PDNS_REQUEST_TIMEOUT", 60); err != nil {
		return cfg, err
	}
	if cfg.RequestTimeout < 0 {
		return cfg, fmt.Errorf("request_timeout must be at least 0, got %d", cfg.RequestTimeout)
	}
	return cfg, nil
}

func (p *PowerDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unknown values (for example, derived from another resource) cannot be
	// used to build a client.
	for name, unknown := range map[string]bool{
		"server_url": data.ServerURL.IsUnknown(),
		"api_key":    data.APIKey.IsUnknown(),
	} {
		if unknown {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				"Unknown PowerDNS configuration",
				fmt.Sprintf("The provider cannot be configured while %q is unknown. Set it statically or via the environment.", name),
			)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := resolveConfig(data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	// Runtime validation of required arguments with env var fallback
	if config.ServerURL == "" {
		resp.Diagnostics.AddError(
			"Missing required configuration",
			"The 'server_url' must be set (via provider configuration or PDNS_SERVER_URL).",
		)
		return
	}

	ctx = tflog.SetField(ctx, "server_url", config.ServerURL)
	ctx = tflog.SetField(ctx, "server_id", config.ServerID)
	if config.RecursorServerURL != "" {
		ctx = tflog.SetField(ctx, "recursor_server_url", config.RecursorServerURL)
		ctx = tflog.SetField(ctx, "recursor_server_id", config.RecursorServerID)
	}
	tflog.Debug(ctx, "Initializing PowerDNS client")

	pdnsClient, recursorClient, err := config.Clients(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create PowerDNS client", err.Error())
		return
	}

	clients := &ProviderClients{
		PDNS:     pdnsClient,
		Recursor: recursorClient, // may be nil if recursor_server_url not set
	}

	p.mu.Lock()
	p.clients = clients
	p.mu.Unlock()

	resp.ResourceData = clients
	resp.DataSourceData = clients
	tflog.Info(ctx, "PowerDNS client(s) initialized")
}

func (p *PowerDNSProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newZoneResource,
		newViewZoneAssociationResource,
		newNetworkResource,
		newZoneMetadataResource,
		newRecordResource,
		newRecordSOAResource,
		newPTRRecordResource,
		newReverseZoneResource,
		newRecursorConfigResource,
		newRecursorForwardZoneResource,
		newZoneCryptokeyResource,
		newZoneNotifyResource,
		newZoneRectifyResource,
		newTSIGKeyResource,
	}
}

func (p *PowerDNSProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newReverseZoneDataSource,
		newRecordDataSource,
		newRecordSOADataSource,
		newZoneDataSource,
		newZoneMetadataDataSource,
		newZoneMetadataListDataSource,
		newZoneCryptokeysDataSource,
		newTSIGKeyDataSource,
	}
}
