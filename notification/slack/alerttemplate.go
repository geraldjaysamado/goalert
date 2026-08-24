package slack

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/target/goalert/config"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/slacktmpl"
	"github.com/target/goalert/util/log"
)

const (
	maxSlackTitleLength  = 2000
	maxSlackTextLength   = 3000
	maxSlackURLLength    = 3000
	maxButtonLabelLength = 75

	defaultRichTitleTemplate = `[{{ if eq .Status "firing" }}PROBLEM:{{ .Alerts.Firing | len }}{{ else }}RESOLVED{{ end }}] {{ or .CommonLabels.alertname .Summary }}`
	defaultRichTextTemplate  = "{{ with .Meta.severity }}*Severity:* `{{ . | title }}`\n{{ end }}" +
		"*Summary:* {{ .Summary }}\n" +
		"{{ with .Meta.description }}*Trigger description:*\n{{ . }}\n{{ end }}" +
		"{{ $labels := .Meta.Remove (stringSlice \"alertname\" \"severity\" \"summary\" \"description\" \"optdata\" \"dashboard_url\" \"dashboard_label\" \"runbook_url\" \"runbook_label\") }}" +
		"{{ if or .Meta.optdata $labels }}*Optdata:*\n{{ with .Meta.optdata }}{{ . }}\n{{ end }}{{ range $labels.SortedPairs }}*{{ .Name }}:* {{ .Value }}\n{{ end }}{{ end }}"
)

type alertTemplateButton struct {
	ActionID string
	Label    string
	URL      string
}

func goAlertStatus(state notification.AlertState) string {
	switch state {
	case notification.AlertStateUnacknowledged:
		return "unacknowledged"
	case notification.AlertStateAcknowledged:
		return "acknowledged"
	case notification.AlertStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

func alertmanagerStatus(state notification.AlertState) string {
	if state == notification.AlertStateClosed {
		return "resolved"
	}
	return "firing"
}

func fallbackAlertmanagerData(summary, details string, state notification.AlertState) slacktmpl.AlertmanagerData {
	status := alertmanagerStatus(state)
	labels := slacktmpl.KV{"alertname": summary}
	annotations := slacktmpl.KV{
		"summary":     summary,
		"description": details,
	}
	return slacktmpl.AlertmanagerData{
		Status:            status,
		CommonLabels:      labels,
		CommonAnnotations: annotations,
		Alerts: slacktmpl.Alerts{{
			Status:      status,
			Labels:      labels,
			Annotations: annotations,
		}},
	}
}

func templateData(ctx context.Context, alertID int, summary, details, serviceID, serviceName string, meta map[string]string, logEntry string, state notification.AlertState) slacktmpl.Data {
	data := fallbackAlertmanagerData(summary, details, state)
	stored, ok, err := slacktmpl.DecodeAlertmanagerData(meta)
	if err != nil {
		log.Log(ctx, err)
	} else if ok {
		data = stored
		data.Status = alertmanagerStatus(state)
		if state == notification.AlertStateClosed {
			for i := range data.Alerts {
				data.Alerts[i].Status = "resolved"
			}
		}
	}
	templateMeta := slacktmpl.MergeMetadata(data)
	for key, value := range meta {
		if key != slacktmpl.MetadataKey {
			templateMeta[key] = value
		}
	}

	cfg := config.FromContext(ctx)
	return slacktmpl.Data{
		AlertmanagerData: data,
		GoAlertStatus:    goAlertStatus(state),
		AlertID:          alertID,
		AlertURL:         cfg.CallbackURL(fmt.Sprintf("/alerts/%d", alertID)),
		Summary:          summary,
		Details:          details,
		ServiceID:        serviceID,
		ServiceName:      serviceName,
		Meta:             slacktmpl.KV(templateMeta),
		AlertMeta:        meta,
		LogEntry:         logEntry,
	}
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func templateButton(meta map[string]string, actionID, urlKey, labelKey, defaultLabel string) (alertTemplateButton, bool) {
	rawURL := strings.TrimSpace(meta[urlKey])
	if rawURL == "" || len([]rune(rawURL)) > maxSlackURLLength {
		return alertTemplateButton{}, false
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return alertTemplateButton{}, false
	}

	label := strings.TrimSpace(meta[labelKey])
	if label == "" {
		label = defaultLabel
	}
	return alertTemplateButton{
		ActionID: actionID,
		Label:    truncateRunes(label, maxButtonLabelLength),
		URL:      rawURL,
	}, true
}

func templateButtons(data slacktmpl.Data, enabled bool) []alertTemplateButton {
	if !enabled {
		return nil
	}
	buttons := make([]alertTemplateButton, 0, 2)
	if button, ok := templateButton(data.Meta, alertDashboardActionID, "dashboard_url", "dashboard_label", "Dashboard 📊"); ok {
		buttons = append(buttons, button)
	}
	if button, ok := templateButton(data.Meta, alertRunbookActionID, "runbook_url", "runbook_label", "Runbook 📖"); ok {
		buttons = append(buttons, button)
	}
	return buttons
}

func richAlertsEnabled(cfg config.Config) bool {
	return cfg.Slack.RichAlerts || cfg.Slack.TitleTemplate != "" || cfg.Slack.TextTemplate != ""
}

func renderAlertTemplates(ctx context.Context, alertID int, summary, details, serviceID, serviceName string, meta map[string]string, logEntry string, state notification.AlertState) (title, text string, buttons []alertTemplateButton, err error) {
	cfg := config.FromContext(ctx)
	data := templateData(ctx, alertID, summary, details, serviceID, serviceName, meta, logEntry, state)
	richEnabled := richAlertsEnabled(cfg)
	titleTemplate := cfg.Slack.TitleTemplate
	textTemplate := cfg.Slack.TextTemplate
	if cfg.Slack.RichAlerts {
		if titleTemplate == "" {
			titleTemplate = defaultRichTitleTemplate
		}
		if textTemplate == "" {
			textTemplate = defaultRichTextTemplate
		}
	}

	title = fmt.Sprintf("Alert #%d: %s", alertID, summary)
	if titleTemplate != "" {
		title, err = slacktmpl.Render("slack-title", titleTemplate, data)
		if err != nil {
			return "", "", nil, fmt.Errorf("render Slack title template: %w", err)
		}
		if strings.TrimSpace(title) == "" {
			title = fmt.Sprintf("Alert #%d: %s", alertID, summary)
		}
	}

	text, err = slacktmpl.Render("slack-text", textTemplate, data)
	if err != nil {
		return "", "", nil, fmt.Errorf("render Slack text template: %w", err)
	}
	buttons = templateButtons(data, richEnabled)
	return truncateRunes(title, maxSlackTitleLength), truncateRunes(text, maxSlackTextLength), buttons, nil
}
