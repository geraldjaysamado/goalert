package slacktmpl

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	data := Data{AlertmanagerData: AlertmanagerData{
		Status:       "firing",
		CommonLabels: map[string]string{"alertname": "QueueDown", "severity": "critical"},
		Alerts: Alerts{
			{Status: "firing", Annotations: map[string]string{"summary": "queue is down"}},
			{Status: "resolved", Annotations: map[string]string{"summary": "old alert"}},
		},
	}}

	title, err := Render("title", `[{{ if eq .Status "firing" }}PROBLEM:{{ .Alerts.Firing | len }}{{ else }}RESOLVED{{ end }}] {{ .CommonLabels.alertname }}`, data)
	require.NoError(t, err)
	assert.Equal(t, "[PROBLEM:1] QueueDown", title)

	text, err := Render("text", `*Severity:* {{ .CommonLabels.severity | title }}
*Summary:* {{ (index .Alerts 0).Annotations.summary }}`, data)
	require.NoError(t, err)
	assert.Equal(t, "*Severity:* Critical\n*Summary:* queue is down", text)
}

func TestRenderAlertmanagerCompatibleTemplate(t *testing.T) {
	startsAt := time.Date(2026, time.August, 24, 1, 2, 3, 0, time.UTC)
	data := Data{AlertmanagerData: AlertmanagerData{
		Status:       "firing",
		GroupLabels:  KV{"alertname": "QueueDown", "environment": "qa"},
		CommonLabels: KV{"alertname": "QueueDown", "environment": "qa", "severity": "critical"},
		Alerts: Alerts{{
			Status:   "firing",
			StartsAt: startsAt,
		}},
	}}

	value, err := Render("alertmanager", `[{{ .Status | toUpper }}:{{ .Alerts.Firing | len }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ with .CommonLabels.Remove .GroupLabels.Names }}({{ .Values | join " " }}){{ end }} started {{ (index .Alerts 0).StartsAt.Format "2006-01-02" }}`, data)
	require.NoError(t, err)
	assert.Equal(t, "[FIRING:1] QueueDown qa (critical) started 2026-08-24", value)
}

func TestRenderAlertmanagerCompatibleFunctions(t *testing.T) {
	data := Data{
		AlertmanagerData: AlertmanagerData{RouteLabels: KV{"team": "platform"}},
		Summary:          "QueueDown",
		Meta:             map[string]string{"severity": "critical"},
	}
	value, err := Render("functions", `{{ if match "^crit" .Meta.severity }}{{ reReplaceAll "Down$" "Unavailable" .Summary }} {{ stringSlice "a" "b" | join "," }} {{ dict "severity" .Meta.severity | toJson }} {{ "value" | base64encode | base64decode }} {{ toDate "2006-01-02" "2026-08-24" | date "2006" }} {{ routeLabels "team" }}{{ end }}`, data)
	require.NoError(t, err)
	assert.Equal(t, `QueueUnavailable a,b {"severity":"critical"} value 2026 platform`, value)
}

func TestValidate(t *testing.T) {
	assert.NoError(t, Validate("empty", ""))
	assert.NoError(t, Validate("valid", `{{ .CommonLabels.alertname }}`))
	assert.Error(t, Validate("invalid", `{{`))
}

func TestDecodeAlertmanagerData(t *testing.T) {
	expected := AlertmanagerData{
		Status:       "firing",
		CommonLabels: map[string]string{"alertname": "QueueDown"},
		Alerts:       Alerts{{Status: "firing", Labels: map[string]string{"severity": "critical"}}},
	}
	raw, err := json.Marshal(expected)
	require.NoError(t, err)

	actual, ok, err := DecodeAlertmanagerData(map[string]string{MetadataKey: string(raw)})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, expected, actual)

	_, ok, err = DecodeAlertmanagerData(map[string]string{"other": "value"})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMergeMetadata(t *testing.T) {
	data := AlertmanagerData{
		CommonLabels:      map[string]string{"severity": "critical", "shared": "label"},
		CommonAnnotations: map[string]string{"description": "common", "shared": "annotation"},
		Alerts: Alerts{{
			Labels:      map[string]string{"instance": "one"},
			Annotations: map[string]string{"description": "single"},
		}},
	}
	assert.Equal(t, map[string]string{
		"severity":    "critical",
		"shared":      "annotation",
		"instance":    "one",
		"description": "common",
	}, MergeMetadata(data))

	data.Alerts = append(data.Alerts, AlertmanagerAlert{Labels: map[string]string{"instance": "two"}})
	assert.Equal(t, map[string]string{
		"severity":    "critical",
		"shared":      "annotation",
		"description": "common",
	}, MergeMetadata(data))
}
