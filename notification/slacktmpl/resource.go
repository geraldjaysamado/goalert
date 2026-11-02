package slacktmpl

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// K8sResource identifies the Kubernetes object an alert is about, derived
// from common Prometheus/kube-state-metrics label conventions. It is a helper
// for templates; it does not decide message layout.
type K8sResource struct {
	Cluster   string
	Namespace string

	// Kind is Deployment, StatefulSet, DaemonSet, Pod, PVC, Node or Target
	// (scrape target), or empty when no resource label is present.
	Kind string
	Name string

	Pod       string
	Container string
	PVC       string
	Node      string
	Job       string
	Instance  string

	// Scraped is true when exported_namespace is present: the plain
	// namespace/pod/container/node/job/instance labels then describe the
	// scraper (e.g. kube-state-metrics), not the affected object.
	Scraped bool
}

// IsWorkload reports whether Kind is a Deployment, StatefulSet or DaemonSet.
func (r K8sResource) IsWorkload() bool {
	switch r.Kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return true
	}
	return false
}

func firstNonBlank(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ResourceFromLabels extracts the affected resource from a label set.
func ResourceFromLabels(l map[string]string) K8sResource {
	r := K8sResource{
		Cluster:   firstNonBlank(l["cluster"], l["cluster_name"], l["k8s_cluster"]),
		Namespace: firstNonBlank(l["exported_namespace"], l["namespace"]),
		PVC:       l["persistentvolumeclaim"],
		Scraped:   strings.TrimSpace(l["exported_namespace"]) != "",
	}
	r.Pod, r.Container, r.Node = l["exported_pod"], l["exported_container"], l["exported_node"]
	if !r.Scraped {
		r.Pod = firstNonBlank(r.Pod, l["pod"])
		r.Container = firstNonBlank(r.Container, l["container"])
		r.Node = firstNonBlank(r.Node, l["node"])
		r.Job = l["job"]
		r.Instance = l["instance"]
	}

	switch {
	case l["deployment"] != "":
		r.Kind, r.Name = "Deployment", l["deployment"]
	case l["statefulset"] != "":
		r.Kind, r.Name = "StatefulSet", l["statefulset"]
	case l["daemonset"] != "":
		r.Kind, r.Name = "DaemonSet", l["daemonset"]
	case r.Pod != "":
		r.Kind, r.Name = "Pod", r.Pod
	case r.PVC != "":
		r.Kind, r.Name = "PVC", r.PVC
	case r.Node != "":
		r.Kind, r.Name = "Node", r.Node
	case r.Job != "" || r.Instance != "":
		r.Kind, r.Name = "Target", firstNonBlank(r.Job, "unknown job")
	}
	return r
}

// SafeURL returns raw if it is an absolute http(s) URL that can be embedded
// in a Slack <url|label> link, otherwise "".
func SafeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2000 || strings.ContainsAny(raw, "<>| \t\r\n") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
}

// Truncate shortens text to at most n runes, ending with "…" when shortened.
func Truncate(n int, text string) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	r := []rune(text)
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// AlertmanagerAlertsURL links to the notification's alert group in the
// Alertmanager UI (filtered by the group labels), or "" when the payload has
// no usable externalURL. Useful for alerts Alertmanager omitted from the
// webhook (truncatedAlerts), which GoAlert never received.
func (d AlertmanagerData) AlertmanagerAlertsURL() string {
	base := SafeURL(d.ExternalURL)
	if base == "" {
		return ""
	}
	keys := make([]string, 0, len(d.GroupLabels))
	for k := range d.GroupLabels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	matchers := make([]string, 0, len(keys))
	for _, k := range keys {
		matchers = append(matchers, k+"="+strconv.Quote(d.GroupLabels[k]))
	}
	link := strings.TrimRight(base, "/") + "/#/alerts"
	if len(matchers) > 0 {
		link += "?filter=" + url.QueryEscape("{"+strings.Join(matchers, ",")+"}")
	}
	if strings.ContainsAny(link, "<>| ") {
		return ""
	}
	return link
}
