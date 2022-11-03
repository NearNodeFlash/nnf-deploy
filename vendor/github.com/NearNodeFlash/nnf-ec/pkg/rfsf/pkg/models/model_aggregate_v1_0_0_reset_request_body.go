/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

// AggregateV100ResetRequestBody - This action is used to reset a collection of resources.  For example, this could be an aggregate or a list of computer systems.
type AggregateV100ResetRequestBody struct {

	// The number of elements in each batch being reset.
	BatchSize int64 `json:"BatchSize,omitempty"`

	// The delay of the batches of elements being reset in seconds.
	DelayBetweenBatchesInSeconds int64 `json:"DelayBetweenBatchesInSeconds,omitempty"`

	ResetType ResourceResetType `json:"ResetType,omitempty"`
}
