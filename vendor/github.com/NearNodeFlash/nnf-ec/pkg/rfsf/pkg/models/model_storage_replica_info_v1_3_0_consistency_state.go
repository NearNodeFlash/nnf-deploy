/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi
// StorageReplicaInfoV130ConsistencyState : The values of ConsistencyState indicate the consistency type used by the source and its associated target group.
type StorageReplicaInfoV130ConsistencyState string

// List of StorageReplicaInfo_v1_3_0_ConsistencyState
const (
	CONSISTENT_SRIV130CST StorageReplicaInfoV130ConsistencyState = "Consistent"
	INCONSISTENT_SRIV130CST StorageReplicaInfoV130ConsistencyState = "Inconsistent"
)
