package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// recordCreateLocks serializes the check-then-write in guardRecordOverwrite
// per zone/name/type. PowerDNS's API has no compare-and-swap for record sets,
// so this only closes the race between resources created concurrently by the
// same provider process; it cannot protect against separate terraform apply
// runs racing each other.
var recordCreateLocks sync.Map

func lockRecordCreate(zone, name, recordType string) func() {
	// Match DNS names and types case-insensitively, while keeping the zone's
	// variant suffix verbatim, just as the zone cache does.
	// NUL bytes (\x00) separate the components so different zone/name/type
	// combinations cannot produce the same key; these values cannot contain NUL.
	key := string(zoneCacheKey(zone)) + "\x00" + strings.ToLower(name) + "\x00" + strings.ToLower(recordType)
	muAny, _ := recordCreateLocks.LoadOrStore(key, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// errRecordExists explains that a record set is already present on the server.
// PowerDNS stores one record set per name and type and its API upserts them, so
// creating one that already exists would replace whatever is there. Refusing is
// the only way to keep an unmanaged record from being destroyed by an apply.
func errRecordExists(resourceType, zone, name, recordType string) error {
	return fmt.Errorf(
		"a %s record set for %q already exists in zone %s and is not managed by this resource. "+
			"Creating it would replace the existing record. Import it instead:\n"+
			"  terraform import %s.<name> '{\"zone\": %q, \"id\": \"%s:::%s\"}'",
		recordType, name, zone, resourceType, zone, name, recordType)
}

// guardRecordOverwrite fails when a record set already exists on the server.
// Callers must only use it on create: an update legitimately rewrites the
// record set it already owns.
func guardRecordOverwrite(ctx context.Context, client *PowerDNSClient, resourceType, zone, name, recordType string) error {
	// Provider aliases have separate caches, so another alias's write may leave
	// this client's snapshot stale. Always check the server while holding the
	// create lock rather than trusting the request cache.
	zoneInfo, err := client.GetZoneWithRRsets(ctx, zone)
	if err != nil {
		// A missing zone is reported by the zone resource itself. Other failures
		// must block the create rather than allow an unchecked overwrite.
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("couldn't check whether %s %q already exists in zone %s: %w", recordType, name, zone, err)
	}

	for _, rrSet := range zoneInfo.ResourceRecordSets {
		if strings.EqualFold(rrSet.Name, name) && strings.EqualFold(rrSet.Type, recordType) {
			return errRecordExists(resourceType, zone, name, recordType)
		}
	}
	// Older API responses carry records directly instead of RRsets.
	for _, record := range zoneInfo.Records {
		if strings.EqualFold(record.Name, name) && strings.EqualFold(record.Type, recordType) {
			return errRecordExists(resourceType, zone, name, recordType)
		}
	}

	return nil
}
