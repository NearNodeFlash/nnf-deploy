/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type ConsistencyGroupApplicationConsistencyMethod string

// List of ConsistencyGroup_ApplicationConsistencyMethod
const (
	HOT_STANDBY_CGACM ConsistencyGroupApplicationConsistencyMethod = "HotStandby"
	VASA_CGACM ConsistencyGroupApplicationConsistencyMethod = "VASA"
	VDI_CGACM ConsistencyGroupApplicationConsistencyMethod = "VDI"
	VSS_CGACM ConsistencyGroupApplicationConsistencyMethod = "VSS"
	OTHER_CGACM ConsistencyGroupApplicationConsistencyMethod = "Other"
)
