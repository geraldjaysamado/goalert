package smoke

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/target/goalert/notification/slacktmpl"
	"github.com/target/goalert/test/smoke/harness"
)

// TestSlackTemplateAlertmanagerMetadataFallback sends an Alertmanager group
// with two different alert names whose payload exceeds the 32 KiB metadata
// limit. Ingestion keeps only the common labels (no alertname). The service's
// assigned "Infrastructure — Clear" template must still recognize the alert as
// an Alertmanager notification (from the alert source) and warn that
// per-alert details are unavailable and it may cover more than one resource,
// through firing, acknowledged and resolved updates. A manual alert carrying
// an alertname must not get that warning.
func TestSlackTemplateAlertmanagerMetadataFallback(t *testing.T) {
	t.Parallel()

	const sql = `
	insert into escalation_policies (id, name, repeat)
	values ({{uuid "eid"}}, 'esc policy', 0);

	insert into escalation_policy_steps (id, escalation_policy_id, delay)
	values ({{uuid "esid"}}, {{uuid "eid"}}, 30);

	insert into notification_channels (id, type, name, value)
	values ({{uuid "chan"}}, 'SLACK', '#infra', {{slackChannelID "infra"}});

	insert into escalation_policy_actions (escalation_policy_step_id, channel_id)
	values ({{uuid "esid"}}, {{uuid "chan"}});

	insert into services (id, escalation_policy_id, name)
	values ({{uuid "sid"}}, {{uuid "eid"}}, 'EKS Infrastructure Nonprod');

	insert into integration_keys (id, type, name, service_id)
	values ({{uuid "int_key"}}, 'prometheusAlertmanager', 'am', {{uuid "sid"}});
`
	h := harness.NewHarness(t, sql, "prometheus-alertmanager-integration")
	defer h.Close()

	var example slacktmpl.Example
	for _, e := range slacktmpl.Examples() {
		if e.Key == "infrastructure-clear" {
			example = e
		}
	}
	require.NotEmpty(t, example.Text)
	const tmplID = "6f1c2c62-55d4-4f43-8a1e-3a4f7f7c9a01"
	lib, err := slacktmpl.Library{{ID: tmplID, Name: example.Name, Title: example.Title, Text: example.Text}}.Encode()
	require.NoError(t, err)
	h.SetConfigValue("Slack.SigningSecret", "smoke-test-secret")
	h.SetConfigValue("Slack.AlertTemplateLibrary", lib)
	h.SetConfigValue("Slack.ServiceAlertTemplates", h.UUID("sid")+"="+tmplID)

	post := func(status string) {
		t.Helper()
		body := `{
			"status": "` + status + `",
			"receiver": "goalert",
			"groupKey": "{}:{namespace=\"kube-system\"}",
			"groupLabels": {"namespace": "kube-system"},
			"commonLabels": {"cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical"},
			"commonAnnotations": {},
			"alerts": [
				{"status": "` + status + `",
				 "labels": {"alertname": "CoreDNSUnavailable", "cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical", "deployment": "coredns"},
				 "annotations": {"summary": "CoreDNS is unavailable", "description": "` + strings.Repeat("a", 20000) + `"}},
				{"status": "` + status + `",
				 "labels": {"alertname": "KubeProxyUnavailable", "cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical", "daemonset": "kube-proxy"},
				 "annotations": {"summary": "kube-proxy is unavailable", "description": "` + strings.Repeat("b", 20000) + `"}}
			]
		}`
		resp, err := http.Post(h.URL()+"/api/v2/prometheusalertmanager/incoming?token="+h.UUID("int_key"), "application/json", strings.NewReader(body))
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	warning := "Per-alert details are not available for this alert, so only its stored labels are shown. For a grouped notification these are the values shared by every alert, so it may cover more than one resource."

	post("firing")

	// Ingestion dropped the full payload but kept the common labels.
	res := h.GraphQLQuery2(`query{ alert(id: 1){ meta { key value } } }`)
	require.Empty(t, res.Errors)
	require.NotContains(t, string(res.Data), slacktmpl.MetadataKey, "full payload should exceed the metadata limit")
	require.Contains(t, string(res.Data), `"kube-system"`)
	require.NotContains(t, string(res.Data), "alertname")

	ch := h.Slack().Channel("infra")
	msg := ch.ExpectMessage("CRITICAL — CoreDNS is unavailable and 1 others", warning, "*Cluster:* pb-nonprod", "*Namespace:* kube-system", "Open the alert in GoAlert")
	msg.AssertNotText("too large", "*Workload:*", "*Affected resources")

	res = h.GraphQLQuery2(`mutation{ updateAlerts(input:{alertIDs: [1], newStatus: StatusAcknowledged}){ id } }`)
	require.Empty(t, res.Errors)
	upd := msg.ExpectUpdate()
	upd.AssertText("CRITICAL — CoreDNS is unavailable and 1 others", warning, "Acknowledged")

	post("resolved")
	upd = msg.ExpectUpdate()
	upd.AssertText("RESOLVED — CoreDNS is unavailable and 1 others", warning, "*Namespace:* kube-system")
	upd.AssertNotText("too large")

	// A manual alert whose metadata includes alertname is not an Alertmanager alert.
	res = h.GraphQLQuery2(`mutation{ createAlert(input:{serviceID: "` + h.UUID("sid") + `", summary: "Manual check", details: "Raised by hand.",
		meta: [{key: "alertname", value: "ManualCheck"}, {key: "namespace", value: "apps"}]}){ id } }`)
	require.Empty(t, res.Errors)
	manual := ch.ExpectMessage("ALERT — Manual check", "*Namespace:* apps", "*Description:* Raised by hand.")
	manual.AssertNotText("Per-alert details", "may cover more than one resource")
}
