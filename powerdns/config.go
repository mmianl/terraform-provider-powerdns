package powerdns

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/terraform-providers/terraform-provider-powerdns/pathorcontents"
)

// Config describes the configuration interface of this provider
type Config struct {
	ServerURL         string
	ServerID          string
	RecursorServerURL string
	RecursorServerID  string
	APIKey            string
	ClientCertFile    string
	ClientCertKeyFile string
	InsecureHTTPS     bool
	CACertificate     string
	CacheEnable       bool
	CacheMemorySize   string
	CacheTTL          int
}

// Client returns a new client for accessing PowerDNS
func (c *Config) Clients(ctx context.Context) (*PowerDNSClient, *RecursorClient, error) {
	// Go's own minimum is already TLS 1.2, but leaving it implicit means the
	// floor moves if that default ever changes. 1.2 rather than 1.3 because the
	// PowerDNS API is often published through a front end that has not moved to
	// 1.3, and failing to reach the server is worse than the difference.
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load custom CA bundle if provided
	if c.CACertificate != "" {
		caCert, _, err := pathorcontents.Read(c.CACertificate)
		if err != nil {
			return nil, nil, fmt.Errorf("error reading CA Cert: %s", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCert))
		tlsConfig.RootCAs = caCertPool

		tflog.Debug(ctx, "Loaded custom CA certificate for PowerDNS client")
	}

	// Load mTLS client certificate if provided
	if c.ClientCertFile != "" && c.ClientCertKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientCertKeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to load client cert: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		tflog.Debug(ctx, "Loaded client certificate/key for PowerDNS client")
	}

	// Optionally disable TLS verification
	tlsConfig.InsecureSkipVerify = c.InsecureHTTPS
	if c.InsecureHTTPS {
		tflog.Warn(ctx, "TLS certificate verification is disabled for PowerDNS client")
	}

	pdnsClient, err := NewPowerDNSClient(
		ctx,
		c.ServerURL,
		c.ServerID,
		c.APIKey,
		tlsConfig,
		c.CacheEnable,
		c.CacheMemorySize,
		c.CacheTTL,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error setting up PowerDNS client: %s", err)
	}

	// Attach some persistent fields for follow-up logs if callers reuse ctx
	ctx = tflog.SetField(ctx, "server_url", c.ServerURL)
	ctx = tflog.SetField(ctx, "recursor_server_url", c.RecursorServerURL)
	ctx = tflog.SetField(ctx, "cache_enabled", c.CacheEnable)
	ctx = tflog.SetField(ctx, "cache_ttl_sec", c.CacheTTL)

	tflog.Info(ctx, "PowerDNS client configured")

	if c.RecursorServerURL != "" {
		recursorClient, err := NewRecursorClient(
			ctx,
			c.RecursorServerURL,
			c.RecursorServerID,
			c.APIKey,
			tlsConfig,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("error setting up Recursor client: %s", err)
		}

		tflog.Info(ctx, "Recursor client configured")

		return pdnsClient, recursorClient, nil
	}

	return pdnsClient, nil, nil
}
