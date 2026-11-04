package slack

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/target/goalert/alert"
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

func templateData(ctx context.Context, alertID int, summary, details, serviceID, serviceName, source string, meta map[string]string, logEntry string, state notification.AlertState) slacktmpl.Data {
	data := fallbackAlertmanagerData(summary, details, state)
	dataState := "none"
	stored, ok, err := slacktmpl.DecodeAlertmanagerData(meta)
	if err != nil {
		log.Log(ctx, err)
		dataState = "unreadable"
	} else if ok {
		dataState = "complete"
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
	// Without a decodable payload, the alert's stored source decides whether
	// this is an Alertmanager alert (whose payload was not stored, e.g. it
	// exceeded the metadata size limit at ingestion, or the alert predates
	// payload storage). Common metadata of a mixed-alertname group has no
	// alertname, so labels alone cannot decide this. Only when the source is
	// unknown is an alertname label used as a hint. Nothing records *why* a
	// payload is missing, so templates must not claim a cause.
	if dataState == "none" {
		switch {
		case source == string(alert.SourcePrometheusAlertmanager):
			dataState = "metadata-only"
		case source == "" && strings.TrimSpace(meta["alertname"]) != "":
			dataState = "metadata-only"
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

		Source:                source,
		AlertmanagerDataState: dataState,
		MaxListedAlerts:       len(data.Alerts),
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

// slackActionsEnabled reports whether the Slack interaction endpoint should
// accept requests. Assigned library templates can add link buttons, so they
// enable it as well. This does not change how unassigned services render.
func slackActionsEnabled(cfg config.Config) bool {
	return cfg.Slack.InteractiveMessages || richAlertsEnabled(cfg) || len(cfg.Slack.ServiceAlertTemplates) > 0
}

// assignedTemplate returns the library template assigned to serviceID. Any
// problem (bad stored data, template missing) is logged and reported as "no
// assignment" so the caller uses the existing formatting instead of dropping
// the notification.
func assignedTemplate(ctx context.Context, cfg config.Config, serviceID string) (slacktmpl.LibraryTemplate, bool) {
	if serviceID == "" || len(cfg.Slack.ServiceAlertTemplates) == 0 {
		return slacktmpl.LibraryTemplate{}, false
	}
	ctx = log.WithField(ctx, "ServiceID", serviceID)
	assignments, err := slacktmpl.ParseAssignments(cfg.Slack.ServiceAlertTemplates)
	if err != nil {
		log.Log(ctx, fmt.Errorf("slack template assignments unreadable, using default formatting: %w", err))
		return slacktmpl.LibraryTemplate{}, false
	}
	templateID, ok := assignments.Lookup(serviceID)
	if !ok {
		return slacktmpl.LibraryTemplate{}, false
	}
	lib, err := slacktmpl.ParseLibrary(cfg.Slack.AlertTemplateLibrary)
	if err != nil {
		log.Log(ctx, fmt.Errorf("slack template library unreadable, using default formatting: %w", err))
		return slacktmpl.LibraryTemplate{}, false
	}
	tmpl, ok := lib.Find(templateID)
	if !ok {
		log.Log(ctx, fmt.Errorf("assigned slack template %s not found, using default formatting", templateID))
		return slacktmpl.LibraryTemplate{}, false
	}
	return tmpl, true
}

func renderAlertTemplates(ctx context.Context, alertID int, summary, details, serviceID, serviceName, source string, meta map[string]string, logEntry string, state notification.AlertState) (title, text string, buttons []alertTemplateButton, err error) {
	title, text, buttons, _, err = renderAlertMessage(ctx, alertID, summary, details, serviceID, serviceName, source, meta, logEntry, state)
	return
}

func renderAlertMessage(ctx context.Context, alertID int, summary, details, serviceID, serviceName, source string, meta map[string]string, logEntry string, state notification.AlertState) (title, text string, buttons []alertTemplateButton, color string, err error) {
	cfg := config.FromContext(ctx)
	data := templateData(ctx, alertID, summary, details, serviceID, serviceName, source, meta, logEntry, state)

	if tmpl, ok := assignedTemplate(ctx, cfg, serviceID); ok {
		title, text, buttons, _, err = renderLibraryTemplate(alertID, summary, tmpl.Title, tmpl.Text, data)
		if err == nil {
			return title, text, buttons, libraryAlertColor(data, state), nil
		}
		log.Log(log.WithFields(ctx, log.Fields{
			"ServiceID":         serviceID,
			"SlackTemplateID":   tmpl.ID,
			"SlackTemplateName": tmpl.Name,
		}), fmt.Errorf("assigned slack template failed, using default formatting: %w", err))
	}

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
	title, text, buttons, err = renderTemplatePair(alertID, summary, titleTemplate, textTemplate, data, richEnabled)
	return title, text, buttons, "", err
}

// renderTemplatePair renders a title/body pair. Behaviour for the shared
// templates is identical to the original single-template implementation.
func renderTemplatePair(alertID int, summary, titleTemplate, textTemplate string, data slacktmpl.Data, buttonsEnabled bool) (title, text string, buttons []alertTemplateButton, err error) {
	title, text, buttons, err = renderTemplatePairRaw(alertID, summary, titleTemplate, textTemplate, data, buttonsEnabled)
	if err != nil {
		return "", "", nil, err
	}
	return title, truncateRunes(text, maxSlackTextLength), buttons, nil
}

// renderTemplatePairRaw renders without shortening the body.
func renderTemplatePairRaw(alertID int, summary, titleTemplate, textTemplate string, data slacktmpl.Data, buttonsEnabled bool) (title, text string, buttons []alertTemplateButton, err error) {
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
	buttons = templateButtons(data, buttonsEnabled)
	return truncateRunes(title, maxSlackTitleLength), text, buttons, nil
}

// shortenedNotice is appended when a library template's body still exceeds
// Slack's limit after listing as few alerts as the template allows.
const shortenedNotice = "\n_…message shortened to fit Slack's limit._"

// renderLibraryTemplate renders an assigned library template so that the body
// fits Slack's limit without cutting through lines or links:
//
//  1. Render with every alert listable (.MaxListedAlerts = len(.Alerts)).
//  2. If too long, find the largest .MaxListedAlerts that fits, so the
//     template itself drops whole entries and prints an accurate "N more"
//     notice and links.
//  3. Only if it still does not fit (e.g. one very long line), keep whole
//     lines up to the limit and add a shortened notice (hardCut=true).
//
// The preview uses this same function, so it shows exactly what is sent.
func renderLibraryTemplate(alertID int, summary, titleTemplate, textTemplate string, data slacktmpl.Data) (title, text string, buttons []alertTemplateButton, hardCut bool, err error) {
	render := func(n int) (string, string, []alertTemplateButton, error) {
		d := data
		d.MaxListedAlerts = n
		t, x, b, err := renderTemplatePairRaw(alertID, summary, titleTemplate, textTemplate, d, true)
		if strings.TrimSpace(d.Meta["dashboard_label"]) == "" {
			for i := range b {
				if b[i].ActionID == alertDashboardActionID {
					b[i].Label = "Grafana Dashboard"
				}
			}
		}
		return t, x, b, err
	}
	fits := func(s string) bool { return utf8.RuneCountInString(s) <= maxSlackTextLength }

	total := len(data.Alerts)
	title, text, buttons, err = render(total)
	if err != nil || fits(text) {
		return title, text, buttons, false, err
	}

	// Largest n in [0, total) whose output fits (output shrinks as n shrinks).
	lo, hi, best := 0, total-1, -1
	var bestTitle, bestText string
	var bestButtons []alertTemplateButton
	for lo <= hi {
		mid := (lo + hi) / 2
		t, x, b, rErr := render(mid)
		if rErr != nil {
			return "", "", nil, false, rErr
		}
		if fits(x) {
			best, bestTitle, bestText, bestButtons = mid, t, x, b
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best >= 0 {
		return bestTitle, bestText, bestButtons, false, nil
	}

	title, text, buttons, err = render(0)
	if err != nil {
		return "", "", nil, false, err
	}
	return title, cutAtLine(text, maxSlackTextLength-utf8.RuneCountInString(shortenedNotice)) + shortenedNotice, buttons, true, nil
}

// cutAtLine returns at most max runes of text, ending at a line break when
// possible and never inside a Slack <...> link.
func cutAtLine(text string, max int) string {
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	cut := string(r[:max])
	if i := strings.LastIndex(cut, "\n"); i > 0 {
		return strings.TrimRight(cut[:i], "\n")
	}
	if open, closed := strings.LastIndex(cut, "<"), strings.LastIndex(cut, ">"); open > closed {
		cut = cut[:open]
	}
	return strings.TrimSpace(cut)
}

// alertColor preserves lifecycle colours; informational firing alerts may override red.
func alertColor(state notification.AlertState, override string) string {
	switch state {
	case notification.AlertStateClosed:
		return colorClosed
	case notification.AlertStateAcknowledged:
		return colorAcked
	case notification.AlertStateUnacknowledged:
		if override == colorInfo {
			return colorInfo
		}
		return colorUnacked
	default:
		return ""
	}
}

// Only successfully rendered library templates use severity colours. Shared
// formatting and error fallbacks retain their existing lifecycle colours.
func libraryAlertColor(data slacktmpl.Data, state notification.AlertState) string {
	isInfo := func(s string) bool { return strings.EqualFold(strings.TrimSpace(s), "info") }
	if data.AlertmanagerDataState == "complete" {
		if len(data.Alerts) == 0 {
			return alertColor(state, "")
		}
		for _, a := range data.Alerts {
			if !isInfo(a.Labels["severity"]) {
				return alertColor(state, "")
			}
		}
		// Unknown omitted alerts cannot be assumed informational.
		if data.TruncatedAlerts > 0 && !isInfo(data.CommonLabels["severity"]) {
			return alertColor(state, "")
		}
	} else if !isInfo(data.AlertMeta["severity"]) {
		return alertColor(state, "")
	}
	return alertColor(state, colorInfo)
}
