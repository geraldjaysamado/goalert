package prometheus

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/target/goalert/alert"
	"github.com/target/goalert/integrationkey"
	"github.com/target/goalert/notification/slacktmpl"
	"github.com/target/goalert/permission"
	"github.com/target/goalert/retry"
	"github.com/target/goalert/util/errutil"
	"github.com/target/goalert/util/log"
	"github.com/target/goalert/validation/validate"
)

/* Example payload

```
{
  "receiver": "goalert",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "InstanceDown",
        "code": "200",
        "instance": "127.0.0.1:9090",
        "job": "prometheus",
        "monitor": "codelab-monitor",
        "severity": "critical"
      },
      "annotations": {
        "details": "127.0.0.1:9090 of job prometheus has been down for more than 1 minute.",
        "summary": "Instance 127.0.0.1:9090 down"
      },
      "startsAt": "2020-08-08T14:32:08.326990857Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://pop-os:9090/graph?g0.expr=promhttp_metric_handler_requests_total+%3E+20\u0026g0.tab=1",
      "fingerprint": "791cec13fcba0368"
    },
    {
      "status": "firing",
      "labels": {
        "alertname": "InstanceDown",
        "code": "200",
        "instance": "localhost:9090",
        "job": "prometheus",
        "monitor": "codelab-monitor",
        "severity": "critical"
      },
      "annotations": {
        "details": "localhost:9090 of job prometheus has been down for more than 1 minute.",
        "summary": "Instance localhost:9090 down"
      },
      "startsAt": "2020-08-08T02:21:08.326990857Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://pop-os:9090/graph?g0.expr=promhttp_metric_handler_requests_total+%3E+20\u0026g0.tab=1",
      "fingerprint": "8df98227bdd81384"
    }
  ],
  "groupLabels": {},
  "commonLabels": {
    "alertname": "InstanceDown",
    "code": "200",
    "job": "prometheus",
    "monitor": "codelab-monitor",
    "severity": "critical"
  },
  "commonAnnotations": {},
  "externalURL": "http://pop-os:9093",
  "version": "4",
  "groupKey": "{}:{}",
  "truncatedAlerts": 0
}
```
*/

type postBody slacktmpl.AlertmanagerData

func alertSummary(a slacktmpl.AlertmanagerAlert) string {
	if a.Annotations["summary"] != "" {
		return a.Annotations["summary"]
	}

	return a.Labels["alertname"] + " " + a.Labels["instance"]
}
func alertGeneratorLink(a slacktmpl.AlertmanagerAlert) string {
	if a.GeneratorURL == "" {
		return ""
	}

	return fmt.Sprintf(" [View](%s)", a.GeneratorURL)
}
func alertDetails(a slacktmpl.AlertmanagerAlert) string {
	if a.Annotations["details"] != "" {
		return a.Annotations["details"] + alertGeneratorLink(a)
	}

	return alertSummary(a) + alertGeneratorLink(a)
}
func (b postBody) Summary() string {
	if b.CommonAnnotations["summary"] != "" {
		return b.CommonAnnotations["summary"]
	}
	if b.CommonLabels["alertname"] == "" {
		if len(b.Alerts) == 0 {
			return "Prometheus Alertmanager alert"
		}
		// different alerts
		return alertSummary(b.Alerts[0]) + fmt.Sprintf(" and %d others", len(b.Alerts)-1)
	}

	// we have a common alert name
	if b.CommonLabels["instance"] != "" {
		return b.CommonLabels["alertname"] + " " + b.CommonLabels["instance"]
	}

	var instances []string
	for _, a := range b.Alerts {
		instances = append(instances, a.Labels["instance"])
	}

	return b.CommonLabels["alertname"] + " " + strings.Join(instances, ",")
}

func (b postBody) DedupKey() string {
	if b.GroupKey == "" {
		// Preserve compatibility with webhook payloads that predate groupKey or
		// are produced by Alertmanager-compatible systems that omit it.
		return b.Summary()
	}

	// Alertmanager's groupKey is stable for firing and resolved notifications.
	// Hash it so unusually large label sets cannot be truncated by GoAlert's
	// user-provided dedup key limit and accidentally collide.
	return fmt.Sprintf("prometheus-alertmanager:%x", sha256.Sum256([]byte(b.GroupKey)))
}

func (b postBody) Details(payload string) string {
	var s strings.Builder
	if b.ExternalURL != "" {
		fmt.Fprintf(&s, "[Prometheus Alertmanager UI](%s)\n\n", b.ExternalURL)
	}
	if b.CommonAnnotations["details"] != "" {
		s.WriteString(b.CommonAnnotations["details"] + "\n\n")
	} else {
		for _, a := range b.Alerts {
			s.WriteString(alertDetails(a) + "\n\n")
		}
	}
	if payload != "" {
		fmt.Fprintf(&s, "## Payload\n\n```json\n%s\n```\n", payload)
	}
	return s.String()
}

func (b postBody) Metadata() (map[string]string, error) {
	alertmanagerData := slacktmpl.AlertmanagerData(b)
	data, err := json.Marshal(alertmanagerData)
	if err != nil {
		return nil, err
	}
	meta := slacktmpl.MergeMetadata(alertmanagerData)
	meta[slacktmpl.MetadataKey] = string(data)
	return meta, nil
}

func clientError(w http.ResponseWriter, code int, err error) bool {
	if err == nil {
		return false
	}

	http.Error(w, http.StatusText(code), code)
	return true
}

func PrometheusAlertmanagerEventsAPI(aDB *alert.Store, intDB *integrationkey.Store) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		err := permission.LimitCheckAny(ctx, permission.Service)
		if errutil.HTTPError(ctx, w, err) {
			return
		}
		serviceID := permission.ServiceID(ctx)

		var body postBody
		var buf bytes.Buffer
		err = json.NewDecoder(io.TeeReader(r.Body, &buf)).Decode(&body)
		if clientError(w, http.StatusBadRequest, err) {
			log.Logf(ctx, "bad request from prometheus alertmanager: %v", err)
			return
		}

		var status alert.Status
		switch body.Status {
		case "firing":
			status = alert.StatusTriggered
		case "resolved":
			status = alert.StatusClosed
		default:
			log.Logf(ctx, "bad request from prometheus alertmanager: missing or invalid state")
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		buf.Reset()
		err = json.Indent(&buf, data, "", "  ")
		if err == nil {
			data = buf.Bytes()
		}

		summary := validate.SanitizeText(body.Summary(), alert.MaxSummaryLength)
		msg := &alert.Alert{
			Summary:   summary,
			Details:   validate.SanitizeText(body.Details(string(data)), alert.MaxDetailsLength),
			Status:    status,
			Source:    alert.SourcePrometheusAlertmanager,
			ServiceID: serviceID,
			Dedup:     alert.NewUserDedup(body.DedupKey()),
		}
		meta, metaErr := body.Metadata()
		if metaErr != nil {
			log.Log(ctx, errors.Wrap(metaErr, "encode prometheus alertmanager metadata"))
			meta = nil
		} else if metaErr = alert.ValidateMetadata(meta); metaErr != nil {
			// Prefer keeping the flattened labels and annotations if only the full
			// Alertmanager template payload pushes metadata over GoAlert's limit.
			delete(meta, slacktmpl.MetadataKey)
			if metaErr = alert.ValidateMetadata(meta); metaErr != nil {
				// Metadata is supplemental. Preserve existing ingestion behavior for
				// unusually large label or annotation sets instead of rejecting the alert.
				log.Log(ctx, errors.Wrap(metaErr, "omit prometheus alertmanager metadata"))
				meta = nil
			}
		}

		err = retry.DoTemporaryError(func(int) error {
			_, _, err = aDB.CreateOrUpdateWithMeta(ctx, msg, meta)
			return err
		},
			retry.Log(ctx),
			retry.Limit(10),
			retry.FibBackoff(time.Second),
		)
		if errutil.HTTPError(ctx, w, errors.Wrap(err, "create or update alert for prometheus alertmanager")) {
			return
		}
	}
}
