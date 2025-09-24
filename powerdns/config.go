package powerdns

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"

	"github.com/terraform-providers/terraform-provider-powerdns/pathorcontents"
)

// Config describes de configuration interface of this provider
type Config struct {
	ServerURL         string
	RecursorServerURL string
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
func (c *Config) Client() (*Client, error) {

	tlsConfig := &tls.Config{}

	if c.CACertificate != "" {
		caCert, _, err := pathorcontents.Read(c.CACertificate)
		if err != nil {
			return nil, fmt.Errorf("error reading CA Cert: %s", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCert))
		tlsConfig.RootCAs = caCertPool
	}

	if c.ClientCertFile != "" && c.ClientCertKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientCertKeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load client cert: %v", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	tlsConfig.InsecureSkipVerify = c.InsecureHTTPS

	client, err := NewClient(c.ServerURL, c.RecursorServerURL, c.APIKey, tlsConfig, c.CacheEnable, c.CacheMemorySize, c.CacheTTL)

	if err != nil {
		return nil, fmt.Errorf("error setting up PowerDNS client: %s", err)
	}

	log.Printf("[INFO] PowerDNS Client configured for server %s", c.ServerURL)

	return client, nil
}
