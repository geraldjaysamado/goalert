package slacktmpl

import (
	"embed"
	"strings"
)

//go:embed examples/*.tmpl
var exampleFS embed.FS

// Example is a built-in starting point for a library template. Examples are
// never applied automatically; an admin must create a template from one and
// assign it to a service.
type Example struct {
	Key         string
	Name        string
	Description string
	Title       string
	Text        string
}

func mustExampleFile(name string) string {
	data, err := exampleFS.ReadFile("examples/" + name)
	if err != nil {
		panic(err)
	}
	// Normalize line endings so a Windows checkout (core.autocrlf) renders the
	// same output as Linux builds.
	return strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
}

// Examples returns the built-in example templates.
func Examples() []Example {
	return []Example{{
		Key:         "infrastructure-clear",
		Name:        "Infrastructure — Clear",
		Description: "Readable Kubernetes infrastructure alerts: severity, summary, a concise description, cluster, namespace and the affected Deployment/StatefulSet/DaemonSet, pod, PVC, node or scrape target. Lists every resource in grouped notifications.",
		Title:       mustExampleFile("infrastructure-clear.title.tmpl"),
		Text:        mustExampleFile("infrastructure-clear.text.tmpl"),
	}}
}
