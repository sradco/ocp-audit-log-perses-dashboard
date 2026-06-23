# Performance Tuning for Audit Log Queries

The dashboard's LogQL query uses `| json` to parse every matching audit log line,
which is the most expensive operation. This document describes how to improve
query performance by leveraging Loki's indexing features.

## Current Query Cost

```
{log_type="audit"} | json | verb=~"create" | ...
```

With this pattern, Loki must:
1. Select all audit log streams (only `log_type` is indexed)
2. Decompress and parse the full JSON of **every** log line
3. Apply label filters after parsing

## Option 1: OTLP + Schema v13 (Recommended)

With OpenShift Logging 6.x using the OTLP data model and LokiStack schema v13,
most audit fields are **already stored as structured metadata** by default:

| Dashboard Filter | OTLP Attribute | Default Storage |
|-----------------|---------------|-----------------|
| Username | `k8s.user.username` | structured metadata |
| Resource | `k8s.audit.event.object_ref.resource` | structured metadata |
| Namespace | `k8s.audit.event.object_ref.namespace` | structured metadata |
| Resource Name | `k8s.audit.event.object_ref.name` | structured metadata |
| Response Code | `k8s.audit.event.response.code` | structured metadata |
| Client | `k8s.audit.event.user_agent` | structured metadata |
| **Verb** | **not mapped** | **not available** |

Structured metadata is queryable with label filter expressions without `| json`
parsing, significantly reducing query cost.

### Gap: `verb` is missing

The `verb` field is not in the default OTLP attribute mapping. We've opened an
upstream issue to add it:
- **Loki Operator**: https://github.com/grafana/loki/issues/22513

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

### Add verb as a custom stream label

Until the upstream issue is resolved, you can manually add `verb` as a stream
label for the audit tenant:

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
