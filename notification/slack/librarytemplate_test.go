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

const (
	libTmplID   = "6f1c2c62-55d4-4f43-8a1e-3a4f7f7c9a01"
	assignedSvc = "0d3b0d4e-0000-4000-8000-000000000001"
	otherSvc    = "0d3b0d4e-0000-4000-8000-000000000002"
)

func libraryConfig(t *testing.T, title, text string) config.Config {
	t.Helper()
	var cfg config.Config
	cfg.Slack.SigningSecret = "secret"
	raw, err := slacktmpl.Library{{ID: libTmplID, Name: "Infra", Title: title, Text: text}}.Encode()
	require.NoError(t, err)
	cfg.Slack.AlertTemplateLibrary = raw
	cfg.Slack.ServiceAlertTemplates = []string{assignedSvc + "=" + libTmplID}
	return cfg
}

func amMeta(t *testing.T) map[string]string {
	t.Helper()
	data := slacktmpl.AlertmanagerData{
		Status:            "firing",
		CommonLabels:      slacktmpl.KV{"alertname": "X", "severity": "critical"},
		CommonAnnotations: slacktmpl.KV{"summary": "Exporter is not ready", "dashboard_url": "https://grafana.example.com/d/x"},
		Alerts:            slacktmpl.Alerts{{Status: "firing", Labels: slacktmpl.KV{"alertname": "X", "severity": "critical"}}},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return map[string]string{slacktmpl.MetadataKey: string(raw)}
}

func render(ctx context.Context, svc string, meta map[string]string, state notification.AlertState) (string, string, []alertTemplateButton, error) {
	return renderAlertTemplates(ctx, 7, "Exporter is not ready", "details", svc, "svc", "", meta, "Unacknowledged", state)
}

func TestLibraryTemplate_AssignedServiceOnly(t *testing.T) {
	meta := amMeta(t)

	// Baseline: no library at all (existing behaviour).
	var base config.Config
	baseTitle, baseText, baseButtons, err := render(base.Context(context.Background()), otherSvc, meta, notification.AlertStateUnacknowledged)
	require.NoError(t, err)

	cfg := libraryConfig(t, `{{ toUpper .CommonLabels.severity }} — {{ .CommonAnnotations.summary }}`, `*Service:* {{ .ServiceName }}`)
	ctx := cfg.Context(context.Background())

	title, text, buttons, err := render(ctx, assignedSvc, meta, notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "CRITICAL — Exporter is not ready", title)
	assert.Equal(t, "*Service:* svc", text)
	require.Len(t, buttons, 1, "assigned templates get dashboard/runbook buttons")
	assert.Equal(t, "https://grafana.example.com/d/x", buttons[0].URL)

	// Unassigned services keep exactly the existing output.
	title, text, buttons, err = render(ctx, otherSvc, meta, notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, baseTitle, title)
	assert.Equal(t, baseText, text)
	assert.Equal(t, baseButtons, buttons)

	// Service IDs are matched case-insensitively.
	title, _, _, err = render(ctx, "0D3B0D4E-0000-4000-8000-000000000001", meta, notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "CRITICAL — Exporter is not ready", title)
}

func TestLibraryTemplate_FallbackOnRuntimeError(t *testing.T) {
	// Parses fine, but fails at execution time (index out of range).
	cfg := libraryConfig(t, `{{ (index .Alerts 5).Labels.alertname }}`, `x`)
	ctx := cfg.Context(context.Background())

	title, text, buttons, err := render(ctx, assignedSvc, amMeta(t), notification.AlertStateUnacknowledged)
	require.NoError(t, err, "a broken assigned template must not drop the notification")
	assert.Equal(t, "Alert #7: Exporter is not ready", title, "falls back to the existing formatting")
	assert.Empty(t, text)
	assert.Empty(t, buttons)
}

func TestLibraryTemplate_FallbackOnMissingTemplate(t *testing.T) {
	var cfg config.Config
	cfg.Slack.ServiceAlertTemplates = []string{assignedSvc + "=" + libTmplID} // library empty
	ctx := cfg.Context(context.Background())

	title, _, _, err := render(ctx, assignedSvc, amMeta(t), notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "Alert #7: Exporter is not ready", title)
}

func TestLibraryTemplate_FallbackKeepsSharedTemplates(t *testing.T) {
	cfg := libraryConfig(t, `{{ (index .Alerts 5).Labels.alertname }}`, `x`)
	cfg.Slack.TitleTemplate = `shared: {{ .Summary }}`
	ctx := cfg.Context(context.Background())

	title, _, _, err := render(ctx, assignedSvc, amMeta(t), notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "shared: Exporter is not ready", title)
}

func TestSlackActionsEnabled(t *testing.T) {
	var cfg config.Config
	assert.False(t, slackActionsEnabled(cfg))
	cfg.Slack.ServiceAlertTemplates = []string{assignedSvc + "=" + libTmplID}
	assert.True(t, slackActionsEnabled(cfg))
	assert.False(t, richAlertsEnabled(cfg), "assignments must not turn on rich output for unassigned services")
}
