/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

// StorageV172CacheSummary - This type describes the cache memory of the storage controller in general detail.
type StorageV172CacheSummary struct {

	// The portion of the cache memory that is persistent, measured in MiB.
	PersistentCacheSizeMiB int64 `json:"PersistentCacheSizeMiB,omitempty"`

	Status ResourceStatus `json:"Status,omitempty"`

	// The total configured cache memory, measured in MiB.
	TotalCacheSizeMiB int64 `json:"TotalCacheSizeMiB"`
}
