package nfymsg

// Alert represents outgoing notifications for alerts.
type Alert struct {
	Base

	AlertID     int // The global alert number
	Summary     string
	Details     string
	ServiceID   string
	ServiceName string
	Meta        map[string]string

	// Source is the alert's stored source (e.g. "prometheusAlertmanager",
	// "generic", "manual"); empty if unknown.
	Source string

	// OriginalStatus is the status of the first Alert notification to this Dest for this AlertID.
	OriginalStatus *SendResult
}
