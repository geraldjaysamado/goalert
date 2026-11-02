package prometheus

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/alert"
	"github.com/target/goalert/notification/slacktmpl"
)

// mixedNameGroupPayload is an Alertmanager group with two different alert
// names whose full payload exceeds GoAlert's 32 KiB metadata limit.
func mixedNameGroupPayload() string {
	return `{
		"status": "firing",
		"receiver": "goalert",
		"groupKey": "{}:{namespace=\"kube-system\"}",
		"groupLabels": {"namespace": "kube-system"},
		"commonLabels": {"cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical"},
		"commonAnnotations": {},
		"alerts": [
			{"status": "firing",
			 "labels": {"alertname": "CoreDNSUnavailable", "cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical", "deployment": "coredns"},
			 "annotations": {"summary": "CoreDNS is unavailable", "description": "` + strings.Repeat("a", 20000) + `"}},
			{"status": "firing",
			 "labels": {"alertname": "KubeProxyUnavailable", "cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical", "daemonset": "kube-proxy"},
			 "annotations": {"summary": "kube-proxy is unavailable", "description": "` + strings.Repeat("b", 20000) + `"}}
		]
	}`
}

// TestMetadata_MixedNameGroupSizeFallback reproduces the ingestion path that
// drops the full payload but keeps the common (flattened) metadata, which
// has no alertname because the group's alert names differ.
func TestMetadata_MixedNameGroupSizeFallback(t *testing.T) {
	var b postBody
	require.NoError(t, json.Unmarshal([]byte(mixedNameGroupPayload()), &b))

	meta, err := b.Metadata()
	require.NoError(t, err)
	require.Error(t, alert.ValidateMetadata(meta), "full payload must exceed the metadata limit")

	delete(meta, slacktmpl.MetadataKey) // what ingestion does next
	require.NoError(t, alert.ValidateMetadata(meta), "flattened metadata must be accepted")

	assert.Equal(t, map[string]string{"cluster": "pb-nonprod", "namespace": "kube-system", "severity": "critical"}, meta)
	assert.NotContains(t, meta, "alertname", "mixed-name groups have no common alertname")
}
