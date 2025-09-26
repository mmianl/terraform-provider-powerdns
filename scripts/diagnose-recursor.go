package main

import (
	"fmt"
	"log"
	"os"

	"github.com/terraform-providers/terraform-provider-powerdns/powerdns"
)

func main() {
	serverURL := os.Getenv("PDNS_SERVER_URL")
	recursorServerURL := os.Getenv("PDNS_RECURSOR_SERVER_URL")
	apiKey := os.Getenv("PDNS_API_KEY")

	if serverURL == "" {
		log.Fatal("PDNS_SERVER_URL environment variable is required")
	}
	if recursorServerURL == "" {
		log.Fatal("PDNS_RECURSOR_SERVER_URL environment variable is required")
	}
	if apiKey == "" {
		log.Fatal("PDNS_API_KEY environment variable is required")
	}

	// Create client
	client, err := powerdns.NewClient(serverURL, recursorServerURL, apiKey, nil, false, "100", 30)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Printf("Testing connection to recursor server: %s\n", recursorServerURL)

	// Test basic connectivity by trying to get server info
	_, err = client.GetRecursorConfigValue("version")
	if err != nil {
		log.Printf("Warning: Could not get version (this might be normal): %v", err)
	} else {
		fmt.Println("✓ Recursor server is accessible")
	}

	// Test configuration options categorized by type
	testConfigs := map[string][]string{
		"Read/Write (R/W)": {
			"allow-from",
			"forward-zones",
			"dont-throttle-names",
			"minimum-ttl-override",
			"max-cache-entries",
			"max-packetcache-entries",
		},
		"Read Only (R)": {
			"query-local-address",
			"local-address",
			"version",
			"threads",
		},
		"Startup Only (S)": {
			"local-port",
			"max-qperq",
			"dnssec",
			"chroot",
		},
	}

	fmt.Println("\nTesting configuration options by category:")
	for category, configs := range testConfigs {
		fmt.Printf("\n%s:\n", category)
		for _, configName := range configs {
			value, err := client.GetRecursorConfigValue(configName)
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", configName, err)
			} else {
				fmt.Printf("  ✓ %s: %s\n", configName, value)
			}
		}
	}

	// Try to set test configuration values for writable configs
	fmt.Println("\nTesting configuration write access:")
	writableTestConfigs := map[string]string{
		"allow-from":           "192.168.1.0/24",
		"minimum-ttl-override": "60",
		"dont-throttle-names":  "example.com",
	}

	for testConfig, testValue := range writableTestConfigs {
		fmt.Printf("Attempting to set %s = %s...\n", testConfig, testValue)
		err = client.SetRecursorConfigValue(testConfig, testValue)
		if err != nil {
			fmt.Printf("✗ Failed to set %s: %v\n", testConfig, err)
		} else {
			fmt.Printf("✓ Successfully set %s\n", testConfig)

			// Verify the value was set
			verifyValue, err := client.GetRecursorConfigValue(testConfig)
			if err != nil {
				fmt.Printf("✗ Failed to verify %s: %v\n", testConfig, err)
			} else if verifyValue != testValue {
				fmt.Printf("✗ Value mismatch: expected %s, got %s\n", testValue, verifyValue)
			} else {
				fmt.Printf("✓ Value verified: %s\n", verifyValue)
			}
		}
	}
}
