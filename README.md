# OCP Audit Log Perses Dashboard

A Perses dashboard for viewing OpenShift Kubernetes API audit logs using Loki as the data source. Inspired by [ocp-user-audit-viewer](https://github.com/orenc1/ocp-user-audit-viewer).

## Prerequisites

- OpenShift cluster with [Cluster Observability Operator](https://docs.openshift.com/container-platform/latest/observability/cluster_observability_operator/cluster-observability-operator-overview.html) (Perses)
- [OpenShift Logging](https://docs.openshift.com/container-platform/latest/observability/logging/cluster-logging.html) with LokiStack collecting audit logs

## Architecture

```
User → OpenShift Console (monitoring-console-plugin) → Perses Server → loki-tls-proxy (Nginx) → Loki Gateway
```

The TLS proxy is a workaround for Perses not dynamically reloading certificates when service-serving CAs rotate.

## Deploy

```bash
# Create namespace
oc new-project perses-dev

# Deploy TLS proxy (workaround for Perses ↔ Loki TLS)
oc apply -f deploy/proxy-configmap.yaml
oc apply -f deploy/proxy-deployment.yaml
oc apply -f deploy/proxy-service.yaml

# Deploy datasource and dashboard
oc apply -f deploy/datasource.yaml
oc apply -f deploy/dashboard.yaml
```

## Filters

| Filter | Type | Description |
|--------|------|-------------|
| Username | Free text | Partial match, case-insensitive |
| Exclude System Users | Multi-select dropdown | Deselect to allow specific system users back |
| Verb | Multi-select dropdown | create, update, patch, delete, get, list |
| Resource | Free text | Kubernetes resource type |
| Namespace | Free text | Target namespace |
| Resource Name | Free text | Partial match, case-insensitive |
| Response Code | Dropdown | HTTP status codes |
| Client | Dropdown | User agent type |

## Known Limitations

See [perses/perses#4143](https://github.com/perses/perses/issues/4143) for feature requests:

1. **No column view** — Log fields shown as formatted text, not separate columns
2. **No value mapping** — Can't translate status codes/user agents to friendly labels
3. **No regex in text filters** — Text inputs don't support pattern matching
4. **No dynamic dropdowns from Loki** — No LokiLogQueryVariable plugin
5. **No CSV/Excel export**
6. **Limited result count** — No configurable limit or pagination (Loki default ~100 entries)

## LogQL Query

```logql
{log_type="audit"}
  | json
  | log_source="kubeAPI"
  | user_username!~"${exclude_sa}"
  | user_username=~"(?i).*${username}.*"
  | verb=~"${verb}"
  | objectRef_resource=~".*${resource}.*"
  | objectRef_namespace=~".*${namespace}.*"
  | objectRef_name=~"(?i).*${resource_name}.*"
  | responseStatus_code=~"${response_code}"
  | userAgent=~".*${client}.*"
  | line_format "User={{.user_username}} | Verb={{.verb}} | Namespace={{.objectRef_namespace}} | Resource={{.objectRef_resource}}/{{.objectRef_name}} | Status={{.responseStatus_code}} | Client={{.userAgent}}"
```
