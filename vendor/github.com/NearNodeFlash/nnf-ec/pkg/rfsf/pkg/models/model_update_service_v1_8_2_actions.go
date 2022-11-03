/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

// UpdateServiceV182Actions - The available actions for this resource.
type UpdateServiceV182Actions struct {

	UpdateServiceSimpleUpdate UpdateServiceV182SimpleUpdate `json:"#UpdateService.SimpleUpdate,omitempty"`

	UpdateServiceStartUpdate UpdateServiceV182StartUpdate `json:"#UpdateService.StartUpdate,omitempty"`

	// The available OEM-specific actions for this resource.
	Oem map[string]interface{} `json:"Oem,omitempty"`
}
