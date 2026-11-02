package slacktmpl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceFromLabels(t *testing.T) {
	// kube-state-metrics with collided target labels: plain labels are the scraper.
	r := ResourceFromLabels(map[string]string{
		"namespace": "monitoring", "pod": "kube-state-metrics-abc", "container": "kube-state-metrics",
		"job": "kube-state-metrics", "instance": "10.0.0.1:8080", "node": "ip-scraper",
		"exported_namespace": "nonprod-exporters", "deployment": "redis-exporter", "cluster": "pb-nonprod",
	})
	assert.Equal(t, K8sResource{Cluster: "pb-nonprod", Namespace: "nonprod-exporters", Kind: "Deployment", Name: "redis-exporter", Scraped: true}, r)
	assert.True(t, r.IsWorkload())

	r = ResourceFromLabels(map[string]string{"exported_namespace": "apps", "exported_pod": "api-1", "exported_container": "app", "pod": "ksm"})
	assert.Equal(t, "Pod", r.Kind)
	assert.Equal(t, "api-1", r.Pod)
	assert.Equal(t, "app", r.Container)

	r = ResourceFromLabels(map[string]string{"namespace": "apps", "pod": "api-1", "container": "app"})
	assert.Equal(t, K8sResource{Namespace: "apps", Kind: "Pod", Name: "api-1", Pod: "api-1", Container: "app"}, r)

	r = ResourceFromLabels(map[string]string{"namespace": "data", "persistentvolumeclaim": "data-0", "node": "n1"})
	assert.Equal(t, "PVC", r.Kind)
	assert.Equal(t, "n1", r.Node)

	assert.Equal(t, "Node", ResourceFromLabels(map[string]string{"node": "n1"}).Kind)
	assert.Equal(t, K8sResource{Kind: "Target", Name: "redis", Job: "redis", Instance: "1.2.3.4:9121"}, ResourceFromLabels(map[string]string{"job": "redis", "instance": "1.2.3.4:9121"}))
	// Scraper-only labels never become the affected target.
	assert.Equal(t, "", ResourceFromLabels(map[string]string{"exported_namespace": "x", "job": "ksm", "instance": "i"}).Kind)
	assert.Equal(t, K8sResource{}, ResourceFromLabels(nil))
}

func TestSafeURL(t *testing.T) {
	assert.Equal(t, "https://g.example.com/d/x?a=1&b=2", SafeURL("https://g.example.com/d/x?a=1&b=2"))
	for _, bad := range []string{"", "javascript:alert(1)", "/relative", "https://", "https://a.com/x|y", "https://a.com/<x>", "https://a.com/a b", "ftp://a.com"} {
		assert.Empty(t, SafeURL(bad), bad)
	}
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", Truncate(5, "abc"))
	assert.Equal(t, "abcd…", Truncate(5, "abcdefgh"))
	assert.Equal(t, "ééé…", Truncate(4, "éééééé"))
	assert.Equal(t, "", Truncate(0, "abc"))
}

func TestAlertmanagerAlertsURL(t *testing.T) {
	d := AlertmanagerData{ExternalURL: "https://am.example.com/", GroupLabels: KV{"alertname": "X", "cluster": "c"}}
	assert.Equal(t, "https://am.example.com/#/alerts?filter=%7Balertname%3D%22X%22%2Ccluster%3D%22c%22%7D", d.AlertmanagerAlertsURL())
	d.GroupLabels = nil
	assert.Equal(t, "https://am.example.com/#/alerts", d.AlertmanagerAlertsURL())
	for _, bad := range []string{"", "javascript:alert(1)", "http://am example.com"} {
		d.ExternalURL = bad
		assert.Empty(t, d.AlertmanagerAlertsURL(), bad)
	}
}
