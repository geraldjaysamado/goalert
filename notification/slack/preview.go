package slack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/target/goalert/alert"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/slacktmpl"
)

// previewAlertID is a placeholder alert number used in previews.
const previewAlertID = 1234

// PreviewButton is a link button shown under a rendered template.
type PreviewButton struct {
	Label string
	URL   string
}

// Preview is the rendered output of a template. Nothing is sent.
type Preview struct {
	Color   string
	Title   string
	Text    string
	Buttons []PreviewButton
	// Truncated is true when the body still exceeded Slack's limit after
	// listing fewer alerts, and whole lines were dropped from the end.
	Truncated bool
}

func previewLogEntry(state notification.AlertState) string {
	switch state {
	case notification.AlertStateAcknowledged:
		return "Acknowledged by Preview User"
	case notification.AlertStateClosed:
		return "Closed by Alertmanager"
	default:
		return "Unacknowledged"
	}
}

// PreviewAlertTemplate renders a title/body pair against sample data exactly
// as an assigned library template would be rendered, including link buttons.
// Unlike delivery, render errors are returned instead of falling back, so the
// editor can show them.
func PreviewAlertTemplate(ctx context.Context, titleTemplate, textTemplate string, sample slacktmpl.Sample, state notification.AlertState) (Preview, error) {
	meta := map[string]string{}
	for k, v := range sample.Meta {
		meta[k] = v
	}
	if sample.Alertmanager != nil {
		raw, err := json.Marshal(sample.Alertmanager)
		if err != nil {
			return Preview{}, fmt.Errorf("encode sample: %w", err)
		}
		meta[slacktmpl.MetadataKey] = string(raw)
	}

	source := sample.Source
	if source == "" {
		source = string(alert.SourceManual)
		if sample.Alertmanager != nil || len(sample.Meta) > 0 {
			source = string(alert.SourcePrometheusAlertmanager)
		}
	}

	data := templateData(ctx, previewAlertID, sample.Summary, sample.Details, "", sample.ServiceName, source, meta, previewLogEntry(state), state)
	title, text, buttons, hardCut, err := renderLibraryTemplate(previewAlertID, sample.Summary, titleTemplate, textTemplate, data)
	if err != nil {
		return Preview{}, err
	}

	res := Preview{Color: libraryAlertColor(data, state), Title: title, Text: text, Truncated: hardCut}
	for _, b := range buttons {
		res.Buttons = append(res.Buttons, PreviewButton{Label: b.Label, URL: b.URL})
	}
	return res, nil
}
