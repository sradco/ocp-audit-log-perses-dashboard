# Performance Tuning for Audit Log Queries

## TL;DR — What to do now

1. **Use the OTLP data model** in your `ClusterLogForwarder`
2. **Use the "OCP Audit Log Viewer (OTLP)" dashboard**

This gives you stream-level filtering of `kubeAPI` vs `auditd` logs, meaning
Loki skips all Linux audit noise without parsing it. On typical clusters this
reduces the data Loki processes by 2-10x.

## Why OTLP is faster

| Format | Stream selector | What Loki parses |
|--------|----------------|------------------|
| ViaQ | `{log_type="audit"}` | ALL audit logs (kubeAPI + auditd), then filters via `| json` |
| OTLP | `{log_type="audit", openshift_log_source="kubeAPI"}` | Only kubeAPI logs — auditd skipped at index |

With ViaQ, `log_source` is inside the JSON body. Loki must decompress and parse
every log line just to check if it's a `kubeAPI` event.

With OTLP, `openshift_log_source` is a stream label. Loki filters at the index
level — `auditd` streams are never read from disk.

## Optional: Add `verb` as a stream label

The `verb` field has only ~6 values (create, update, patch, delete, get, list),
making it safe as a stream label. This lets Loki skip irrelevant verbs at the
index level too.

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
  limits:
    tenants:
      audit:
        otlp:
          streamLabels:
            logAttributes:
              - k8s.audit.event.verb
```

> **Note:** Requires the upstream mapping of `verb` to an OTLP attribute
> (tracked in https://github.com/grafana/loki/issues/22513).

## Future: Structured metadata for audit fields

Currently, audit event fields (username, resource, namespace, etc.) are still in
the JSON log body and require `| json` parsing. We've opened upstream issues to
have them stored as structured metadata, which would eliminate `| json` entirely:

- https://github.com/grafana/loki/issues/22513
- https://github.com/openshift/cluster-logging-operator/issues/3317

Once available, the OTLP dashboard will be updated to remove `| json` and query
structured metadata directly.

## Cardinality guidelines

Only add **low-cardinality** fields as stream labels:

| Field | Cardinality | Safe as stream label? |
|-------|-------------|----------------------|
| verb | ~6 values | Yes |
| responseStatus.code | ~12 values | Borderline |
| objectRef.resource | ~50-100 | No — use structured metadata |
| user.username | High | No — use structured metadata |
| objectRef.namespace | High | No — use structured metadata |
| userAgent | High | No — use structured metadata |

## References

- [Loki Label Best Practices](https://grafana.com/docs/loki/next/get-started/labels/bp-labels/)
- [Structured Metadata](https://grafana.com/docs/loki/latest/get-started/labels/structured-metadata/)
- [OpenShift Logging OTLP Data Model](https://docs.redhat.com/en/documentation/red_hat_openshift_logging/6.5/html/configuring_logging/opentelemetry-data-model)
- [OTLP Ingestion in Loki](https://docs.redhat.com/en/documentation/red_hat_openshift_logging/6.5/html/configuring_logging/configuring-lokistack-otlp)
