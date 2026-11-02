package slack

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/config"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/slacktmpl"
)

var (
	// emptyLabelLine matches a labelled line with no value, e.g. "*Namespace:* ".
	emptyLabelLine = regexp.MustCompile(`(?m)^\*[^*]+:\*\s*$`)
	mrkdwnLink     = regexp.MustCompile(`<[^<>]*>`)
	validLink      = regexp.MustCompile(`^<https?://[^|<>\s]+\|[^<>|]+>$`)
)

func infraExample(t *testing.T) slacktmpl.Example {
	t.Helper()
	for _, e := range slacktmpl.Examples() {
		if e.Key == "infrastructure-clear" {
			return e
		}
	}
	t.Fatal("example not found")
	return slacktmpl.Example{}
}

func previewCfgCtx() context.Context {
	var cfg config.Config
	cfg.General.PublicURL = "https://goalert.example.com"
	return cfg.Context(context.Background())
}

// assertWellFormed checks properties every rendered message must have.
func assertWellFormed(t *testing.T, name string, p Preview) {
	t.Helper()
	assert.NotContains(t, p.Title+p.Text, "<no value>", name)
	assert.False(t, emptyLabelLine.MatchString(p.Text), "%s: empty labelled line in:\n%s", name, p.Text)
	assert.LessOrEqual(t, utf8.RuneCountInString(p.Text), maxSlackTextLength, name)
	assert.Equal(t, strings.Count(p.Text, "<"), strings.Count(p.Text, ">"), "%s: unbalanced link brackets", name)
	for _, l := range mrkdwnLink.FindAllString(p.Text, -1) {
		assert.Regexp(t, validLink, l, "%s: malformed link", name)
	}
}

func renderSample(t *testing.T, s slacktmpl.Sample, state notification.AlertState) Preview {
	t.Helper()
	e := infraExample(t)
	p, err := PreviewAlertTemplate(previewCfgCtx(), e.Title, e.Text, s, state)
	require.NoError(t, err)
	assertWellFormed(t, s.ID, p)
	return p
}

func previewSample(t *testing.T, id string, state notification.AlertState) Preview {
	t.Helper()
	s, ok := slacktmpl.FindSample(id)
	require.True(t, ok, id)
	return renderSample(t, s, state)
}

func sampleData(t *testing.T, id string) slacktmpl.Sample {
	t.Helper()
	s, ok := slacktmpl.FindSample(id)
	require.True(t, ok, id)
	cp := *s.Alertmanager
	cp.Alerts = append(slacktmpl.Alerts{}, s.Alertmanager.Alerts...)
	s.Alertmanager = &cp
	return s
}

func TestInfrastructureClear_Deployment(t *testing.T) {
	p := previewSample(t, "deployment", notification.AlertStateUnacknowledged)
	assert.Equal(t, "CRITICAL — Exporter is not ready", p.Title)
	assert.Equal(t, strings.Join([]string{
		"*Cluster:* pb-nonprod",
		"*Namespace:* nonprod-exporters",
		"*Workload:* pb-devtest-redis-exporter (Deployment)",
		"*State:* 0 of 1 replicas available",
		"*Details:* Deployment nonprod-exporters/pb-devtest-redis-exporter has no available replicas.",
		"*Next check:* Pod readiness and recent events",
		"*Link:* <https://grafana.example.com/d/workload?var-namespace=nonprod-exporters&var-workload=pb-devtest-redis-exporter|Open in Grafana>",
	}, "\n"), p.Text)
	// Scraper (kube-state-metrics) labels are not the affected resource.
	assert.NotContains(t, p.Text, "monitoring")
	assert.NotContains(t, p.Text, "kube-state-metrics")
	assert.False(t, p.Truncated)
	require.Len(t, p.Buttons, 1)
}

func TestInfrastructureClear_Statuses(t *testing.T) {
	p := previewSample(t, "deployment", notification.AlertStateAcknowledged)
	assert.Equal(t, "CRITICAL — Exporter is not ready", p.Title)
	assert.Contains(t, p.Text, "*State:* 0 of 1 replicas available")
	assert.Contains(t, p.Text, "*Next check:*")

	p = previewSample(t, "deployment", notification.AlertStateClosed)
	assert.Equal(t, "RESOLVED — Exporter is not ready", p.Title)
	assert.Contains(t, p.Text, "*Last reported state:* 0 of 1 replicas available")
	assert.NotContains(t, p.Text, "Next check")
}

func TestInfrastructureClear_ResourceKinds(t *testing.T) {
	cases := map[string][]string{
		"statefulset": {"*Namespace:* nonprod-data", "*Workload:* redis-master (StatefulSet)", "*State:* Ready replicas: 2"},
		"daemonset":   {"*Namespace:* kube-system", "*Workload:* aws-node (DaemonSet)", "*Details:* Some aws-node pods are not scheduled."},
		"pvc":         {"*Namespace:* nonprod-data", "*PVC:* data-redis-master-0", "*Node:* ip-10-0-3-21.ec2.internal", "*State:* 3% free"},
		"node":        {"*Node:* ip-10-0-3-21.ec2.internal", "*Next check:* follow the runbook"},
	}
	for id, want := range cases {
		t.Run(id, func(t *testing.T) {
			p := previewSample(t, id, notification.AlertStateUnacknowledged)
			for _, w := range want {
				assert.Contains(t, p.Text, w)
			}
			assert.NotContains(t, p.Text, "kube-state-metrics")
			assert.NotContains(t, p.Text, "*Namespace:* monitoring")
		})
	}
}

func TestInfrastructureClear_GroupedListsEveryResource(t *testing.T) {
	p := previewSample(t, "pods-grouped", notification.AlertStateUnacknowledged)
	assert.Equal(t, "WARNING — Pod is crash looping (3 alerts)", p.Title)
	assert.Contains(t, p.Text, "*Affected resources (3)*")
	assert.Contains(t, p.Text, "• nonprod-apps / Pod api-7f9c-1 — 4 restarts in 10 minutes")
	assert.Contains(t, p.Text, "• nonprod-apps / Pod api-7f9c-2 — 6 restarts in 10 minutes")
	assert.Contains(t, p.Text, "• nonprod-apps / Pod worker-5d2b-1 — 3 restarts in 10 minutes")
	assert.Contains(t, p.Text, "*Next check:* Container logs and last termination reason")
	assert.NotContains(t, p.Text, "not shown")
	assert.NotContains(t, p.Text, "left out")

	p = previewSample(t, "targets-many", notification.AlertStateUnacknowledged)
	assert.Equal(t, "WARNING — Scrape target is down (14 alerts)", p.Title)
	assert.Equal(t, 14, strings.Count(p.Text, "• Target redis-exporter"), "everything fits, so everything is listed")
	assert.NotContains(t, p.Text, "not shown")
}

func TestInfrastructureClear_SingleReceivedWithUpstreamOmissions(t *testing.T) {
	p := previewSample(t, "omitted-upstream", notification.AlertStateUnacknowledged)
	assert.Equal(t, "CRITICAL — Exporter is not ready (6 alerts)", p.Title)
	assert.Contains(t, p.Text, "*Affected resources (6; 1 received)*")
	assert.Contains(t, p.Text, "• nonprod-exporters / Deployment pb-devtest-redis-exporter — 0 of 1 replicas available")
	assert.Contains(t, p.Text, "Alertmanager left out 5 more alerts from this notification (max_alerts), so GoAlert has no details for them. "+
		"<https://alertmanager.example.com/#/alerts?filter=%7Balertname%3D%22KubeDeploymentReplicasUnavailable%22%7D|View the group in Alertmanager>")
	// Not the single-resource layout, and no promise that GoAlert has the rest.
	assert.NotContains(t, p.Text, "*Workload:*")
	assert.NotContains(t, p.Text, "GoAlert>")

	s := sampleData(t, "omitted-upstream")
	s.Alertmanager.TruncatedAlerts = 1
	s.Alertmanager.ExternalURL = "javascript:alert(1)"
	p = renderSample(t, s, notification.AlertStateUnacknowledged)
	assert.Contains(t, p.Text, "Alertmanager left out 1 more alert from this notification (max_alerts), so GoAlert has no details for it.\n")
	assert.NotContains(t, p.Text, "javascript")
	assert.NotContains(t, p.Text, "Alertmanager>")

	s.Alertmanager.ExternalURL = ""
	p = renderSample(t, s, notification.AlertStateClosed)
	assert.Equal(t, "RESOLVED — Exporter is not ready (2 alerts)", p.Title)
	assert.Contains(t, p.Text, "left out 1 more alert")
}

func TestInfrastructureClear_GroupedWithUpstreamOmissions(t *testing.T) {
	s := sampleData(t, "pods-grouped")
	s.Alertmanager.TruncatedAlerts = 2
	p := renderSample(t, s, notification.AlertStateAcknowledged)
	assert.Equal(t, "WARNING — Pod is crash looping (5 alerts)", p.Title)
	assert.Contains(t, p.Text, "*Affected resources (5; 3 received)*")
	assert.Equal(t, 3, strings.Count(p.Text, "• nonprod-apps / Pod "))
	assert.Contains(t, p.Text, "Alertmanager left out 2 more alerts")
	assert.NotContains(t, p.Text, "not shown")
}

func TestInfrastructureClear_LongContentFitsBudget(t *testing.T) {
	check := func(t *testing.T, p Preview, received, omitted int) {
		t.Helper()
		assert.False(t, p.Truncated, "list shortening must avoid a hard cut")
		bullets := 0
		for _, line := range strings.Split(p.Text, "\n") {
			if !strings.HasPrefix(line, "• ") {
				continue
			}
			bullets++
			// Every listed entry is complete: name, state and its own link.
			assert.Regexp(t, `^• nonprod-very-long-namespace-name / Pod very-long-application-name-.*-abcde — Not ready .* <https://grafana\.example\.com/[^|]+-abcde\|Grafana>$`, line)
		}
		require.Greater(t, bullets, 0)
		require.Less(t, bullets, received, "should have been shortened")
		notShown := received - bullets
		assert.Contains(t, p.Text, "…and "+strconv.Itoa(notShown)+" more received alerts not shown (Slack length limit). <https://goalert.example.com/alerts/1234|Open the alert in GoAlert>")
		if omitted > 0 {
			assert.Contains(t, p.Text, "Alertmanager left out "+strconv.Itoa(omitted)+" more alerts")
		}
	}

	p := previewSample(t, "long-names", notification.AlertStateUnacknowledged)
	assert.Equal(t, "WARNING — Pod is not ready (40 alerts)", p.Title)
	check(t, p, 40, 0)

	s := sampleData(t, "long-names")
	s.Alertmanager.TruncatedAlerts = 7
	s.Alertmanager.ExternalURL = "https://alertmanager.example.com/"
	p = renderSample(t, s, notification.AlertStateUnacknowledged)
	assert.Equal(t, "WARNING — Pod is not ready (47 alerts)", p.Title)
	assert.Contains(t, p.Text, "*Affected resources (47; 40 received)*")
	check(t, p, 40, 7)
	assert.Contains(t, p.Text, "|View the group in Alertmanager>")
}

func TestLibraryRender_HardCutKeepsLinesAndLinks(t *testing.T) {
	// A template that cannot shorten itself: one huge line, then many lines.
	long := strings.Repeat("x", 3500)
	s := slacktmpl.Sample{Summary: "s", Details: long}
	p, err := PreviewAlertTemplate(previewCfgCtx(), "t", `start <https://example.com/a|a> {{ .Details }} <https://example.com/b|b>`, s, notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.True(t, p.Truncated)
	assertWellFormed(t, "hardcut-line", p)
	assert.True(t, strings.HasSuffix(p.Text, shortenedNotice[1:]) || strings.HasSuffix(p.Text, shortenedNotice))

	p, err = PreviewAlertTemplate(previewCfgCtx(), "t", `{{ range $i, $_ := (firstN 200 .Alerts) }}{{ end }}{{ range (list 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30) }}line {{ . }} <https://example.com/{{ . }}|link {{ . }}> padding padding padding padding padding padding padding padding padding padding padding padding padding padding padding
{{ end }}`, s, notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.True(t, p.Truncated)
	assertWellFormed(t, "hardcut-lines", p)
	for _, line := range strings.Split(strings.TrimSuffix(p.Text, shortenedNotice), "\n") {
		assert.Regexp(t, `^line \d+ <https://example.com/\d+\|link \d+> (padding ?)+$`, line)
	}
}

func TestInfrastructureClear_MetadataOnly(t *testing.T) {
	for _, st := range []notification.AlertState{notification.AlertStateUnacknowledged, notification.AlertStateAcknowledged, notification.AlertStateClosed} {
		p := previewSample(t, "metadata-only", st)
		if st == notification.AlertStateClosed {
			assert.Equal(t, "RESOLVED — Exporter is not ready", p.Title)
			assert.Contains(t, p.Text, "*Last reported state:* 0 of 1 replicas available")
		} else {
			assert.Equal(t, "CRITICAL — Exporter is not ready", p.Title, "no count: the number of alerts is unknown")
			assert.Contains(t, p.Text, "*State:* 0 of 1 replicas available")
			assert.Contains(t, p.Text, "*Next check:* Pod readiness and recent events")
		}
		assert.Contains(t, p.Text, "_Per-alert details are not available for this alert, so only its stored labels are shown.")
		assert.Contains(t, p.Text, "may cover more than one resource")
		assert.NotContains(t, p.Text, "too large", "the cause is not recorded")
		assert.Contains(t, p.Text, "*Cluster:* pb-nonprod")
		assert.Contains(t, p.Text, "*Namespace:* nonprod-exporters")
		assert.Contains(t, p.Text, "*Workload:* pb-devtest-redis-exporter (Deployment)")
		assert.Contains(t, p.Text, "<https://goalert.example.com/alerts/1234|Open the alert in GoAlert>")
		assert.NotContains(t, p.Text, "monitoring")
		assert.NotContains(t, p.Text, "kube-state-metrics")
		assert.NotContains(t, p.Text, "Affected resources")
	}
}

func TestInfrastructureClear_MetadataOnlyAmbiguousGroup(t *testing.T) {
	// Ingestion keeps only values common to all alerts of a group.
	s := slacktmpl.Sample{Summary: "Pod is crash looping and 2 others", Meta: map[string]string{
		"alertname": "KubePodCrashLooping", "severity": "warning", "cluster": "pb-nonprod", "namespace": "nonprod-apps", "container": "app",
	}}
	p := renderSample(t, s, notification.AlertStateUnacknowledged)
	assert.Equal(t, "WARNING — KubePodCrashLooping", p.Title)
	assert.Contains(t, p.Text, "*Namespace:* nonprod-apps")
	assert.NotContains(t, p.Text, "*Pod:*", "no pod is common to the group")
	assert.NotContains(t, p.Text, "*Workload:*")
	assert.Contains(t, p.Text, "may cover more than one resource")
}

func TestInfrastructureClear_MalformedPayloadUsesFlattenedMetadata(t *testing.T) {
	e := infraExample(t)
	cfg := libraryConfig(t, e.Title, e.Text)
	ctx := cfg.Context(context.Background())
	meta := map[string]string{
		slacktmpl.MetadataKey: "{not json",
		"alertname":           "KubeDeploymentReplicasUnavailable", "severity": "critical", "cluster": "pb-nonprod",
		"namespace": "monitoring", "exported_namespace": "nonprod-exporters", "deployment": "pb-devtest-redis-exporter",
		"pod": "kube-state-metrics-6d9c7b5f4-abcde", "summary": "Exporter is not ready", "state": "0 of 1 replicas available",
	}
	title, text, _, err := renderAlertTemplates(ctx, 7, "Exporter is not ready", "details", assignedSvc, "svc", "prometheusAlertmanager", meta, "Unacknowledged", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "CRITICAL — Exporter is not ready", title)
	assert.Contains(t, text, "*Namespace:* nonprod-exporters")
	assert.Contains(t, text, "*Workload:* pb-devtest-redis-exporter (Deployment)")
	assert.NotContains(t, text, "kube-state-metrics")
	assert.Contains(t, text, "_The stored alert payload could not be read, so only its stored labels are shown.")
	assert.NotContains(t, text, "too large")
}

func TestInfrastructureClear_ManualAndMissingFields(t *testing.T) {
	p := previewSample(t, "manual", notification.AlertStateUnacknowledged)
	assert.Equal(t, "ALERT — Database failover test", p.Title)
	assert.Equal(t, "*Details:* Created manually from the GoAlert UI.", p.Text)
	assert.Empty(t, p.Buttons)

	s := slacktmpl.Sample{Summary: "Something", Alertmanager: &slacktmpl.AlertmanagerData{
		CommonLabels: slacktmpl.KV{"alertname": "SomethingHappened"},
		Alerts:       slacktmpl.Alerts{{Status: "firing", Labels: slacktmpl.KV{"alertname": "SomethingHappened"}}},
	}}
	p = renderSample(t, s, notification.AlertStateUnacknowledged)
	assert.Equal(t, "ALERT — SomethingHappened", p.Title)
	assert.Empty(t, p.Text)

	// Unsafe dashboard URL is not rendered as a link.
	s.Alertmanager.Alerts[0].Annotations = slacktmpl.KV{"dashboard_url": "javascript:alert(1)", "state": "a <b> & c"}
	p = renderSample(t, s, notification.AlertStateUnacknowledged)
	assert.Equal(t, "*State:* a &lt;b&gt; &amp; c", p.Text)
}

func TestInfrastructureClear_AllSamplesAllStates(t *testing.T) {
	for _, s := range slacktmpl.Samples() {
		for _, st := range []notification.AlertState{notification.AlertStateUnacknowledged, notification.AlertStateAcknowledged, notification.AlertStateClosed} {
			previewSample(t, s.ID, st)
		}
	}
}

// Ordinary alerts must not be described as Alertmanager notifications with a
// missing or oversized payload just because they carry metadata.
func TestInfrastructureClear_OrdinaryAlertWithMetadata(t *testing.T) {
	e := infraExample(t)
	cfg := libraryConfig(t, e.Title, e.Text)
	cfg.General.PublicURL = "https://goalert.example.com"
	ctx := cfg.Context(context.Background())

	states := map[notification.AlertState]string{
		notification.AlertStateUnacknowledged: "ALERT — Generic API alert",
		notification.AlertStateAcknowledged:   "ALERT — Generic API alert",
		notification.AlertStateClosed:         "RESOLVED — Generic API alert",
	}
	for st, wantTitle := range states {
		title, text, buttons, err := renderAlertTemplates(ctx, 7, "Generic API alert", "Raised by the generic API.", assignedSvc, "svc", "generic", map[string]string{"owner": "infra"}, "log", st)
		require.NoError(t, err)
		assert.Equal(t, wantTitle, title)
		assert.Equal(t, "*Details:* Raised by the generic API.", text)
		assert.Empty(t, buttons)
	}

	// Resource identity in ordinary metadata is still shown, without a caveat.
	_, text, _, err := renderAlertTemplates(ctx, 7, "API errors", "", assignedSvc, "svc", "generic",
		map[string]string{"namespace": "nonprod-apps", "deployment": "api", "owner": "infra"}, "log", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "*Namespace:* nonprod-apps\n*Workload:* api (Deployment)", text)

	p := previewSample(t, "generic-metadata", notification.AlertStateUnacknowledged)
	assert.Equal(t, "ALERT — Generic API alert", p.Title)
	assert.Equal(t, "*Details:* Raised by the generic API.", p.Text)
}

func TestTemplateData_AlertmanagerDataState(t *testing.T) {
	ctx := previewCfgCtx()
	full := amMeta(t)
	const am, generic, manual = "prometheusAlertmanager", "generic", "manual"
	cases := []struct {
		name   string
		source string
		meta   map[string]string
		want   string
	}{
		{"complete payload", am, full, "complete"},
		{"malformed payload", am, map[string]string{slacktmpl.MetadataKey: "{", "alertname": "X"}, "unreadable"},
		{"malformed payload only", am, map[string]string{slacktmpl.MetadataKey: "not json"}, "unreadable"},
		{"alertmanager, same-name group fallback", am, map[string]string{"alertname": "X", "namespace": "a"}, "metadata-only"},
		{"alertmanager, mixed-name group fallback (no alertname)", am, map[string]string{"cluster": "c", "namespace": "kube-system", "severity": "critical"}, "metadata-only"},
		{"alertmanager, no metadata at all", am, nil, "metadata-only"},
		{"manual, no metadata", manual, nil, "none"},
		{"generic metadata", generic, map[string]string{"owner": "infra"}, "none"},
		{"generic alert containing alertname", generic, map[string]string{"alertname": "X", "namespace": "apps"}, "none"},
		{"manual alert containing alertname", manual, map[string]string{"alertname": "X"}, "none"},
		{"unknown source, alertname hint", "", map[string]string{"alertname": "X"}, "metadata-only"},
		{"unknown source, ordinary metadata", "", map[string]string{"owner": "infra"}, "none"},
	}
	for _, c := range cases {
		d := templateData(ctx, 1, "s", "d", "svc", "svc", c.source, c.meta, "", notification.AlertStateUnacknowledged)
		assert.Equal(t, c.want, d.AlertmanagerDataState, c.name)
		assert.Equal(t, c.source, d.Source, c.name)
	}
}

// TestInfrastructureClear_MixedNameGroupSizeFallback renders the metadata
// that ingestion keeps when a mixed-name group's payload exceeds the
// metadata limit (see prometheusalertmanager TestMetadata_MixedNameGroupSizeFallback).
func TestInfrastructureClear_MixedNameGroupSizeFallback(t *testing.T) {
	e := infraExample(t)
	cfg := libraryConfig(t, e.Title, e.Text)
	cfg.General.PublicURL = "https://goalert.example.com"
	ctx := cfg.Context(context.Background())
	meta := map[string]string{"cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical"}
	summary := "CoreDNS is unavailable and 1 others" // what ingestion stores as the alert summary

	for st, prefix := range map[notification.AlertState]string{
		notification.AlertStateUnacknowledged: "CRITICAL — ",
		notification.AlertStateAcknowledged:   "CRITICAL — ",
		notification.AlertStateClosed:         "RESOLVED — ",
	} {
		title, text, _, err := renderAlertTemplates(ctx, 7, summary, "details", assignedSvc, "svc", "prometheusAlertmanager", meta, "log", st)
		require.NoError(t, err)
		assert.Equal(t, prefix+summary, title)
		assert.Equal(t, strings.Join([]string{
			"_Per-alert details are not available for this alert, so only its stored labels are shown. For a grouped notification these are the values shared by every alert, so it may cover more than one resource._",
			"*Cluster:* pb-nonprod",
			"*Namespace:* kube-system",
			"<https://goalert.example.com/alerts/7|Open the alert in GoAlert> for the received details.",
		}, "\n"), text)
		assert.NotContains(t, text, "too large")
	}

	// Same-name group fallback keeps alertname and still warns.
	_, text, _, err := renderAlertTemplates(ctx, 7, "Pod is crash looping and 2 others", "d", assignedSvc, "svc", "prometheusAlertmanager",
		map[string]string{"alertname": "KubePodCrashLooping", "namespace": "nonprod-apps", "severity": "warning"}, "log", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Contains(t, text, "may cover more than one resource")
	assert.Contains(t, text, "*Namespace:* nonprod-apps")

	// Generic and manual alerts with an alertname are not Alertmanager alerts.
	for _, src := range []string{"generic", "manual"} {
		_, text, _, err = renderAlertTemplates(ctx, 7, "Something", "Raised elsewhere.", assignedSvc, "svc", src,
			map[string]string{"alertname": "Something", "namespace": "apps"}, "log", notification.AlertStateUnacknowledged)
		require.NoError(t, err)
		assert.Equal(t, "*Namespace:* apps\n*Details:* Raised elsewhere.", text, src)
	}
}

func TestInfrastructureClear_NoPayloadWordingForCompleteData(t *testing.T) {
	for _, id := range []string{"deployment", "pods-grouped", "omitted-upstream", "long-names", "targets-many"} {
		for _, st := range []notification.AlertState{notification.AlertStateUnacknowledged, notification.AlertStateAcknowledged, notification.AlertStateClosed} {
			p := previewSample(t, id, st)
			assert.NotContains(t, p.Text, "payload", id)
			assert.NotContains(t, p.Text, "stored labels", id)
		}
	}
}

// Default/shared formatting for unassigned services is unaffected by metadata.
func TestDefaultFormatting_GenericMetadataUnchanged(t *testing.T) {
	var base config.Config
	meta := map[string]string{"owner": "infra"}
	bt, bx, bb, err := renderAlertTemplates(base.Context(context.Background()), 7, "Generic API alert", "d", otherSvc, "svc", "", meta, "log", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, "Alert #7: Generic API alert", bt)
	cfg := libraryConfig(t, "{{ .Summary }}", "x")
	tt, tx, tb, err := renderAlertTemplates(cfg.Context(context.Background()), 7, "Generic API alert", "d", otherSvc, "svc", "", meta, "log", notification.AlertStateUnacknowledged)
	require.NoError(t, err)
	assert.Equal(t, bt, tt)
	assert.Equal(t, bx, tx)
	assert.Equal(t, bb, tb)
}
