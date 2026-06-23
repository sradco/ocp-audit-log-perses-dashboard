# Performance Tuning for Audit Log Queries

The dashboard's LogQL query uses `| json` to parse every matching audit log line,
which is the most expensive operation. This document describes how to improve
query performance by leveraging Loki's indexing features.

## Current Query Cost

**ViaQ format:**
```
{log_type="audit"} | json | log_source="kubeAPI" | verb=~"create" | ...
```

**OTLP format (current improvement):**
```
{log_type="audit", openshift_log_source="kubeAPI"} | json | verb=~"create" | ...
```

With ViaQ, Loki must:
1. Select all audit log streams (only `log_type` is indexed)
2. Decompress and parse the full JSON of **every** log line
3. Apply label filters after parsing

With OTLP, `openshift_log_source` is a stream label, so step 1 already excludes
non-kubeAPI audit logs (e.g. `auditd`) at the index level. However, the audit
event fields still require `| json` extraction.

## Option 1: OTLP + Schema v13 + Structured Metadata (Recommended — Future)

> **Status: Not yet available.** As of June 2026, switching to OTLP adds
> `openshift_log_source` as a stream label but does **not** automatically store
> audit event fields as structured metadata. The fields below require upstream
> changes to the Cluster Logging Operator to map them as OTLP attributes.

With OpenShift Logging 6.x using the OTLP data model and LokiStack schema v13,
audit fields **could be** stored as structured metadata if the upstream mapping
is implemented:

| Dashboard Filter | OTLP Attribute (proposed) | Current Status |
|-----------------|--------------------------|----------------|
| Username | `k8s.user.username` | **not mapped yet** |
| Verb | `k8s.audit.event.verb` | **not mapped yet** |
| Resource | `k8s.audit.event.object_ref.resource` | **not mapped yet** |
| Namespace | `k8s.audit.event.object_ref.namespace` | **not mapped yet** |
| Resource Name | `k8s.audit.event.object_ref.name` | **not mapped yet** |
| Response Code | `k8s.audit.event.response.code` | **not mapped yet** |
| Client | `k8s.audit.event.user_agent` | **not mapped yet** |

Once these fields are available as structured metadata, they would be queryable
without `| json` parsing, significantly reducing query cost.

### Upstream Issues

We've opened issues to track this work:
- **Loki Operator** (add verb as stream label): https://github.com/grafana/loki/issues/22513
- **Cluster Logging Operator** (audit field OTLP mapping): https://github.com/openshift/cluster-logging-operator/issues/3317

### What OTLP gives you today

Switching to OTLP currently provides:
- `openshift_log_source` as a **stream label** — filters `kubeAPI` vs `auditd` at index level
- Eliminates the need for `| json | log_source="kubeAPI"` post-filter
- Modest performance improvement for clusters with mixed audit sources

### Enable schema v13

```yaml
apiVersion: loki.grafana.com/v1
kind: LokiStack
metadata:
  name: logging-loki
  namespace: openshift-logging
spec:
  storage:
    schemas:
      - effectiveDate: "2026-07-01"
        version: v13
```

### Add verb as a custom stream label (optional)

If you want `verb` indexed at the stream level (low cardinality — only ~6 values):

```yaml
apiVersion: loki.grafana.com/v1
kind: LokiStack
metadata:
  name: logging-loki
  namespace: openshift-logging
spec:
  limits:
    tenants:
      audit:
        otlp:
          streamLabels:
            logAttributes:
              - k8s.audit.event.verb
```

> **Note:** This only works once the upstream mapping of `verb` to an OTLP
> attribute is implemented.

## Option 2: ViaQ Model — labelKeys

For clusters using the ViaQ data model (OpenShift Logging < 6.x or not using OTLP),
configure `labelKeys` in the `ClusterLogForwarder`:

```yaml
apiVersion: observability.openshift.io/v1
kind: ClusterLogForwarder
metadata:
  name: instance
  namespace: openshift-logging
spec:
  outputs:
    - name: lokistack-out
      type: lokiStack
      lokiStack:
        target:
          name: logging-loki
          namespace: openshift-logging
        labelKeys:
          audit:
            labelKeys:
              - verb
```

We've opened an upstream issue to make this a default:
- **Cluster Logging Operator**: https://github.com/openshift/cluster-logging-operator/issues/3317

## Cardinality Guidelines

Only add **low-cardinality** fields as stream labels:

| Field | Cardinality | Safe as Stream Label? |
|-------|-------------|----------------------|
| verb | ~6 values | **Yes** |
| responseStatus.code | ~12 values | Yes (borderline) |
| objectRef.resource | ~50-100 | No — too many streams |
| user.username | High | No — use structured metadata |
| objectRef.namespace | High | No — use structured metadata |
| userAgent | High | No — use structured metadata |

## References

- [Loki Label Best Practices](https://grafana.com/docs/loki/next/get-started/labels/bp-labels/)
- [Structured Metadata](https://grafana.com/docs/loki/latest/get-started/labels/structured-metadata/)
- [OpenShift Logging 6.5 OTLP Data Model](https://docs.redhat.com/en/documentation/red_hat_openshift_logging/6.5/html/configuring_logging/opentelemetry-data-model)
- [OTLP Ingestion in Loki](https://docs.redhat.com/en/documentation/red_hat_openshift_logging/6.5/html/configuring_logging/configuring-lokistack-otlp)
