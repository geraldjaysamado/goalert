package graphqlapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/target/goalert/config"
	"github.com/target/goalert/graphql2"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/slack"
	"github.com/target/goalert/notification/slacktmpl"
	"github.com/target/goalert/permission"
	"github.com/target/goalert/service"
	"github.com/target/goalert/validation"
	"github.com/target/goalert/validation/validate"
)

// Slack template library resolvers.
//
// Templates and per-service assignments are stored in the GoAlert config
// (Slack.AlertTemplateLibrary / Slack.ServiceAlertTemplates). Every change goes
// through ConfigStore.UpdateConfig, which requires admin, locks the config
// table, re-reads the latest config inside the transaction, and runs full
// config validation. That makes each change atomic: concurrent edits to other
// templates or other services' assignments are not overwritten.

func requireSlackTemplateAdmin(ctx context.Context) error {
	return permission.LimitCheckAny(ctx, permission.System, permission.Admin)
}

func slackTemplateFieldErr(err error) error {
	var tErr *slacktmpl.TemplateError
	if errors.As(err, &tErr) {
		return validation.NewFieldError(tErr.Field, tErr.Msg)
	}
	return err
}

func (a *App) servicesByID(ctx context.Context, ids []string) (map[string]service.Service, error) {
	res := make(map[string]service.Service, len(ids))
	for len(ids) > 0 {
		n := min(len(ids), 100)
		svcs, err := a.ServiceStore.FindMany(ctx, ids[:n])
		if err != nil {
			return nil, err
		}
		for _, s := range svcs {
			res[strings.ToLower(s.ID)] = s
		}
		ids = ids[n:]
	}
	return res, nil
}

func (q *Query) SlackAlertTemplates(ctx context.Context) ([]graphql2.SlackAlertTemplate, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return nil, err
	}
	cfg := q.ConfigStore.Config()
	lib, err := slacktmpl.ParseLibrary(cfg.Slack.AlertTemplateLibrary)
	if err != nil {
		return nil, err
	}
	assign, err := slacktmpl.ParseAssignments(cfg.Slack.ServiceAlertTemplates)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(assign))
	for svcID := range assign {
		ids = append(ids, svcID)
	}
	sort.Strings(ids)
	svcs, err := (*App)(q).servicesByID(ctx, ids)
	if err != nil {
		return nil, err
	}

	res := make([]graphql2.SlackAlertTemplate, 0, len(lib))
	for _, t := range lib {
		item := graphql2.SlackAlertTemplate{ID: t.ID, Name: t.Name, Title: t.Title, Text: t.Text, Revision: t.Revision(), Services: []service.Service{}}
		for _, svcID := range assign.ServicesUsing(t.ID) {
			if s, ok := svcs[svcID]; ok {
				item.Services = append(item.Services, s)
			}
		}
		sort.Slice(item.Services, func(i, j int) bool {
			return strings.ToLower(item.Services[i].Name) < strings.ToLower(item.Services[j].Name)
		})
		res = append(res, item)
	}
	sort.SliceStable(res, func(i, j int) bool { return strings.ToLower(res[i].Name) < strings.ToLower(res[j].Name) })
	return res, nil
}

func (q *Query) SlackAlertTemplateExamples(ctx context.Context) ([]graphql2.SlackAlertTemplateExample, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return nil, err
	}
	var res []graphql2.SlackAlertTemplateExample
	for _, e := range slacktmpl.Examples() {
		res = append(res, graphql2.SlackAlertTemplateExample{Key: e.Key, Name: e.Name, Description: e.Description, Title: e.Title, Text: e.Text})
	}
	return res, nil
}

func (q *Query) SlackAlertTemplatePreviewSamples(ctx context.Context) ([]graphql2.SlackAlertTemplatePreviewSample, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return nil, err
	}
	var res []graphql2.SlackAlertTemplatePreviewSample
	for _, s := range slacktmpl.Samples() {
		res = append(res, graphql2.SlackAlertTemplatePreviewSample{ID: s.ID, Name: s.Name})
	}
	return res, nil
}

const maxPreviewPayloadSize = 256 * 1024

func (q *Query) PreviewSlackAlertTemplate(ctx context.Context, input graphql2.PreviewSlackAlertTemplateInput) (*graphql2.SlackAlertTemplatePreview, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return nil, err
	}
	// Size check only: templates may legitimately start/end with whitespace,
	// which validate.Text would reject.
	if len(input.Title) > slacktmpl.MaxLibraryTemplateLength {
		return nil, validation.NewFieldError("title", fmt.Sprintf("must be at most %d bytes", slacktmpl.MaxLibraryTemplateLength))
	}
	if len(input.Text) > slacktmpl.MaxLibraryTemplateLength {
		return nil, validation.NewFieldError("text", fmt.Sprintf("must be at most %d bytes", slacktmpl.MaxLibraryTemplateLength))
	}
	if err := slacktmpl.Validate("title", input.Title); err != nil {
		return nil, validation.NewFieldError("title", err.Error())
	}
	if err := slacktmpl.Validate("body", input.Text); err != nil {
		return nil, validation.NewFieldError("text", err.Error())
	}

	var sample slacktmpl.Sample
	switch {
	case input.AlertmanagerPayload != nil && strings.TrimSpace(*input.AlertmanagerPayload) != "":
		if len(*input.AlertmanagerPayload) > maxPreviewPayloadSize {
			return nil, validation.NewFieldError("alertmanagerPayload", fmt.Sprintf("must be at most %d bytes", maxPreviewPayloadSize))
		}
		var data slacktmpl.AlertmanagerData
		if err := json.Unmarshal([]byte(*input.AlertmanagerPayload), &data); err != nil {
			return nil, validation.NewFieldError("alertmanagerPayload", "not valid Alertmanager webhook JSON: "+err.Error())
		}
		if len(data.Alerts) == 0 {
			return nil, validation.NewFieldError("alertmanagerPayload", "payload contains no alerts")
		}
		summary := data.CommonAnnotations["summary"]
		if summary == "" {
			summary = data.CommonLabels["alertname"]
		}
		if summary == "" {
			summary = "Alertmanager alert"
		}
		sample = slacktmpl.Sample{Summary: summary, ServiceName: "Preview service", Alertmanager: &data}
	default:
		id := "deployment"
		if input.SampleID != nil && *input.SampleID != "" {
			id = *input.SampleID
		}
		var ok bool
		sample, ok = slacktmpl.FindSample(id)
		if !ok {
			return nil, validation.NewFieldError("sampleID", "unknown sample")
		}
	}

	state := notification.AlertStateUnacknowledged
	if input.Status != nil {
		switch *input.Status {
		case graphql2.AlertStatusStatusAcknowledged:
			state = notification.AlertStateAcknowledged
		case graphql2.AlertStatusStatusClosed:
			state = notification.AlertStateClosed
		}
	}

	ctx = q.ConfigStore.Config().Context(ctx)
	p, err := slack.PreviewAlertTemplate(ctx, input.Title, input.Text, sample, state)
	if err != nil {
		return nil, validation.NewFieldError("text", err.Error())
	}
	res := &graphql2.SlackAlertTemplatePreview{Title: p.Title, Text: p.Text, Truncated: p.Truncated, Buttons: []graphql2.SlackAlertTemplatePreviewButton{}}
	for _, b := range p.Buttons {
		res.Buttons = append(res.Buttons, graphql2.SlackAlertTemplatePreviewButton{Label: b.Label, URL: b.URL})
	}
	return res, nil
}

// updateSlackLibrary applies fn to the latest stored library atomically.
func (a *App) updateSlackLibrary(ctx context.Context, fn func(cfg config.Config, lib slacktmpl.Library) (config.Config, slacktmpl.Library, error)) error {
	return a.ConfigStore.UpdateConfig(ctx, func(cfg config.Config) (config.Config, error) {
		lib, err := slacktmpl.ParseLibrary(cfg.Slack.AlertTemplateLibrary)
		if err != nil {
			return cfg, err
		}
		cfg, lib, err = fn(cfg, lib)
		if err != nil {
			return cfg, err
		}
		if err := slacktmpl.ValidateLibrary(lib); err != nil {
			return cfg, slackTemplateFieldErr(err)
		}
		cfg.Slack.AlertTemplateLibrary, err = lib.Encode()
		return cfg, err
	})
}

func (m *Mutation) CreateSlackAlertTemplate(ctx context.Context, input graphql2.CreateSlackAlertTemplateInput) (*graphql2.SlackAlertTemplate, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return nil, err
	}
	t := slacktmpl.LibraryTemplate{ID: uuid.NewString(), Name: strings.TrimSpace(input.Name), Title: input.Title, Text: input.Text}
	if err := slacktmpl.ValidateTemplate(t); err != nil {
		return nil, slackTemplateFieldErr(err)
	}
	err := (*App)(m).updateSlackLibrary(ctx, func(cfg config.Config, lib slacktmpl.Library) (config.Config, slacktmpl.Library, error) {
		return cfg, append(lib, t), nil
	})
	if err != nil {
		return nil, err
	}
	return &graphql2.SlackAlertTemplate{ID: t.ID, Name: t.Name, Title: t.Title, Text: t.Text, Revision: t.Revision(), Services: []service.Service{}}, nil
}

func (m *Mutation) UpdateSlackAlertTemplate(ctx context.Context, input graphql2.UpdateSlackAlertTemplateInput) (bool, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return false, err
	}
	err := (*App)(m).updateSlackLibrary(ctx, func(cfg config.Config, lib slacktmpl.Library) (config.Config, slacktmpl.Library, error) {
		// Checked against the latest stored library inside the locked config
		// transaction, so two stale editors cannot both succeed.
		res, err := lib.ApplyUpdate(slacktmpl.TemplateUpdate{
			ID:               input.ID,
			ExpectedRevision: input.ExpectedRevision,
			Name:             input.Name,
			Title:            input.Title,
			Text:             input.Text,
		})
		return cfg, res, slackTemplateFieldErr(err)
	})
	return err == nil, err
}

func (m *Mutation) DeleteSlackAlertTemplate(ctx context.Context, id string) (bool, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return false, err
	}
	err := (*App)(m).updateSlackLibrary(ctx, func(cfg config.Config, lib slacktmpl.Library) (config.Config, slacktmpl.Library, error) {
		tmpl, ok := lib.Find(id)
		if !ok {
			return cfg, lib, validation.NewFieldError("id", "template not found")
		}
		assign, err := slacktmpl.ParseAssignments(cfg.Slack.ServiceAlertTemplates)
		if err != nil {
			return cfg, lib, err
		}
		using := assign.ServicesUsing(tmpl.ID)
		svcs, err := (*App)(m).servicesByID(ctx, using)
		if err != nil {
			return cfg, lib, err
		}
		var names []string
		for _, svcID := range using {
			if s, ok := svcs[svcID]; ok {
				names = append(names, s.Name)
				continue
			}
			// The service no longer exists; drop its stale assignment.
			delete(assign, svcID)
		}
		if len(names) > 0 {
			sort.Strings(names)
			return cfg, lib, validation.NewFieldError("id", fmt.Sprintf("%q is used by %d service(s): %s. Assign a different template (or the default formatting) to those services first.", tmpl.Name, len(names), strings.Join(names, ", ")))
		}
		cfg.Slack.ServiceAlertTemplates = assign.Encode()

		res := make(slacktmpl.Library, 0, len(lib))
		for _, t := range lib {
			if t.ID != tmpl.ID {
				res = append(res, t)
			}
		}
		return cfg, res, nil
	})
	return err == nil, err
}

func (m *Mutation) SetServiceSlackAlertTemplate(ctx context.Context, input graphql2.SetServiceSlackAlertTemplateInput) (bool, error) {
	if err := requireSlackTemplateAdmin(ctx); err != nil {
		return false, err
	}
	if err := validate.UUID("serviceID", input.ServiceID); err != nil {
		return false, err
	}
	// Confirms the service exists (and that the caller can see it).
	if _, err := m.ServiceStore.FindOne(ctx, input.ServiceID); err != nil {
		return false, err
	}
	templateID := ""
	if input.TemplateID != nil {
		templateID = strings.TrimSpace(*input.TemplateID)
	}

	err := m.ConfigStore.UpdateConfig(ctx, func(cfg config.Config) (config.Config, error) {
		assign, err := slacktmpl.ParseAssignments(cfg.Slack.ServiceAlertTemplates)
		if err != nil {
			return cfg, err
		}
		svcID := strings.ToLower(input.ServiceID)
		if templateID == "" {
			delete(assign, svcID)
		} else {
			lib, err := slacktmpl.ParseLibrary(cfg.Slack.AlertTemplateLibrary)
			if err != nil {
				return cfg, err
			}
			tmpl, ok := lib.Find(templateID)
			if !ok {
				return cfg, validation.NewFieldError("templateID", "template not found; it may have been deleted")
			}
			assign[svcID] = tmpl.ID
		}
		cfg.Slack.ServiceAlertTemplates = assign.Encode()
		return cfg, nil
	})
	return err == nil, err
}
