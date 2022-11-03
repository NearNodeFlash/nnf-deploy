/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi

// ProcessorMetricsV111ProcessorMetrics - The ProcessorMetrics schema contains usage and health statistics for a processor.
type ProcessorMetricsV111ProcessorMetrics struct {

	// The OData description of a payload.
	OdataContext string `json:"@odata.context,omitempty"`

	// The current ETag of the resource.
	OdataEtag string `json:"@odata.etag,omitempty"`

	// The unique identifier for a resource.
	OdataId string `json:"@odata.id"`

	// The type of a resource.
	OdataType string `json:"@odata.type"`

	Actions ProcessorMetricsV111Actions `json:"Actions,omitempty"`

	// The average frequency of the processor.
	AverageFrequencyMHz *float32 `json:"AverageFrequencyMHz,omitempty"`

	// The CPU bandwidth as a percentage.
	BandwidthPercent *float32 `json:"BandwidthPercent,omitempty"`

	// The processor cache metrics.
	Cache []ProcessorMetricsV111CacheMetrics `json:"Cache,omitempty"`

	// The power, in watts, that the processor has consumed.
	ConsumedPowerWatt *float32 `json:"ConsumedPowerWatt,omitempty"`

	// The processor core metrics.
	CoreMetrics []ProcessorMetricsV111CoreMetrics `json:"CoreMetrics,omitempty"`

	// The description of this resource.  Used for commonality in the schema definitions.
	Description string `json:"Description,omitempty"`

	// The frequency relative to the nominal processor frequency ratio.
	FrequencyRatio *float32 `json:"FrequencyRatio,omitempty"`

	// The identifier that uniquely identifies the resource within the collection of similar resources.
	Id string `json:"Id"`

	// The percentage of time spent in kernel mode.
	KernelPercent *float32 `json:"KernelPercent,omitempty"`

	// The local memory bandwidth usage in bytes.
	LocalMemoryBandwidthBytes int64 `json:"LocalMemoryBandwidthBytes,omitempty"`

	// The name of the resource or array member.
	Name string `json:"Name"`

	// The OEM extension.
	Oem map[string]interface{} `json:"Oem,omitempty"`

	// Operating speed of the processor in MHz.
	OperatingSpeedMHz int64 `json:"OperatingSpeedMHz,omitempty"`

	// The remote memory bandwidth usage in bytes.
	RemoteMemoryBandwidthBytes int64 `json:"RemoteMemoryBandwidthBytes,omitempty"`

	// The temperature of the processor.
	TemperatureCelsius *float32 `json:"TemperatureCelsius,omitempty"`

	// The CPU margin to throttle (temperature offset in degree Celsius).
	ThrottlingCelsius *float32 `json:"ThrottlingCelsius,omitempty"`

	// The percentage of time spent in user mode.
	UserPercent *float32 `json:"UserPercent,omitempty"`
}
