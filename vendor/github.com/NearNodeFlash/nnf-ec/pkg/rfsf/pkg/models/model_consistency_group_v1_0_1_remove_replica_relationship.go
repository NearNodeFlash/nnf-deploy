/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

// ConsistencyGroupV101RemoveReplicaRelationship - This action is used to disable data synchronization between a source and target consistency group, remove the replication relationship, and optionally delete the target consistency group.
type ConsistencyGroupV101RemoveReplicaRelationship struct {

	// Link to invoke action
	Target string `json:"target,omitempty"`

	// Friendly action name
	Title string `json:"title,omitempty"`
}
