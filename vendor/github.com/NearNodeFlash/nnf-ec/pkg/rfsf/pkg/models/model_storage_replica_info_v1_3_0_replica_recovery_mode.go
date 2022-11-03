/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi
// StorageReplicaInfoV130ReplicaRecoveryMode : Values of ReplicaRecoveryMode describe whether the copy operation continues after a broken link is restored.
type StorageReplicaInfoV130ReplicaRecoveryMode string

// List of StorageReplicaInfo_v1_3_0_ReplicaRecoveryMode
const (
	AUTOMATIC_SRIV130RRM StorageReplicaInfoV130ReplicaRecoveryMode = "Automatic"
	MANUAL_SRIV130RRM StorageReplicaInfoV130ReplicaRecoveryMode = "Manual"
)
