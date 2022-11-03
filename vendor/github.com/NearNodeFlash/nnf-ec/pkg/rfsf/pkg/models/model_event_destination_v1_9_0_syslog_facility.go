/*
 * Swordfish API
 *
 * This contains the definition of the Swordfish extensions to a Redfish service.
 *
 * API version: v1.2.c
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi
// EventDestinationV190SyslogFacility : The syslog facility code is an enumeration of program types.
type EventDestinationV190SyslogFacility string

// List of EventDestination_v1_9_0_SyslogFacility
const (
	KERN_EDV190SF EventDestinationV190SyslogFacility = "Kern"
	USER_EDV190SF EventDestinationV190SyslogFacility = "User"
	MAIL_EDV190SF EventDestinationV190SyslogFacility = "Mail"
	DAEMON_EDV190SF EventDestinationV190SyslogFacility = "Daemon"
	AUTH_EDV190SF EventDestinationV190SyslogFacility = "Auth"
	SYSLOG_EDV190SF EventDestinationV190SyslogFacility = "Syslog"
	LPR_EDV190SF EventDestinationV190SyslogFacility = "LPR"
	NEWS_EDV190SF EventDestinationV190SyslogFacility = "News"
	UUCP_EDV190SF EventDestinationV190SyslogFacility = "UUCP"
	CRON_EDV190SF EventDestinationV190SyslogFacility = "Cron"
	AUTHPRIV_EDV190SF EventDestinationV190SyslogFacility = "Authpriv"
	FTP_EDV190SF EventDestinationV190SyslogFacility = "FTP"
	NTP_EDV190SF EventDestinationV190SyslogFacility = "NTP"
	SECURITY_EDV190SF EventDestinationV190SyslogFacility = "Security"
	CONSOLE_EDV190SF EventDestinationV190SyslogFacility = "Console"
	SOLARIS_CRON_EDV190SF EventDestinationV190SyslogFacility = "SolarisCron"
	LOCAL0_EDV190SF EventDestinationV190SyslogFacility = "Local0"
	LOCAL1_EDV190SF EventDestinationV190SyslogFacility = "Local1"
	LOCAL2_EDV190SF EventDestinationV190SyslogFacility = "Local2"
	LOCAL3_EDV190SF EventDestinationV190SyslogFacility = "Local3"
	LOCAL4_EDV190SF EventDestinationV190SyslogFacility = "Local4"
	LOCAL5_EDV190SF EventDestinationV190SyslogFacility = "Local5"
	LOCAL6_EDV190SF EventDestinationV190SyslogFacility = "Local6"
	LOCAL7_EDV190SF EventDestinationV190SyslogFacility = "Local7"
)
