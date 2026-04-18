package powerdns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandRRSetCommentsPreservesOrder(t *testing.T) {
	comments := expandRRSetComments([]interface{}{"managed-by=terraform", "owner=dns-team"})

	if assert.Len(t, comments, 2) {
		assert.Equal(t, "managed-by=terraform", comments[0].Content)
		assert.Equal(t, "owner=dns-team", comments[1].Content)
	}
}

func TestExpandRRSetCommentsPreservesBlankValues(t *testing.T) {
	comments := expandRRSetComments([]interface{}{"managed-by=terraform", "   ", "", "owner=dns-team"})

	if assert.Len(t, comments, 4) {
		assert.Equal(t, "managed-by=terraform", comments[0].Content)
		assert.Equal(t, "   ", comments[1].Content)
		assert.Equal(t, "", comments[2].Content)
		assert.Equal(t, "owner=dns-team", comments[3].Content)
	}
}

func TestFlattenRRSetCommentsPreservesOrder(t *testing.T) {
	comments := []Comment{
		{Content: "managed-by=terraform"},
		{Content: "owner=dns-team"},
	}
	flattened := flattenRRSetComments(&comments)

	assert.Equal(t, []string{"managed-by=terraform", "owner=dns-team"}, flattened)
}

func TestResourceRecordSetMarshalJSONIncludesEmptyCommentsWhenConfigured(t *testing.T) {
	comments := []Comment{}
	encoded, err := json.Marshal(ResourceRecordSet{
		Name:     "www.example.com.",
		Type:     "A",
		TTL:      300,
		Comments: &comments,
	})
	if !assert.NoError(t, err) {
		return
	}

	assert.Contains(t, string(encoded), `"comments":[]`)
}

func TestResourceRecordSetMarshalJSONOmitsCommentsWhenUnconfigured(t *testing.T) {
	encoded, err := json.Marshal(ResourceRecordSet{
		Name: "www.example.com.",
		Type: "A",
		TTL:  300,
	})
	if !assert.NoError(t, err) {
		return
	}

	assert.NotContains(t, string(encoded), `"comments"`)
}

func TestRecordsFromRRSetHydratesInheritedFields(t *testing.T) {
	rrSet := &ResourceRecordSet{
		Name: "www.example.com.",
		Type: "A",
		TTL:  300,
		Records: []Record{
			{Content: "192.0.2.1", Disabled: true, SetPtr: true},
			{Content: "192.0.2.2"},
		},
	}

	records := recordsFromRRSet(rrSet)

	if assert.Len(t, records, 2) {
		assert.Equal(t, Record{
			Name:     "www.example.com.",
			Type:     "A",
			Content:  "192.0.2.1",
			TTL:      300,
			Disabled: true,
			SetPtr:   true,
		}, records[0])
		assert.Equal(t, Record{
			Name:    "www.example.com.",
			Type:    "A",
			Content: "192.0.2.2",
			TTL:     300,
		}, records[1])
	}
}

func TestRRSetDisabledReturnsTrueWhenAllRecordsDisabled(t *testing.T) {
	assert.True(t, rrSetDisabled([]Record{
		{Disabled: true},
		{Disabled: true},
	}))
}

func TestRRSetDisabledReturnsFalseWhenAnyRecordEnabled(t *testing.T) {
	assert.False(t, rrSetDisabled([]Record{
		{Disabled: true},
		{Disabled: false},
	}))
}

func TestRRSetDisabledReturnsFalseWhenNoRecordsExist(t *testing.T) {
	assert.False(t, rrSetDisabled(nil))
}

func TestValidateRRSetCommentRejectsWhitespaceOnlyValues(t *testing.T) {
	_, errs := validateRRSetComment("   ", "comments.0")
	if assert.Len(t, errs, 1) {
		assert.Contains(t, errs[0].Error(), "must not be empty or whitespace only")
	}
}

func TestValidateRRSetCommentAcceptsNonEmptyValues(t *testing.T) {
	_, errs := validateRRSetComment("managed-by=terraform", "comments.0")
	assert.Empty(t, errs)
}
