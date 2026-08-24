package slack

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/config"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/slacktmpl"
)

func TestRenderAlertTemplates_Default(t *testing.T) {
	var cfg config.Config
	ctx := cfg.Context(context.Background())

	title, text, buttons, err := renderAlertTemplates(ctx, 42, "queue down", "details", "svc-id", "service", nil, "Unacknowledged", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "Alert #42: queue down", title)
	assert.Empty(t, text)
	assert.Empty(t, buttons)
}

func TestRenderAlertTemplates_AlertmanagerData(t *testing.T) {
	alertmanagerData := slacktmpl.AlertmanagerData{
		Status:       "firing",
		CommonLabels: map[string]string{"alertname": "QueueDown", "severity": "critical"},
		CommonAnnotations: map[string]string{
			"description":     "No workers are available",
			"dashboard_url":   "https://example.com/dashboard",
			"dashboard_label": "Queue Dashboard 📊",
			"runbook_url":     "https://example.com/runbook",
		},
		Alerts: slacktmpl.Alerts{{
			Status:      "firing",
			Labels:      map[string]string{"alertname": "QueueDown"},
			Annotations: map[string]string{"summary": "queue is down"},
		}},
	}
	raw, err := json.Marshal(alertmanagerData)
	require.NoError(t, err)
	meta := map[string]string{slacktmpl.MetadataKey: string(raw)}

	var cfg config.Config
	cfg.Slack.TitleTemplate = `[{{ if eq .Status "firing" }}PROBLEM:{{ .Alerts.Firing | len }}{{ else }}RESOLVED{{ end }}] {{ .CommonLabels.alertname }}`
	cfg.Slack.TextTemplate = `*Summary:* {{ (index .Alerts 0).Annotations.summary }}
*Severity:* {{ .Meta.severity }}
*Description:* {{ .Meta.description }}
*GoAlert:* {{ .GoAlertStatus }}`
	ctx := cfg.Context(context.Background())

	title, text, buttons, err := renderAlertTemplates(ctx, 42, "queue down", "details", "svc-id", "service", meta, "Unacknowledged", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "[PROBLEM:1] QueueDown", title)
	assert.Equal(t, "*Summary:* queue is down\n*Severity:* critical\n*Description:* No workers are available\n*GoAlert:* unacknowledged", text)
	assert.Equal(t, []alertTemplateButton{
		{ActionID: alertDashboardActionID, Label: "Queue Dashboard 📊", URL: "https://example.com/dashboard"},
		{ActionID: alertRunbookActionID, Label: "Runbook 📖", URL: "https://example.com/runbook"},
	}, buttons)

	title, text, buttons, err = renderAlertTemplates(ctx, 42, "queue down", "details", "svc-id", "service", meta, "Closed", notification.AlertStateClosed)
	require.NoError(t, err)
	assert.Equal(t, "[RESOLVED] QueueDown", title)
	assert.Equal(t, "*Summary:* queue is down\n*Severity:* critical\n*Description:* No workers are available\n*GoAlert:* closed", text)
	require.Len(t, buttons, 2)
}

func TestRenderAlertTemplates_RichDefault(t *testing.T) {
	alertmanagerData := slacktmpl.AlertmanagerData{
		Status:       "firing",
		CommonLabels: slacktmpl.KV{"alertname": "Celery_Queue_Backlog_Critical_PROD", "severity": "critical", "environment": "production", "app": "playbookregistration", "queue": "l_priority"},
		CommonAnnotations: slacktmpl.KV{
			"summary":         "Celery queue backlog (critical, prod)",
			"description":     "Queue backlog in prod is greater than 1000 tasks for 10 minutes.",
			"optdata":         "backlog=12149 tasks (threshold: 1000)",
			"dashboard_url":   "https://example.com/dashboard",
			"dashboard_label": "Celery Dashboard 📊",
			"runbook_url":     "https://example.com/runbook",
		},
		Alerts: slacktmpl.Alerts{{Status: "firing"}},
	}
	raw, err := json.Marshal(alertmanagerData)
	require.NoError(t, err)

	var cfg config.Config
	cfg.Slack.RichAlerts = true
	ctx := cfg.Context(context.Background())
	title, text, buttons, err := renderAlertTemplates(ctx, 99, "Celery queue backlog (critical, prod)", "details", "svc-id", "service", map[string]string{slacktmpl.MetadataKey: string(raw)}, "Unacknowledged", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "[PROBLEM:1] Celery_Queue_Backlog_Critical_PROD", title)
	assert.Contains(t, text, "*Severity:* `Critical`")
	assert.Contains(t, text, "*Summary:* Celery queue backlog (critical, prod)")
	assert.Contains(t, text, "*Trigger description:*\nQueue backlog in prod is greater than 1000 tasks for 10 minutes.")
	assert.Contains(t, text, "*Optdata:*\nbacklog=12149 tasks (threshold: 1000)")
	assert.Contains(t, text, "*app:* playbookregistration")
	assert.Contains(t, text, "*environment:* production")
	assert.Contains(t, text, "*queue:* l_priority")
	require.Len(t, buttons, 2)

	title, text, buttons, err = renderAlertTemplates(ctx, 99, "Celery queue backlog (critical, prod)", "details", "svc-id", "service", map[string]string{slacktmpl.MetadataKey: string(raw)}, "Closed", notification.AlertStateClosed)
	require.NoError(t, err)
	assert.Equal(t, "[RESOLVED] Celery_Queue_Backlog_Critical_PROD", title)
	assert.Contains(t, text, "*app:* playbookregistration")
	require.Len(t, buttons, 2)
}

func TestRenderAlertTemplates_FlexibleLabels(t *testing.T) {
	alertmanagerData := slacktmpl.AlertmanagerData{
		Status: "firing",
		CommonLabels: slacktmpl.KV{
			"alertname":   "QueueDown",
			"app":         "worker",
			"environment": "production",
			"severity":    "critical",
		},
		CommonAnnotations: slacktmpl.KV{
			"description": "No workers are available",
			"owner":       "platform",
		},
		Alerts: slacktmpl.Alerts{{Status: "firing"}},
	}
	raw, err := json.Marshal(alertmanagerData)
	require.NoError(t, err)
	meta := map[string]string{slacktmpl.MetadataKey: string(raw)}

	t.Run("selected values", func(t *testing.T) {
		var cfg config.Config
		cfg.Slack.TextTemplate = `environment={{ .Meta.environment }} owner={{ .Meta.owner }}`

		_, text, _, err := renderAlertTemplates(cfg.Context(context.Background()), 1, "queue down", "details", "svc-id", "service", meta, "Unacknowledged", notification.AlertStateUnacknowledged)
		require.NoError(t, err)
		assert.Equal(t, "environment=production owner=platform", text)
	})

	t.Run("all values", func(t *testing.T) {
		var cfg config.Config
		cfg.Slack.TextTemplate = `{{ range .Meta.SortedPairs }}{{ .Name }}={{ .Value }}
{{ end }}`

		_, text, _, err := renderAlertTemplates(cfg.Context(context.Background()), 1, "queue down", "details", "svc-id", "service", meta, "Unacknowledged", notification.AlertStateUnacknowledged)
		require.NoError(t, err)
		assert.Equal(t, "alertname=QueueDown\napp=worker\ndescription=No workers are available\nenvironment=production\nowner=platform\nseverity=critical", text)
	})
}

func TestRenderAlertTemplates_NonAlertmanagerFallback(t *testing.T) {
	var cfg config.Config
	cfg.Slack.TitleTemplate = `{{ .CommonLabels.alertname }}`
	cfg.Slack.TextTemplate = `{{ .Meta.description }}`
	ctx := cfg.Context(context.Background())

	title, text, buttons, err := renderAlertTemplates(ctx, 7, "manual alert", "manual details", "svc-id", "service", nil, "Acknowledged", notification.AlertStateAcknowledged)
	require.NoError(t, err)
	assert.Equal(t, "manual alert", title)
	assert.Equal(t, "manual details", text)
	assert.Empty(t, buttons)
}

func TestTemplateButtons_InvalidURLs(t *testing.T) {
	data := slacktmpl.Data{Meta: map[string]string{
		"dashboard_url": "javascript:alert(1)",
		"runbook_url":   "/relative/runbook",
	}}
	assert.Empty(t, templateButtons(data, true))
}
