package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/config"
	"github.com/target/goalert/gadb"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/nfymsg"
	"github.com/target/goalert/notification/slacktmpl"
)

func TestLibraryInfoColour(t *testing.T) {
	e := infraExample(t)
	for _, tc := range []struct {
		name       string
		severities []string
		common     string
		omitted    int
		want       string
	}{
		{"info", []string{"info"}, "info", 0, colorInfo},
		{"case insensitive", []string{" INFO "}, "", 0, colorInfo},
		{"critical", []string{"critical"}, "critical", 0, colorUnacked},
		{"warning", []string{"warning"}, "warning", 0, colorUnacked},
		{"missing", []string{""}, "info", 0, colorUnacked},
		{"mixed", []string{"info", "critical"}, "", 0, colorUnacked},
		{"all info", []string{"info", "info"}, "info", 0, colorInfo},
		{"omitted unknown", []string{"info"}, "", 2, colorUnacked},
		{"omitted info", []string{"info"}, "info", 2, colorInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sampleData(t, "deployment")
			s.Alertmanager.Alerts = nil
			for _, sev := range tc.severities {
				s.Alertmanager.Alerts = append(s.Alertmanager.Alerts, slacktmpl.AlertmanagerAlert{Status: "firing", Labels: slacktmpl.KV{"severity": sev}})
			}
			s.Alertmanager.CommonLabels = slacktmpl.KV{"severity": tc.common}
			s.Alertmanager.TruncatedAlerts = tc.omitted
			for _, st := range []notification.AlertState{notification.AlertStateUnacknowledged, notification.AlertStateAcknowledged, notification.AlertStateClosed} {
				p, err := PreviewAlertTemplate(previewCfgCtx(), e.Title, e.Text, s, st)
				require.NoError(t, err)
				want := tc.want
				if st == notification.AlertStateAcknowledged {
					want = colorAcked
				}
				if st == notification.AlertStateClosed {
					want = colorClosed
				}
				assert.Equal(t, want, p.Color)
			}
		})
	}
}

// Capture the real sender's HTTP request using a local fake Slack server.
// No Slack credentials or external delivery are used.
func TestInfoColourDelivery(t *testing.T) {
	e := infraExample(t)
	for _, tc := range []struct {
		name string
		cfg  func() config.Config
		svc  string
		want string
	}{
		{"assigned", func() config.Config { return libraryConfig(t, e.Title, e.Text) }, assignedSvc, colorInfo},
		{"unassigned", func() config.Config { return libraryConfig(t, e.Title, e.Text) }, otherSvc, colorUnacked},
		{"default", func() config.Config { return config.Config{} }, assignedSvc, colorUnacked},
		{"render fallback", func() config.Config { return libraryConfig(t, "{{ (index .Alerts 99).Status }}", "x") }, assignedSvc, colorUnacked},
		{"missing template", func() config.Config {
			c := libraryConfig(t, e.Title, e.Text)
			c.Slack.AlertTemplateLibrary = ""
			return c
		}, assignedSvc, colorUnacked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []struct {
				Color  string
				Blocks json.RawMessage
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/chat.postMessage", r.URL.Path)
				assert.NoError(t, json.Unmarshal([]byte(r.FormValue("attachments")), &got))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true,"channel":"C_TEST","ts":"1.0"}`))
			}))
			defer srv.Close()
			cfg := tc.cfg()
			cfg.Slack.AccessToken = "local-test-token"
			ctx := cfg.Context(context.Background())
			sender, err := NewChannelSender(ctx, Config{BaseURL: srv.URL, Client: srv.Client()})
			require.NoError(t, err)
			_, err = sender.SendMessage(ctx, notification.Alert{
				Base:    nfymsg.Base{ID: "test", Dest: gadb.DestV1{Type: DestTypeSlackChannel, Args: map[string]string{FieldSlackChannelID: "C_TEST"}}},
				AlertID: 7, Summary: "Notification check", ServiceID: tc.svc, Source: "generic",
				Meta: map[string]string{"severity": "info", "dashboard_url": "https://grafana.example.com/d/infra"},
			})
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0].Color)
			if tc.name == "assigned" {
				assert.Contains(t, string(got[0].Blocks), `"text":"Grafana Dashboard"`)
				assert.Contains(t, string(got[0].Blocks), `"type":"button"`)
				assert.Contains(t, string(got[0].Blocks), `"url":"https://grafana.example.com/d/infra"`)
				assert.NotContains(t, string(got[0].Blocks), "*Link:*")
			}
		})
	}
}

func TestInfoColourFlattenedMetadata(t *testing.T) {
	for _, state := range []string{"none", "metadata-only", "unreadable"} {
		assert.Equal(t, colorInfo, libraryAlertColor(slacktmpl.Data{AlertmanagerDataState: state, AlertMeta: map[string]string{"severity": "info"}}, notification.AlertStateUnacknowledged))
		assert.Equal(t, colorUnacked, libraryAlertColor(slacktmpl.Data{AlertmanagerDataState: state}, notification.AlertStateUnacknowledged))
	}
}
