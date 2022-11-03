/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

type NetworkPortV130LinkStatus string

// List of NetworkPort_v1_3_0_LinkStatus
const (
	DOWN_NPV130LS NetworkPortV130LinkStatus = "Down"
	UP_NPV130LS NetworkPortV130LinkStatus = "Up"
	STARTING_NPV130LS NetworkPortV130LinkStatus = "Starting"
	TRAINING_NPV130LS NetworkPortV130LinkStatus = "Training"
)
