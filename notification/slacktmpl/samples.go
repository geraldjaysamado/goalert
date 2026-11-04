package slacktmpl

import (
	"fmt"
	"time"
)

// Sample is illustrative alert data used to preview templates without
// sending a notification. Alertmanager is nil for alerts that did not come
// from Alertmanager (manual alerts, generic API, email, etc.).
type Sample struct {
	ID           string
	Name         string
	Summary      string
	Details      string
	ServiceName  string
	Alertmanager *AlertmanagerData

	// Meta is extra alert metadata. With an alertname but no Alertmanager
	// data it mimics flattened Alertmanager labels stored without the full
	// payload; without alertname it is ordinary alert metadata.
	Meta map[string]string

	// Source is the alert source used for the preview. Empty means
	// "prometheusAlertmanager" when Alertmanager or Meta is set, else "manual".
	Source string
}

var sampleStart = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

func amData(common KV, commonAnn KV, alerts ...AlertmanagerAlert) *AlertmanagerData {
	for i := range alerts {
		if alerts[i].Status == "" {
			alerts[i].Status = "firing"
		}
		if alerts[i].StartsAt.IsZero() {
			alerts[i].StartsAt = sampleStart
		}
	}
	return &AlertmanagerData{
		Receiver:          "goalert",
		Status:            "firing",
		Alerts:            alerts,
		GroupLabels:       KV{"alertname": common["alertname"]},
		CommonLabels:      common,
		CommonAnnotations: commonAnn,
	}
}

func merge(a, b KV) KV {
	res := make(KV, len(a)+len(b))
	for k, v := range a {
		res[k] = v
	}
	for k, v := range b {
		res[k] = v
	}
	return res
}

// Samples returns the built-in preview samples. Values are illustrative.
func Samples() []Sample {
	ksmScraper := KV{"namespace": "monitoring", "pod": "kube-state-metrics-6d9c7b5f4-abcde", "container": "kube-state-metrics", "job": "kube-state-metrics", "instance": "10.0.12.34:8080"}

	deployLabels := merge(ksmScraper, KV{"alertname": "KubeDeploymentReplicasUnavailable", "severity": "critical", "cluster": "pb-nonprod", "exported_namespace": "nonprod-exporters", "deployment": "pb-devtest-redis-exporter"})
	deployAnn := KV{
		"summary":       "Exporter is not ready",
		"description":   "Deployment nonprod-exporters/pb-devtest-redis-exporter has no available replicas.",
		"state":         "0 of 1 replicas available",
		"dashboard_url": "https://grafana.example.com/d/workload?var-namespace=nonprod-exporters&var-workload=pb-devtest-redis-exporter",
	}

	podAlert := func(pod, restarts string) AlertmanagerAlert {
		return AlertmanagerAlert{
			Labels:      KV{"alertname": "KubePodCrashLooping", "severity": "warning", "cluster": "pb-nonprod", "namespace": "nonprod-apps", "pod": pod, "container": "app"},
			Annotations: KV{"summary": "Pod is crash looping", "value_used": restarts + " restarts in 10 minutes", "description": "Container app is repeatedly restarting."},
		}
	}

	var targets []AlertmanagerAlert
	for i := 1; i <= 14; i++ {
		targets = append(targets, AlertmanagerAlert{
			Labels:      KV{"alertname": "TargetDown", "severity": "warning", "cluster": "pb-nonprod", "job": "redis-exporter", "instance": fmt.Sprintf("10.0.20.%d:9121", i)},
			Annotations: KV{"summary": "Scrape target is down"},
		})
	}

	return []Sample{
		{
			ID: "deployment", Name: "Deployment not ready (kube-state-metrics)",
			Summary: "Exporter is not ready", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: amData(deployLabels, deployAnn, AlertmanagerAlert{Labels: deployLabels, Annotations: deployAnn}),
		},
		{
			ID: "statefulset", Name: "StatefulSet replicas mismatch",
			Summary: "StatefulSet replicas are not ready", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: func() *AlertmanagerData {
				l := merge(ksmScraper, KV{"alertname": "KubeStatefulSetReplicasMismatch", "severity": "warning", "cluster": "pb-nonprod", "exported_namespace": "nonprod-data", "statefulset": "redis-master"})
				a := KV{"summary": "StatefulSet replicas are not ready", "value_used": "Ready replicas: 2"}
				return amData(l, a, AlertmanagerAlert{Labels: l, Annotations: a})
			}(),
		},
		{
			ID: "daemonset", Name: "DaemonSet pods not scheduled",
			Summary: "DaemonSet pods are missing", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: func() *AlertmanagerData {
				l := merge(ksmScraper, KV{"alertname": "KubeDaemonSetNotScheduled", "severity": "warning", "cluster": "pb-nonprod", "exported_namespace": "kube-system", "daemonset": "aws-node"})
				a := KV{"summary": "DaemonSet pods are missing", "description": "Some aws-node pods are not scheduled."}
				return amData(l, a, AlertmanagerAlert{Labels: l, Annotations: a})
			}(),
		},
		{
			ID: "pods-grouped", Name: "Grouped: three crash-looping pods",
			Summary: "Pod is crash looping", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: amData(
				KV{"alertname": "KubePodCrashLooping", "severity": "warning", "cluster": "pb-nonprod", "namespace": "nonprod-apps", "container": "app"},
				KV{"summary": "Pod is crash looping", "description": "Container app is repeatedly restarting."},
				podAlert("api-7f9c-1", "4"), podAlert("api-7f9c-2", "6"), podAlert("worker-5d2b-1", "3"),
			),
		},
		{
			ID: "pvc", Name: "PVC almost full",
			Summary: "Persistent volume is almost full", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: func() *AlertmanagerData {
				l := KV{"alertname": "KubePersistentVolumeFillingUp", "severity": "critical", "cluster": "pb-nonprod", "namespace": "nonprod-data", "persistentvolumeclaim": "data-redis-master-0", "node": "ip-10-0-3-21.ec2.internal"}
				a := KV{"summary": "Persistent volume is almost full", "value_used": "3% free"}
				return amData(l, a, AlertmanagerAlert{Labels: l, Annotations: a})
			}(),
		},
		{
			ID: "node", Name: "Node not ready",
			Summary: "Node is not ready", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: func() *AlertmanagerData {
				l := KV{"alertname": "KubeNodeNotReady", "severity": "critical", "cluster": "pb-nonprod", "node": "ip-10-0-3-21.ec2.internal", "job": "kube-state-metrics"}
				a := KV{"summary": "Node is not ready", "description": "The node is not reporting a Ready condition.", "runbook_url": "https://runbooks.example.com/kube-node-not-ready"}
				return amData(l, a, AlertmanagerAlert{Labels: l, Annotations: a})
			}(),
		},
		{
			ID: "targets-many", Name: "Grouped: 14 scrape targets down (shortened)",
			Summary: "Scrape target is down", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: amData(KV{"alertname": "TargetDown", "severity": "warning", "cluster": "pb-nonprod", "job": "redis-exporter"}, KV{"summary": "Scrape target is down"}, targets...),
		},
		{
			ID: "omitted-upstream", Name: "1 received + 5 left out by Alertmanager (max_alerts)",
			Summary: "Exporter is not ready", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: func() *AlertmanagerData {
				d := amData(deployLabels, deployAnn, AlertmanagerAlert{Labels: deployLabels, Annotations: deployAnn})
				d.TruncatedAlerts = 5
				d.ExternalURL = "https://alertmanager.example.com"
				return d
			}(),
		},
		{
			ID: "long-names", Name: "Grouped: 40 alerts with long names (shortened to fit)",
			Summary: "Pod is not ready", ServiceName: "EKS Infrastructure Nonprod",
			Alertmanager: func() *AlertmanagerData {
				var alerts []AlertmanagerAlert
				for i := 1; i <= 40; i++ {
					pod := fmt.Sprintf("very-long-application-name-for-testing-message-budgets-worker-%02d-7f9c8b6d5f-abcde", i)
					alerts = append(alerts, AlertmanagerAlert{
						Labels: KV{"alertname": "KubePodNotReady", "severity": "warning", "cluster": "pb-nonprod", "namespace": "nonprod-very-long-namespace-name", "pod": pod},
						Annotations: KV{
							"description":   "The application health endpoint is returning HTTP 503.",
							"summary":       "Pod is not ready",
							"state":         "Not ready for longer than expected; readiness probe failing with HTTP 503 from the application health endpoint",
							"dashboard_url": "https://grafana.example.com/d/pod-detail/kubernetes-pod-detail?orgId=1&var-namespace=nonprod-very-long-namespace-name&var-pod=" + pod,
						},
					})
				}
				return amData(KV{"alertname": "KubePodNotReady", "severity": "warning", "cluster": "pb-nonprod", "namespace": "nonprod-very-long-namespace-name"}, KV{"summary": "Pod is not ready"}, alerts...)
			}(),
		},
		{
			ID: "metadata-only", Name: "No full payload stored; flattened Alertmanager labels only",
			Summary: "Exporter is not ready", ServiceName: "EKS Infrastructure Nonprod",
			Meta: merge(deployLabels, deployAnn),
		},
		{
			ID: "generic-metadata", Name: "Generic API alert with unrelated metadata",
			Summary: "Generic API alert", Details: "Raised by the generic API.", ServiceName: "EKS Infrastructure Nonprod",
			Meta: map[string]string{"owner": "infra"}, Source: "generic",
		},
		{
			ID: "manual", Name: "Manual alert (not from Alertmanager)",
			Summary: "Database failover test", Details: "Created manually from the GoAlert UI.", ServiceName: "EKS Infrastructure Nonprod",
		},
	}
}

// FindSample returns the built-in sample with the given ID.
func FindSample(id string) (Sample, bool) {
	for _, s := range Samples() {
		if s.ID == id {
			return s, true
		}
	}
	return Sample{}, false
}
