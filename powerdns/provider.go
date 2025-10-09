package powerdns

import (
	"context"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Provider returns a schema.Provider for PowerDNS.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_API_KEY", nil),
				Description: "REST API authentication API key. Can also be set via PDNS_API_KEY.",
			},
			"client_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CLIENT_CERT_FILE", nil),
				Description: "REST API authentication client certificate file path (.crt). Can also be set via PDNS_CLIENT_CERT_FILE.",
			},
			"client_cert_key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CLIENT_CERT_KEY_FILE", nil),
				Description: "REST API authentication client certificate key file path (.key). Can also be set via PDNS_CLIENT_CERT_KEY_FILE.",
			},
			"server_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_SERVER_URL", nil),
				Description: "Base URL of the PowerDNS server (e.g., https://pdns.example.com). Can also be set via PDNS_SERVER_URL.",
			},
			"insecure_https": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_INSECURE_HTTPS", false),
				Description: "Disable verification of the PowerDNS server's TLS certificate. Also via PDNS_INSECURE_HTTPS.",
			},
			"ca_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACERT", ""),
				Description: "Content or path of a Root CA to verify the server certificate. Also via PDNS_CACERT.",
			},
			"cache_requests": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACHE_REQUESTS", false),
				Description: "Enable caching of REST API requests. Also via PDNS_CACHE_REQUESTS.",
			},
			"cache_mem_size": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACHE_MEM_SIZE", "100"),
				Description: "Cache memory size in MB. Also via PDNS_CACHE_MEM_SIZE.",
			},
			"cache_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACHE_TTL", 30),
				Description: "Cache TTL in seconds. Also via PDNS_CACHE_TTL.",
			},
			"recursor_server_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_RECURSOR_SERVER_URL", nil),
				Description: "Base URL of the PowerDNS recursor server. Also via PDNS_RECURSOR_SERVER_URL.",
			},
			"dnsdist_server_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_DNSDIST_SERVER_URL", nil),
				Description: "Location of PowerDNS DNSdist server",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"powerdns_zone":                  resourcePDNSZone(),
			"powerdns_record":                resourcePDNSRecord(),
			"powerdns_ptr_record":            resourcePDNSPTRRecord(),
			"powerdns_reverse_zone":          resourcePDNSReverseZone(),
			"powerdns_recursor_config":       resourcePDNSRecursorConfig(),
			"powerdns_recursor_forward_zone": resourcePDNSRecursorForwardZone(),
			"powerdns_dnsdist_rule":          resourcePDNSDNSdistRule(),
		},

		DataSourcesMap: map[string]*schema.Resource{
			"powerdns_reverse_zone":       dataSourcePDNSReverseZone(),
			"powerdns_zone":               dataSourcePDNSZone(),
			"powerdns_dnsdist_statistics": dataSourcePDNSDNSdistStatistics(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, data *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	config := Config{
		APIKey:            data.Get("api_key").(string),
		ClientCertFile:    data.Get("client_cert_file").(string),
		ClientCertKeyFile: data.Get("client_cert_key_file").(string),
		ServerURL:         data.Get("server_url").(string),
		RecursorServerURL: data.Get("recursor_server_url").(string),
		DNSdistServerURL:  data.Get("dnsdist_server_url").(string),
		InsecureHTTPS:     data.Get("insecure_https").(bool),
		CACertificate:     data.Get("ca_certificate").(string),
		CacheEnable:       data.Get("cache_requests").(bool),
		CacheMemorySize:   data.Get("cache_mem_size").(string),
		CacheTTL:          data.Get("cache_ttl").(int),
	}

	// Runtime validation of required arguments with env var fallback
	if config.ServerURL == "" {
		return nil, diag.Diagnostics{
			diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Missing required configuration",
				Detail:   "The 'server_url' must be set (via provider configuration or PDNS_SERVER_URL).",
			},
		}
	}

	tflog.SetField(ctx, "server_url", config.ServerURL)
	if config.RecursorServerURL != "" {
		tflog.SetField(ctx, "recursor_server_url", config.RecursorServerURL)
	}
	tflog.Debug(ctx, "Initializing PowerDNS client")

	client, err := config.Client(ctx)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to create PowerDNS client",
			Detail:   err.Error(),
		})
		return nil, diags
	}

	tflog.Info(ctx, "PowerDNS client initialized")
	return client, diags
}
