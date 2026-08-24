// Package slacktmpl contains the shared data model and helpers used by Slack
// alert title and text templates.
package slacktmpl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	commonTemplates "github.com/prometheus/common/helpers/templates"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// MetadataKey is the alert metadata key used for the Alertmanager template
// payload. The version suffix allows the stored shape to evolve safely.
const MetadataKey = "prometheusAlertmanager.v1"

// AlertmanagerAlert is one alert from an Alertmanager webhook payload.
type AlertmanagerAlert struct {
	Status       string    `json:"status"`
	Labels       KV        `json:"labels"`
	Annotations  KV        `json:"annotations"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	GeneratorURL string    `json:"generatorURL"`
	Fingerprint  string    `json:"fingerprint"`
}

// KV mirrors Alertmanager's label and annotation template type.
type KV map[string]string

// Pair is a name/value pair returned by KV.SortedPairs.
type Pair struct {
	Name  string
	Value string
}

// Pairs is an ordered list of template name/value pairs.
type Pairs []Pair

// Strings is a list of strings with Alertmanager-compatible template helpers.
type Strings []string

// Join joins the strings using sep.
func (s Strings) Join(sep string) string { return strings.Join(s, sep) }

// Names returns the names from a list of pairs.
func (p Pairs) Names() Strings {
	res := make(Strings, len(p))
	for i := range p {
		res[i] = p[i].Name
	}
	return res
}

// Values returns the values from a list of pairs.
func (p Pairs) Values() Strings {
	res := make(Strings, len(p))
	for i := range p {
		res[i] = p[i].Value
	}
	return res
}

func (p Pairs) String() string {
	var result strings.Builder
	for i, pair := range p {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(pair.Name)
		result.WriteByte('=')
		result.WriteString(pair.Value)
	}
	return result.String()
}

// SortedPairs returns label pairs ordered by name, with alertname first.
func (kv KV) SortedPairs() Pairs {
	keys := make([]string, 0, len(kv))
	for key := range kv {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if i := sort.SearchStrings(keys, "alertname"); i < len(keys) && keys[i] == "alertname" {
		copy(keys[1:i+1], keys[:i])
		keys[0] = "alertname"
	}

	pairs := make(Pairs, len(keys))
	for i, key := range keys {
		pairs[i] = Pair{Name: key, Value: kv[key]}
	}
	return pairs
}

// Remove returns a copy without the provided keys.
func (kv KV) Remove(keys []string) KV {
	remove := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		remove[key] = struct{}{}
	}
	res := make(KV, len(kv))
	for key, value := range kv {
		if _, ok := remove[key]; !ok {
			res[key] = value
		}
	}
	return res
}

// Names returns the sorted label names.
func (kv KV) Names() Strings { return kv.SortedPairs().Names() }

// Values returns values ordered by their sorted label names.
func (kv KV) Values() Strings { return kv.SortedPairs().Values() }

func (kv KV) String() string { return kv.SortedPairs().String() }

// Alerts is the list of alerts available to a template.
type Alerts []AlertmanagerAlert

// Firing returns alerts whose Alertmanager status is firing.
func (a Alerts) Firing() Alerts {
	return a.withStatus("firing")
}

// Resolved returns alerts whose Alertmanager status is resolved.
func (a Alerts) Resolved() Alerts {
	return a.withStatus("resolved")
}

func (a Alerts) withStatus(status string) Alerts {
	res := make(Alerts, 0, len(a))
	for _, alert := range a {
		if alert.Status == status {
			res = append(res, alert)
		}
	}
	return res
}

// AlertmanagerData is the template-relevant subset of an Alertmanager webhook
// payload. It intentionally mirrors Alertmanager's notification template data.
type AlertmanagerData struct {
	Receiver           string `json:"receiver"`
	Status             string `json:"status"`
	Alerts             Alerts `json:"alerts"`
	NotificationReason string `json:"notification_reason"`
	GroupLabels        KV     `json:"groupLabels"`
	CommonLabels       KV     `json:"commonLabels"`
	CommonAnnotations  KV     `json:"commonAnnotations"`
	RouteLabels        KV     `json:"routeLabels"`
	ExternalURL        string `json:"externalURL"`
	Version            string `json:"version"`
	GroupKey           string `json:"groupKey"`
	TruncatedAlerts    int    `json:"truncatedAlerts"`
}

// MergeMetadata returns the labels and annotations that are unambiguous for a
// notification. Common annotations take precedence over common labels. When
// there is exactly one alert, its labels and annotations are included as well.
func MergeMetadata(data AlertmanagerData) map[string]string {
	meta := make(map[string]string, len(data.CommonLabels)+len(data.CommonAnnotations))
	if len(data.Alerts) == 1 {
		for key, value := range data.Alerts[0].Labels {
			meta[key] = value
		}
		for key, value := range data.Alerts[0].Annotations {
			meta[key] = value
		}
	}
	for key, value := range data.CommonLabels {
		meta[key] = value
	}
	for key, value := range data.CommonAnnotations {
		meta[key] = value
	}
	return meta
}

// Data is the complete value passed to Slack title and text templates.
type Data struct {
	AlertmanagerData

	GoAlertStatus string
	AlertID       int
	AlertURL      string
	Summary       string
	Details       string
	ServiceID     string
	ServiceName   string
	// Meta is a convenient merged view of alert labels and annotations.
	// For a single alert it includes alert-specific values as well as common
	// values; grouped notifications contain only values common to every alert.
	Meta      KV
	AlertMeta map[string]string
	LogEntry  string
}

func funcs(routeLabels KV) template.FuncMap {
	return template.FuncMap{
		"toUpper":     strings.ToUpper,
		"toLower":     strings.ToLower,
		"title":       func(text string) string { return cases.Title(language.AmericanEnglish).String(text) },
		"trimSpace":   strings.TrimSpace,
		"join":        func(sep string, values []string) string { return strings.Join(values, sep) },
		"match":       regexp.MatchString,
		"safeHtml":    func(text string) string { return text },
		"safeUrl":     func(text string) string { return text },
		"urlUnescape": url.QueryUnescape,
		"reReplaceAll": func(pattern, replacement, text string) string {
			return regexp.MustCompile(pattern).ReplaceAllString(text, replacement)
		},
		"stringSlice": func(values ...string) []string { return values },
		"date":        func(layout string, value time.Time) string { return value.Format(layout) },
		"tz": func(name string, value time.Time) (time.Time, error) {
			location, err := time.LoadLocation(name)
			if err != nil {
				return time.Time{}, err
			}
			return value.In(location), nil
		},
		"now":              time.Now,
		"since":            time.Since,
		"humanizeDuration": commonTemplates.HumanizeDuration,
		"toDate": func(layout, value string) time.Time {
			parsed, _ := time.ParseInLocation(layout, value, time.UTC)
			return parsed
		},
		"mustToDate": func(layout, value string) (time.Time, error) {
			return time.ParseInLocation(layout, value, time.UTC)
		},
		"toJson": func(value any) (string, error) {
			data, err := json.Marshal(value)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		"base64encode": func(text string) string {
			return base64.URLEncoding.EncodeToString([]byte(text))
		},
		"base64decode": func(text string) (string, error) {
			data, err := base64.URLEncoding.DecodeString(text)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		"list": func(values ...any) ([]any, error) {
			if values == nil {
				return []any{}, nil
			}
			return values, nil
		},
		"append": func(slice []any, values ...any) []any {
			return append(slice, values...)
		},
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			result := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				result[key] = values[i+1]
			}
			return result, nil
		},
		"routeLabels": func(name string) string { return routeLabels[name] },
	}
}

func parse(name, value string, routeLabels KV) (*template.Template, error) {
	tmpl, err := template.New(name).Option("missingkey=zero").Funcs(funcs(routeLabels)).Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	return tmpl, nil
}

// Validate checks that a configured template can be parsed.
func Validate(name, value string) error {
	if value == "" {
		return nil
	}
	_, err := parse(name, value, nil)
	return err
}

// Render executes a configured template and trims whitespace around its output.
func Render(name, value string, data Data) (string, error) {
	if value == "" {
		return "", nil
	}
	tmpl, err := parse(name, value, data.RouteLabels)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// DecodeAlertmanagerData decodes Alertmanager template data from alert metadata.
// The boolean result is false when the metadata belongs to another alert source.
func DecodeAlertmanagerData(meta map[string]string) (AlertmanagerData, bool, error) {
	raw, ok := meta[MetadataKey]
	if !ok {
		return AlertmanagerData{}, false, nil
	}

	var data AlertmanagerData
	err := json.Unmarshal([]byte(raw), &data)
	if err != nil {
		return AlertmanagerData{}, true, fmt.Errorf("decode Alertmanager metadata: %w", err)
	}
	return data, true, nil
}
