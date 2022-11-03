/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

import (
	"time"
)

// MetricReportV140MetricReport - The metric definitions that create a metric report.
type MetricReportV140MetricReport struct {

	// The OData description of a payload.
	OdataContext string `json:"@odata.context,omitempty"`

	// The current ETag of the resource.
	OdataEtag string `json:"@odata.etag,omitempty"`

	// The unique identifier for a resource.
	OdataId string `json:"@odata.id"`

	// The type of a resource.
	OdataType string `json:"@odata.type"`

	Actions MetricReportV140Actions `json:"Actions,omitempty"`

	// A context can be supplied at subscription time.  This property is the context value supplied by the subscriber.
	Context string `json:"Context,omitempty"`

	// The description of this resource.  Used for commonality in the schema definitions.
	Description string `json:"Description,omitempty"`

	// The identifier that uniquely identifies the resource within the collection of similar resources.
	Id string `json:"Id"`

	MetricReportDefinition OdataV4IdRef `json:"MetricReportDefinition,omitempty"`

	// An array of metric values for the metered items of this metric report.
	MetricValues []MetricReportV140MetricValue `json:"MetricValues,omitempty"`

	// The name of the resource or array member.
	Name string `json:"Name"`

	// The OEM extension.
	Oem map[string]interface{} `json:"Oem,omitempty"`

	// The current sequence identifier for this metric report.
	ReportSequence string `json:"ReportSequence,omitempty"`

	// The time associated with the metric report in its entirety.  The time of the metric report can be relevant when the time of individual metrics are minimally different.
	Timestamp *time.Time `json:"Timestamp,omitempty"`
}
