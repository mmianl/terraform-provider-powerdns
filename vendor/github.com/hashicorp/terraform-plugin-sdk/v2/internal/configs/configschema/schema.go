// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package configschema

import (
	"github.com/hashicorp/go-cty/cty"
)

// StringKind represents the format a string is in.
type StringKind int

const (
	// StringPlain indicates a string is plain-text and requires no processing for display.
	StringPlain StringKind = iota
	// StringMarkdown indicates a string is in markdown format and may
	// require additional processing to display.
	StringMarkdown
)

// Block represents a configuration block.
//
// "Block" here is a logical grouping construct, though it happens to map
// directly onto the physical block syntax of Terraform's native configuration
// syntax. It may be a more a matter of convention in other syntaxes, such as
// JSON.
//
// When converted to a value, a Block always becomes an instance of an object
// type derived from its defined attributes and nested blocks
type Block struct {
	// Attributes describes any attributes that may appear directly inside
	// the block.
	Attributes map[string]*Attribute

	// BlockTypes describes any nested block types that may appear directly
	// inside the block.
	BlockTypes map[string]*NestedBlock

	// Description and DescriptionKind contain a user facing description of the block
	// and the format of that string.
	Description     string
	DescriptionKind StringKind

	// Deprecated indicates whether the block has been marked as deprecated in the
	// provider and usage should be discouraged.
	Deprecated bool
}

// Attribute represents a configuration attribute, within a block.
type Attribute struct {
	// Type is a type specification that the attribute's value must conform to.
	Type cty.Type

	// Description is an English-language description of the purpose and
	// usage of the attribute. A description should be concise and use only
	// one or two sentences, leaving full definition to longer-form
	// documentation defined elsewhere.
	Description     string
	DescriptionKind StringKind

	// Required, if set to true, specifies that an omitted or null value is
	// not permitted.
	Required bool

	// Optional, if set to true, specifies that an omitted or null value is
	// permitted. This field conflicts with Required.
	Optional bool

	// Computed, if set to true, specifies that the value comes from the
	// provider rather than from configuration. If combined with Optional,
	// then the config may optionally provide an overridden value.
	Computed bool

	// Sensitive, if set to true, indicates that an attribute may contain
	// sensitive information.
	//
	// At present nothing is done with this information, but callers are
	// encouraged to set it where appropriate so that it may be used in the
	// future to help Terraform mask sensitive information. (Terraform
	// currently achieves this in a limited sense via other mechanisms.)
	Sensitive bool

	// Deprecated indicates whether the attribute has been marked as deprecated in the
	// provider and usage should be discouraged.
	Deprecated bool

	// WriteOnly indicates that the practitioner can choose a value for this
	// attribute, but Terraform will not store this attribute in plan or state.
	// WriteOnly can only be set for managed resource schemas. If WriteOnly is true,
	// either Optional or Required must also be true. WriteOnly cannot be set with ForceNew.
	//
	// WriteOnly cannot be set to true for TypeList, TypeMap, or TypeSet.
	//
	// This functionality is only supported in Terraform 1.11 and later.
	// Practitioners that choose a value for this attribute with older
	// versions of Terraform will receive an error.
	WriteOnly bool

	// RequiredForImport, if set to true, specifies that an omitted or null value is
	// not permitted when importing by the identity. This field conflicts with OptionalForImport.
	// Only valid for identity schemas.
	RequiredForImport bool

	// OptionalForImport, if set to true, specifies that an omitted or null value is
	// permitted when importing by the identity. This field conflicts with RequiredForImport.
	// Only valid for identity schemas.
	OptionalForImport bool
}

// NestedBlock represents the embedding of one block within another.
type NestedBlock struct {
	// Block is the description of the block that's nested.
	Block

	// Nesting provides the nesting mode for the child block, which determines
	// how many instances of the block are allowed, how many labels it expects,
	// and how the resulting data will be converted into a data structure.
	Nesting NestingMode

	// MinItems and MaxItems set, for the NestingList and NestingSet nesting
	// modes, lower and upper limits on the number of child blocks allowed
	// of the given type. If both are left at zero, no limit is applied.
	//
	// As a special case, both values can be set to 1 for NestingSingle in
	// order to indicate that a particular single block is required.
	//
	// These fields are ignored for other nesting modes and must both be left
	// at zero.
	MinItems, MaxItems int
}

// NestingMode is an enumeration of modes for nesting blocks inside other
// blocks.
type NestingMode int

// This code was previously generated with a go:generate directive calling:
// go run golang.org/x/tools/cmd/stringer -type=NestingMode
// However, it is now considered frozen and the tooling dependency has been
// removed. The String method can be manually updated if necessary.

const (
	nestingModeInvalid NestingMode = iota

	// NestingSingle indicates that only a single instance of a given
	// block type is permitted, with no labels, and its content should be
	// provided directly as an object value.
	NestingSingle

	// NestingGroup is similar to NestingSingle in that it calls for only a
	// single instance of a given block type with no labels, but it additionally
	// guarantees that its result will never be null, even if the block is
	// absent, and instead the nested attributes and blocks will be treated
	// as absent in that case. (Any required attributes or blocks within the
	// nested block are not enforced unless the block is explicitly present
	// in the configuration, so they are all effectively optional when the
	// block is not present.)
	//
	// This is useful for the situation where a remote API has a feature that
	// is always enabled but has a group of settings related to that feature
	// that themselves have default values. By using NestingGroup instead of
	// NestingSingle in that case, generated plans will show the block as
	// present even when not present in configuration, thus allowing any
	// default values within to be displayed to the user.
	NestingGroup

	// NestingList indicates that multiple blocks of the given type are
	// permitted, with no labels, and that their corresponding objects should
	// be provided in a list.
	NestingList

	// NestingSet indicates that multiple blocks of the given type are
	// permitted, with no labels, and that their corresponding objects should
	// be provided in a set.
	NestingSet

	// NestingMap indicates that multiple blocks of the given type are
	// permitted, each with a single label, and that their corresponding
	// objects should be provided in a map whose keys are the labels.
	//
	// It's an error, therefore, to use the same label value on multiple
	// blocks.
	NestingMap
)
