package powerdns

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestStringsFromSet(t *testing.T) {
	ctx := context.Background()

	t.Run("returns sorted values", func(t *testing.T) {
		set := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("zeta"), types.StringValue("alpha"), types.StringValue("beta")})

		var diags diag.Diagnostics
		got := stringsFromSet(ctx, set, &diags)

		assert.False(t, diags.HasError())
		assert.Equal(t, []string{"alpha", "beta", "zeta"}, got)
	})

	t.Run("returns empty slice for empty or null set", func(t *testing.T) {
		var diags diag.Diagnostics
		for _, set := range []types.Set{types.SetValueMust(types.StringType, []attr.Value{}), types.SetNull(types.StringType)} {
			got := stringsFromSet(ctx, set, &diags)
			assert.NotNil(t, got)
			assert.Equal(t, 0, len(got))
		}
	})
}

func TestGetMetadataValues(t *testing.T) {
	metadata := []ZoneMetadata{
		{
			Kind:     "ALSO-NOTIFY",
			Metadata: []string{"192.0.2.10", "192.0.2.11:5300"},
		},
		{
			Kind:     "ALLOW-AXFR-FROM",
			Metadata: []string{"2001:db8::/48", "AUTO-NS"},
		},
	}

	t.Run("matches kind case-insensitively and sorts values", func(t *testing.T) {
		got := getMetadataValues(metadata, "allow-axfr-from")

		assert.Equal(t, []string{"2001:db8::/48", "AUTO-NS"}, got)
	})

	t.Run("returns a copy, not backing source slice", func(t *testing.T) {
		got := getMetadataValues(metadata, "ALSO-NOTIFY")
		got[0] = "changed"

		assert.Equal(t, "192.0.2.10", metadata[0].Metadata[0])
	})

	t.Run("returns empty slice when kind missing", func(t *testing.T) {
		got := getMetadataValues(metadata, "NON-EXISTENT")

		assert.NotNil(t, got)
		assert.Equal(t, 0, len(got))
	})
}
