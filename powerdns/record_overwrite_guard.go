package powerdns

import (
	"context"
	"errors"
	"fmt"
)

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
	exists, err := client.RecordExists(ctx, zone, name, recordType)
	if err != nil {
		// A missing zone is reported by the zone resource itself, and any other
		// failure will resurface on the write that follows, so a failed check
		// must not block the apply on its own.
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("couldn't check whether %s %q already exists in zone %s: %w", recordType, name, zone, err)
	}

	if exists {
		return errRecordExists(resourceType, zone, name, recordType)
	}

	return nil
}
