# OCP Audit Log Perses Dashboard

A Perses dashboard for viewing OpenShift Kubernetes API audit logs using Loki as the data source. Inspired by [ocp-user-audit-viewer](https://github.com/orenc1/ocp-user-audit-viewer).

## Prerequisites

- OpenShift cluster with [Cluster Observability Operator](https://docs.openshift.com/container-platform/latest/observability/cluster_observability_operator/cluster-observability-operator-overview.html) (Perses)
- [OpenShift Logging](https://docs.openshift.com/container-platform/latest/observability/logging/cluster-logging.html) with LokiStack collecting audit logs
- Recommended: [Audit log filtering](docs/loki-audit-filter.md) to reduce log volume before collection

## Deploy

```bash
# Create namespace (or use an existing one)
oc new-project perses-dev

# Deploy datasource and dashboard
oc apply -f deploy/datasource.yaml
oc apply -f deploy/dashboard.yaml
```

The datasource connects directly to the Loki gateway at:
`https://logging-loki-gateway-http.openshift-logging.svc.cluster.local:8080/api/logs/v1/audit`

## Filters

| Filter | Type | Description |
|--------|------|-------------|
| Username | Free text | Partial match, case-insensitive. Supports regex (e.g. `sradco\|ocohen`) |
| Hide Unauthenticated | Dropdown (Yes/No) | Exclude events with no user identity (default: Yes) |
| Exclude System Users | Multi-select dropdown | Deselect to allow specific system users back |
| Exclude Users (regex) | Free text | Exclude additional users by regex (e.g. `bot-.*\|ci-runner.*\|system:hive.*`) |
| Verb | Multi-select dropdown | create, update, patch, delete, get, list |
| Resource | Free text | Kubernetes resource type |
| Namespace | Free text | Target namespace |
| Resource Name | Free text | Partial match, case-insensitive |
| Response Code | Dropdown | HTTP status codes |

## Known Limitations

1. **No columnar display** — log fields shown as formatted text, not separate table columns
2. **No client filter** — user agent strings too complex for static dropdown matching
3. **No value mapping** — can't translate status codes/user agents to friendly labels
4. **No dynamic dropdowns** — no Loki-based variable plugin for populating filters from live data
5. **No CSV/Excel export**
6. **Limited result count** — no configurable limit or pagination (Loki default ~100 entries)

See [docs/roadmap.md](docs/roadmap.md) for detailed plans and implementation notes.
Upstream feature request: [perses/perses#4143](https://github.com/perses/perses/issues/4143)

## Go SDK

The dashboard can also be built programmatically using the [Perses Go SDK](go-sdk/main.go):

```bash
cd go-sdk
go run . > ../deploy/dashboard.json
```

## Docs

- [Recommended Loki audit log filtering](docs/loki-audit-filter.md) — ClusterLogForwarder filters to reduce volume by ~95%
- [Roadmap](docs/roadmap.md) — planned improvements blocked on upstream Perses features

## LogQL Query

```logql
{log_type="audit"}
  | json
  | log_source="kubeAPI"
  | user_username=~"${hide_unauth}"
  | user_username!~"${exclude_sa}|${exclude_custom}"
  | user_username=~"(?i).*${username}.*"
  | verb=~"${verb}"
  | objectRef_resource=~".*${resource}.*"
  | objectRef_namespace=~".*${namespace}.*"
  | objectRef_name=~"(?i).*${resource_name}.*"
  | responseStatus_code=~"${response_code}"
  | line_format "User={{.user_username}} | Verb={{.verb}} | Namespace={{.objectRef_namespace}} | Resource={{.objectRef_resource}} | Resource Name={{.objectRef_name}} | Status={{.responseStatus_code}} | Client={{.userAgent}}"
```
