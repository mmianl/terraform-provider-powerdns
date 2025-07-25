package powerdns

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Provider returns a schema.Provider for PowerDNS.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_API_KEY", nil),
				Description: "REST API authentication api key",
			},
			"client_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CLIENT_CERT_FILE", nil),
				Description: "REST API authentication client certificate file (.crt)",
			},
			"client_cert_key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CLIENT_CERT_KEY_FILE", nil),
				Description: "REST API authentication client certificate key file (.key)",
			},
			"server_url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_SERVER_URL", nil),
				Description: "Location of PowerDNS server",
			},
			"insecure_https": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_INSECURE_HTTPS", false),
				Description: "Disable verification of the PowerDNS server's TLS certificate",
			},
			"ca_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACERT", ""),
				Description: "Content or path of a Root CA to be used to verify PowerDNS's SSL certificate",
			},
			"cache_requests": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACHE_REQUESTS", false),
				Description: "Enable cache REST API requests",
			},
			"cache_mem_size": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACHE_MEM_SIZE", "100"),
				Description: "Set cache memory size in MB",
			},
			"cache_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PDNS_CACHE_TTL", 30),
				Description: "Set cache TTL in seconds",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"powerdns_zone":         resourcePDNSZone(),
			"powerdns_record":       resourcePDNSRecord(),
			"powerdns_ptr_record":   resourcePDNSPTRRecord(),
			"powerdns_reverse_zone": resourcePDNSReverseZone(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(data *schema.ResourceData) (interface{}, error) {
	config := Config{
		APIKey:            data.Get("api_key").(string),
		ClientCertFile:    data.Get("client_cert_file").(string),
		ClientCertKeyFile: data.Get("client_cert_key_file").(string),
		ServerURL:         data.Get("server_url").(string),
		InsecureHTTPS:     data.Get("insecure_https").(bool),
		CACertificate:     data.Get("ca_certificate").(string),
		CacheEnable:       data.Get("cache_requests").(bool),
		CacheMemorySize:   data.Get("cache_mem_size").(string),
		CacheTTL:          data.Get("cache_ttl").(int),
	}

	return config.Client()
}
