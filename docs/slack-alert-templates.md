# Slack Alert Template Library

Named Slack templates that can be assigned to individual services. Services without an assignment keep the existing formatting (the shared `Slack.RichAlerts`, `Slack.TitleTemplate` and `Slack.TextTemplate` settings, or GoAlert's built-in message).

## How it works

```text
Alertmanager ──webhook──▶ GoAlert integration key ──▶ alert (+ Alertmanager data in alert metadata)
                                                         │
                         escalation ──▶ Slack sender ────┤ is a template assigned to this service ID?
                                                         ├─ yes → render that library template
                                                         │        (render error → log + use existing formatting)
                                                         └─ no  → existing formatting (unchanged)
```

- **Storage:** GoAlert's existing database-backed config, in two keys:
  - `Slack.AlertTemplateLibrary`: the templates as JSON (`id`, `name`, `title`, `text`).
  - `Slack.ServiceAlertTemplates`: `serviceID=templateID` lines.

  Both survive restarts and are editable without rebuilding the image. Do not edit them by hand. They are hidden on **Admin → Config**.

- **IDs:** every template has a generated UUID. Assignments use the service UUID and the template UUID, never names, Slack channels or integration keys. Renaming a template does not affect assignments.
- **Concurrent edits.** Every create, update, delete and assignment is a single config update. Each update locks the config table, re-reads the latest stored config inside the transaction, changes one item, validates and saves.

  | Concurrent operations                                   | Result                                                                                                                                                                                                                                                                                                          |
  | ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
  | Different templates, or different services' assignments | Both kept. Each change applies to the latest config.                                                                                                                                                                                                                                                            |
  | Two editors save the **same template**                  | The second save is rejected. Each template has a `revision` (a hash of its name, title and body), and updates must send the `expectedRevision` they loaded. The editor stays open with the second admin's draft, explains the conflict, and offers **Load latest version**. The first admin's changes are kept. |
  | Edit while another admin deletes the template           | The edit fails with "template no longer exists".                                                                                                                                                                                                                                                                |
  | Delete while another admin assigns the template         | Whichever commits second is validated against the first. Deleting an assigned template, or assigning a deleted one, is rejected.                                                                                                                                                                                |
  | Two admins set the **same service's** template          | Last write wins. Both are explicit choices for the same field, so there is no revision check here.                                                                                                                                                                                                              |

- **Unchanged:** Alertmanager ingestion and deduplication, escalation, Acknowledge/Close buttons, thread replies and status updates, dashboard/runbook actions. Assigned templates also show a blue status bar for unacknowledged INFO alerts; acknowledged stays amber and resolved stays green. Unassigned services and rendering fallbacks keep the existing colours. The preview uses the delivery colour.

The Infrastructure — Clear example uses the native dashboard button without a duplicate Link line. The button defaults to **Grafana Dashboard**; an explicit dashboard_label annotation still overrides it. Resource-specific links remain beside affected resources when a group has different dashboard URLs.

## Using it

1. **Admin → Slack Templates** → **New template** or **Start from an example**.
2. Edit the name, title and body. The preview renders against built-in samples (or a pasted Alertmanager webhook JSON) as Firing, Acknowledged or Resolved. Nothing is sent.
3. Open a service → **Edit** → **Slack template** → choose a template. The default is **Use existing/default formatting**.
4. **Duplicate** copies a template under a new name. **Delete** is blocked while any service uses the template; the dialog lists those services.

### Permissions

All template and assignment operations require the **admin** role. The same rule already applies to editing GoAlert config. Non-admins don't see the Slack template field on the service Edit dialog, and the API rejects their requests (`access denied`).

Assigning any template requires `Slack.SigningSecret`, because the link buttons use the Slack interaction endpoint.

### Validation and failure behavior

| When          | Check                                                                                              | Result                                                                                                            |
| ------------- | -------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Save template | name required, unique (case-insensitive), ≤100 chars; title/body ≤16 KiB each; template must parse | rejected with a field error                                                                                       |
| Save config   | every assignment points to an existing template                                                    | rejected, which is what blocks deleting an assigned template                                                      |
| Send          | assigned template missing, or render fails at runtime                                              | **error logged** (`ServiceID`, `SlackTemplateID`, `SlackTemplateName`); message sent with the existing formatting |
| Send          | body longer than Slack's 3000-character limit                                                      | see _Message length_ below                                                                                        |

There is no fixed limit on the number of templates. The whole library is capped at 1 MiB.

### Message length

Slack accepts 3000 characters per body section. For assigned library templates:

1. GoAlert renders with `.MaxListedAlerts` set to the number of received alerts.
2. If the body is too long, GoAlert finds the largest `.MaxListedAlerts` that fits (binary search) and renders again. A template that lists alerts with `firstN .MaxListedAlerts .Alerts` therefore drops whole entries and prints an accurate "…and N more" notice and links.
3. Only if the body still doesn't fit (for example, one enormous line) does GoAlert keep whole lines up to the limit. It never cuts inside a `<url|label>` link, and it appends "_…message shortened to fit Slack's limit._". The preview flags this case.

The preview calls the same function, so it shows exactly what will be sent. Unassigned services keep the existing behaviour: the shared templates are cut at 3000 characters, as before.

### Alerts GoAlert didn't fully receive

| Situation                                                         | What GoAlert has                                                                                                                          | `.AlertmanagerDataState` |
| ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| Normal                                                            | full payload: `.Alerts` and so on                                                                                                         | `complete`               |
| Alertmanager `max_alerts` exceeded                                | full payload for the received alerts; `.TruncatedAlerts` = how many were **left out upstream** (GoAlert never saw them)                   | `complete`               |
| Stored payload present but can't be decoded                       | `.AlertMeta` (flattened labels and annotations, if stored)                                                                                | `unreadable`             |
| Alertmanager alert (by stored source) without a payload           | `.AlertMeta`: one alert's values, or for a group only the values **common to every alert**. A mixed-name group has no common `alertname`. | `metadata-only`          |
| Manual, generic API, email, etc. (any metadata, even `alertname`) | `.Summary`, `.Details`, `.AlertMeta`                                                                                                      | `none`                   |

Except for `complete`, `.Alerts` is a single placeholder, so its length means nothing.

**How Alertmanager alerts are identified:** by the alert's stored `source` (`prometheusAlertmanager`), which the engine passes with every Slack alert and status message (`.Source` in templates). Every stored alert has a source, so alerts created before this change are classified correctly too. No new metadata is written and ingestion is unchanged. Only if the source is unknown (empty, which no current GoAlert code path produces) does an `alertname` label act as a hint.

**Remaining ambiguity:** for a `metadata-only` alert, GoAlert doesn't record _why_ the payload is missing. Ingestion drops it when the metadata would exceed the 32 KiB limit, and alerts created before payload storage never had one. So templates shouldn't claim a cause. The example says "Per-alert details are not available for this alert" or "The stored alert payload could not be read".

Ingestion and deduplication are unchanged.

## Template data and helpers

Templates receive the same data as the shared templates: `.Status`, `.Alerts` (each with `.Labels`, `.Annotations`, `.Status`, `.StartsAt`), `.CommonLabels`, `.CommonAnnotations`, `.GroupLabels`, `.TruncatedAlerts`, `.ExternalURL`, `.Meta`, `.GoAlertStatus` (`unacknowledged`/`acknowledged`/`closed`), `.AlertID`, `.AlertURL`, `.Summary`, `.Details`, `.ServiceID`, `.ServiceName`, `.AlertMeta`, `.AlertmanagerDataState`, `.MaxListedAlerts`, `.AlertmanagerAlertsURL`.

New helpers, available to all Slack templates:

| Helper                   | Example                                                 | Purpose                                                                                                                                                                            |
| ------------------------ | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `coalesce`               | `coalesce .Labels.exported_namespace .Labels.namespace` | first non-blank value                                                                                                                                                              |
| `firstN`                 | `range firstN 10 .Alerts`                               | limit a grouped list                                                                                                                                                               |
| `add`, `sub`             | `sub (len .Alerts) 10`                                  | counts for "…and N more"                                                                                                                                                           |
| `slackEscape`            | `slackEscape .Labels.pod`                               | escape `&`, `<`, `>` for Slack mrkdwn                                                                                                                                              |
| `truncate`               | `truncate 150 .Annotations.state`                       | at most N characters, ending in "…"                                                                                                                                                |
| `safeURL`                | `with safeURL .Annotations.dashboard_url`               | the URL only if it is absolute http(s) and safe inside `<url\|label>`; otherwise empty                                                                                             |
| `k8sResource`            | `$r := k8sResource .Labels`                             | affected Kubernetes resource: `.Cluster .Namespace .Kind .Name .Pod .Container .PVC .Node .Job .Instance .Scraped .IsWorkload`. Uses `exported_*` labels when present (see below). |
| `.AlertmanagerAlertsURL` | `with .AlertmanagerAlertsURL`                           | Alertmanager UI link for the group (externalURL plus group-label filter), empty unless externalURL is a valid http(s) URL                                                          |

The title is plain text. GoAlert escapes it and links it to the alert.

## Example: Infrastructure — Clear

Source: `notification/slacktmpl/examples/infrastructure-clear.{title,text}.tmpl`. This is a starting point only. It is not assigned to anything.

```text
CRITICAL — Exporter is not ready
Cluster: pb-nonprod
Namespace: nonprod-exporters
Workload: pb-devtest-redis-exporter (Deployment)
State: 0 of 1 replicas available
Description: Deployment nonprod-exporters/pb-devtest-redis-exporter has no available replicas.
[Grafana Dashboard button]
```

What it shows, and from where (lines appear only when the value exists):

| Line       | Source                                                                                                                                                                                                                       |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Title      | `RESOLVED` when closed, else upper-cased `severity` (or `ALERT`), then `summary` → `alertname` → GoAlert summary; `(N alerts)` when received + left-out alerts > 1 (no count in `metadata-only` mode, because it is unknown) |
| Cluster    | `cluster` / `cluster_name` / `k8s_cluster`                                                                                                                                                                                   |
| Namespace  | `exported_namespace`, else `namespace`                                                                                                                                                                                       |
| Workload   | `deployment` / `statefulset` / `daemonset`                                                                                                                                                                                   |
| Pod        | `pod` (+ `container`)                                                                                                                                                                                                        |
| PVC / Node | `persistentvolumeclaim` / `node`                                                                                                                                                                                             |
| Target     | `job` (+ `instance`), only when no resource label above is present                                                                                                                                                           |
| State      | annotation `state`, else `value_used` (`Last reported state` when resolved)                                                                                                                                                  |
| Description | annotation `description` (omitted when it repeats `summary` or the state)                                                                                                                                                                       |
| Dashboard button | annotation `dashboard_url` (+ `dashboard_label`)                                                                                                                                                                             |

**kube-state-metrics:** when `exported_namespace` is present, the plain `namespace`, `pod`, `container`, `node`, `instance` and `job` labels describe the kube-state-metrics scraper, not the affected object. The template then uses only the `exported_*` values for the affected resource.

**Layouts:**

- **One alert received and none left out:** the detailed lines above.
- **Grouped, or any left out upstream:** "_Affected resources (total; N received)_", then one line per received alert: namespace / kind name — state, plus its own summary or dashboard link if it differs from the common one. A different description appears directly beneath that resource; a shared description appears once below the list. As many lines are listed as fit. Then, as applicable:
  - "…and N more received alerts not shown (Slack length limit)" with a GoAlert alert link.
  - "Alertmanager left out N more alerts… GoAlert has no details for them", with **View the group in Alertmanager** only when `externalURL` is valid. That URL comes from Alertmanager's `--web.external-url`; if it is an in-cluster address, Slack users may not be able to open it.
- **`metadata-only` / `unreadable`:** a neutral note (no cause claimed) that only the stored labels are shown and that a grouped notification may cover more than one resource, then the detailed lines built from `.AlertMeta`, then a GoAlert link.
- **`none` (ordinary alerts):** the alert details appear as **Description**, plus resource lines when its metadata has resource labels. No caveat note.

**Description:** write a short, factual explanation of the condition. Avoid repeating the title or state. Missing descriptions are omitted. The example does not display `next_check` or a generic runbook instruction; valid runbook buttons are still available.

Updating this built-in example does not rewrite templates already saved in the database. Existing saved templates must be updated separately; no service assignment is changed automatically.

**Not inferred:** replica counts, durations and healthy values are never computed or assumed. They appear only if an annotation provides them.

### Recommended annotations for clearer messages

```yaml
annotations:
  summary: Exporter is not ready # readable title (instead of the rule name)
  state: >- # concrete current state
    {{ $value }} available (desired: see dashboard)
  description: Deployment has no available replicas. # concise explanation of the condition
  dashboard_url: https://grafana.example/d/<uid>?var-namespace={{ $labels.exported_namespace }}&var-workload={{ $labels.deployment }}
  dashboard_label: Grafana Dashboard
  runbook_url: https://runbooks.example/exporter-not-ready
```

- To produce "0 of 1 replicas ready", the rule must give both numbers. You can put them in the alert expression output, or use the Prometheus template `query` function:
  ```yaml
  state: >-
    {{ $value }} of {{ with printf `kube_deployment_spec_replicas{exported_namespace="%s",deployment="%s"}` $labels.exported_namespace $labels.deployment | query }}{{ . | first | value }}{{ end }} replicas available
  ```
  Confirm that the ruler you use supports `query` in annotations before relying on it.
- If you want the duration shown, write it into the annotation yourself (for example "for 5 minutes", matching the rule's `for:`). Alertmanager does not send the `for:` value.

## Rollback

- **Per service:** set **Slack template** back to **Use existing/default formatting**.
- **Feature:** remove all assignments. Rendering then goes back to the existing path.

### Running an older image against this config (downgrade)

Checked against commit `2458b9d` (19 Aug 2026, "feat(escalation): add multi-ack feature to escalation policy steps", the base before the Slack template work). The same code is unchanged at `8347674`. Other versions were not checked.

- **Loading:** `config.Store` decodes with `json.Unmarshal`, so the old binary ignores `Slack.AlertTemplateLibrary` and `Slack.ServiceAlertTemplates`. All services render with the existing formatting.
- **Normal updates are safe.** Admin Config **Save** (`setConfig`) and other `UpdateConfig` callers keep the keys. `updateConfigTx` calls `jsonutil.Apply(cfg.data, newCfg)`, which merges the old struct's fields into the stored raw JSON, recursing into nested objects such as `Slack`, so unknown keys survive.
- **Full replacement drops them.** `goalert set-config` and an HTTP `PUT` to the config endpoint both call `SetConfigData` with the supplied document. They replace the stored JSON wholesale, so the keys survive only if the imported file contains them. `goalert get-config` and the config download export the raw stored JSON, so a fresh export includes the keys. An older or hand-made file may not.
- This was verified by reading the code. No downgrade was performed against a live deployment.
