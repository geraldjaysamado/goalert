package smoke

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/test/smoke/harness"
)

func TestPrometheusAlertManager(t *testing.T) {
	t.Parallel()

	const sql = `
	insert into users (id, name, email)
	values
		({{uuid "user"}}, 'bob', 'joe');

	insert into user_contact_methods (id, user_id, name, type, value)
	values
		({{uuid "cm1"}}, {{uuid "user"}}, 'personal', 'SMS', {{phone "1"}});

	insert into user_notification_rules (user_id, contact_method_id, delay_minutes)
	values
		({{uuid "user"}}, {{uuid "cm1"}}, 0);

	insert into escalation_policies (id, name)
	values
		({{uuid "eid"}}, 'esc policy');

	insert into escalation_policy_steps (id, escalation_policy_id)
	values
		({{uuid "esid"}}, {{uuid "eid"}});

	insert into escalation_policy_actions (escalation_policy_step_id, user_id)
	values
		({{uuid "esid"}}, {{uuid "user"}});

	insert into services (id, escalation_policy_id, name)
	values
		({{uuid "sid"}}, {{uuid "eid"}}, 'service');

	insert into integration_keys (id, type, name, service_id)
	values
		({{uuid "int_key"}}, 'prometheusAlertmanager', 'my key', {{uuid "sid"}});
`
	h := harness.NewHarness(t, sql, "prometheus-alertmanager-integration")
	defer h.Close()

	url := h.URL() + "/api/v2/prometheusalertmanager/incoming?token=" + h.UUID("int_key")

	resp, err := http.Post(url, "application/json", bytes.NewBufferString(`
		{
			"status": "firing",
			"receiver": "alert-name-receiver-1",
			"externalURL": "http://my.url",
			"alerts": [
				{
					"status": "firing",
					"labels": {"alertname": "TestAlert"},
					"annotations": {"summary": "My alert summary", "description": "My description"}
				}
			],
			"commonLabels": {"alertname": "InstanceDown", "instance": "foobar"},
			"commonAnnotations": {"alertname": "InstanceDown", "instance": "foobar"}
		}
		`))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode, "HTTP response code")

	h.Twilio(t).Device(h.Phone("1")).ExpectSMS("InstanceDown")
}

func TestPrometheusAlertManagerGroupKeyDedup(t *testing.T) {
	t.Parallel()

	const sql = `
	insert into escalation_policies (id, name)
	values ({{uuid "eid"}}, 'esc policy');

	insert into services (id, escalation_policy_id, name)
	values ({{uuid "sid"}}, {{uuid "eid"}}, 'service');

	insert into integration_keys (id, type, name, service_id)
	values ({{uuid "int_key"}}, 'prometheusAlertmanager', 'my key', {{uuid "sid"}});
`
	h := harness.NewHarness(t, sql, "prometheus-alertmanager-group-key-dedup")
	defer h.Close()

	url := h.URL() + "/api/v2/prometheusalertmanager/incoming?token=" + h.UUID("int_key")
	post := func(status, groupKey, summary string) {
		t.Helper()
		body := fmt.Sprintf(`{
			"status": %q,
			"receiver": "goalert",
			"groupKey": %q,
			"alerts": [{
				"status": %q,
				"labels": {"alertname": "QueueDown"},
				"annotations": {"summary": %q}
			}],
			"commonLabels": {"alertname": "QueueDown"},
			"commonAnnotations": {"summary": %q}
		}`, status, groupKey, status, summary, summary)
		resp, err := http.Post(url, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "response: %s", data)
	}

	type alertNode struct {
		ID      string
		Summary string
		Status  string
	}
	getAlerts := func() []alertNode {
		t.Helper()
		res := h.GraphQLQuery2(fmt.Sprintf(`query{ alerts(input: {filterByServiceID: ["%s"]}){ nodes { id summary status } } }`, h.UUID("sid")))
		require.Empty(t, res.Errors)
		var data struct {
			Alerts struct{ Nodes []alertNode }
		}
		require.NoError(t, json.Unmarshal(res.Data, &data))
		return data.Alerts.Nodes
	}

	post("firing", "queue-down-app-a", "Queue is down")
	post("firing", "queue-down-app-b", "Queue is down")

	alerts := getAlerts()
	require.Len(t, alerts, 2, "same-summary Alertmanager groups must create distinct alerts")

	for _, a := range alerts {
		assert.Equal(t, "StatusUnacknowledged", a.Status)
	}

	post("resolved", "queue-down-app-a", "Queue is down")

	alerts = getAlerts()
	require.Len(t, alerts, 2)
	var closed, unacked int
	for _, a := range alerts {
		switch a.Status {
		case "StatusClosed":
			closed++
		case "StatusUnacknowledged":
			unacked++
		}
	}
	assert.Equal(t, 1, closed, "resolved notification must close exactly one matching alert")
	assert.Equal(t, 1, unacked, "the other same-summary group must remain active")
}
