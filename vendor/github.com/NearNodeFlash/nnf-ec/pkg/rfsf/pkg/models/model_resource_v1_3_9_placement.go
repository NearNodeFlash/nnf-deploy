/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

// ResourceV139Placement - The placement within the addressed location.
type ResourceV139Placement struct {

	// The name of a rack location within a row.
	Rack string `json:"Rack,omitempty"`

	// The vertical location of the item, in terms of RackOffsetUnits.
	RackOffset int64 `json:"RackOffset,omitempty"`

	RackOffsetUnits ResourceV139RackUnits `json:"RackOffsetUnits,omitempty"`

	// The name of the row.
	Row string `json:"Row,omitempty"`
}
