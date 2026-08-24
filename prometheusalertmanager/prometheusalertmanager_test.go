package prometheus

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/notification/slacktmpl"
)

func TestPostBodyMetadata(t *testing.T) {
	var body postBody
	err := json.Unmarshal([]byte(`{
		"receiver":"goalert",
		"status":"firing",
		"externalURL":"https://alertmanager.example.com",
		"alerts":[{
			"status":"firing",
			"labels":{"alertname":"QueueDown","severity":"critical","environment":"qa"},
			"annotations":{"summary":"Queue is down","description":"No workers are available"},
			"generatorURL":"https://prometheus.example.com/graph",
			"fingerprint":"abc123"
		}],
		"groupLabels":{"alertname":"QueueDown"},
		"commonLabels":{"alertname":"QueueDown","severity":"critical"},
		"commonAnnotations":{"summary":"Queue is down"},
		"version":"4",
		"groupKey":"group-key"
	}`), &body)
	require.NoError(t, err)
	assert.Equal(t, "Queue is down", body.Summary())

	meta, err := body.Metadata()
	require.NoError(t, err)
	assert.Equal(t, "critical", meta["severity"])
	assert.Equal(t, "No workers are available", meta["description"])
	data, ok, err := slacktmpl.DecodeAlertmanagerData(meta)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "critical", data.CommonLabels["severity"])
	require.Len(t, data.Alerts, 1)
	assert.Equal(t, "No workers are available", data.Alerts[0].Annotations["description"])
	assert.Equal(t, "abc123", data.Alerts[0].Fingerprint)
}

func TestPostBodySummaryWithoutAlerts(t *testing.T) {
	assert.Equal(t, "Prometheus Alertmanager alert", (postBody{}).Summary())
}
