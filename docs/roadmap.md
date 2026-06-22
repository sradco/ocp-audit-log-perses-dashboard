# Roadmap: Planned Improvements

Features to implement once upstream [Perses](https://github.com/perses/perses) adds the required support.
Tracking upstream: [perses/perses#4143](https://github.com/perses/perses/issues/4143)

## Current Limitations

| # | Limitation | Impact |
|---|-----------|--------|
| 1 | No columnar display | Log fields shown as formatted text, not separate table columns |
| 2 | No client filter | User agent strings too complex for static dropdown matching |
| 3 | No value mapping | Can't translate status codes or user agents to friendly labels |
| 4 | No dynamic dropdowns from Loki | Filters like Username and Namespace require free text |
| 5 | No CSV/Excel export | No way to export filtered audit events |
| 6 | Limited result count | No configurable limit or pagination (Loki default ~100 entries) |
| 7 | No regex mode for text variables | Text inputs don't indicate regex support to users |

## Planned Features

### 1. Columnar Table Display

**Priority:** High — biggest UX gap vs. the original dashboard.

**Blocked on:** LogsTable column extraction from structured logs.

Once available, define columns mapping extracted JSON fields to table headers:

```yaml
kind: LogsTable
spec:
  enableDetails: true
  showTime: true
  columns:
    - field: "user_username"
      header: "User"
    - field: "verb"
      header: "Verb"
    - field: "objectRef_namespace"
      header: "Namespace"
    - field: "objectRef_resource"
      header: "Resource"
    - field: "objectRef_name"
      header: "Resource Name"
    - field: "responseStatus_code"
      header: "Status"
    - field: "userAgent"
      header: "Client"
```

**Action:** Remove `| line_format` from the query and let Perses render fields as columns.

---

### 2. Client (User Agent) Filter

**Priority:** High — frequently needed for isolating human vs. automated actions.

**Blocked on:** Value mapping support in variables or regex-aware dropdown matching.

The raw `userAgent` field contains strings like `oc/4.21.0 (linux/amd64) kubernetes/0c09391` that don't match cleanly with static values. We need Perses to translate partial matches to friendly labels:

| Raw value (contains) | Display label |
|-----------------------|---------------|
| `oc/` | oc CLI |
| `kubectl/` | kubectl |
| `Mozilla` | Browser |
| `openshift-console` | OpenShift Console |
| `Prometheus` | Prometheus |

**Action:** Add a Client dropdown variable with value mappings, and add `| userAgent=~".*${client}.*"` back to the query.

---

### 3. Value Mapping for Status Codes

**Priority:** Medium — improves readability.

**Blocked on:** Value mapping / display overrides for column values.

Map numeric codes to HTTP reason phrases:

| Code | Display |
|------|---------|
| 200 | 200 OK |
| 201 | 201 Created |
| 204 | 204 No Content |
| 400 | 400 Bad Request |
| 401 | 401 Unauthorized |
| 403 | 403 Forbidden |
| 404 | 404 Not Found |
| 409 | 409 Conflict |
| 422 | 422 Unprocessable Entity |
| 500 | 500 Internal Server Error |

---

### 4. Dynamic Dropdowns from Loki

**Priority:** Medium — reduces manual input errors.

**Blocked on:** `LokiLogQueryVariable` plugin (similar to Grafana's `label_values()`).

Once available, populate filters dynamically:

- **Username** — `{log_type="audit"} | json | line_format "{{.user_username}}" | dedup`
- **Namespace** — from `objectRef_namespace` distinct values
- **Resource** — from `objectRef_resource` distinct values
- **Exclude System Users** — multi-select populated from actual `system:*` users seen in logs

---

### 5. CSV/Excel Export

**Priority:** Medium — audit events often need to be shared or archived.

**Blocked on:** Panel-level export action in Perses.

**Action:** Add export button to the LogsTable panel supporting CSV and JSON formats.

---

### 6. Configurable Result Limit / Pagination

**Priority:** Medium — currently limited to ~100 entries.

**Blocked on:** Configurable `limit` parameter in LogQuery and pagination controls in LogsTable.

**Action:** Set default limit to 1000 and enable pagination for browsing large result sets.

---

### 7. Regex Mode for Text Variables

**Priority:** Low — works today via passthrough but not user-friendly.

**Blocked on:** Text variable with explicit regex mode indicator or validation.

Currently users can type regex in text filters (e.g. `sradco|ocohen`) and it works because the value is injected into `=~"(?i).*${var}.*"`. But there's no UI indicator that regex is supported.

---

## References

- Upstream feature request: [perses/perses#4143](https://github.com/perses/perses/issues/4143)
- Local tracking issue: [#1](https://github.com/sradco/ocp-audit-log-perses-dashboard/issues/1)
- Original dashboard: [ocp-user-audit-viewer](https://github.com/orenc1/ocp-user-audit-viewer)
