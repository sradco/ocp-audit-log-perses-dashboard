# Recommended Loki Audit Log Filtering

By default, Kubernetes API audit logs are extremely verbose — most entries come from service accounts, node heartbeats, and watch/proxy operations. Filtering at the collection layer reduces Loki storage costs and improves query performance.

## ClusterLogForwarder Filter Configuration

Add the following filters to your `ClusterLogForwarder` to drop noise before it reaches Loki:

```yaml
apiVersion: observability.openshift.io/v1
kind: ClusterLogForwarder
metadata:
  name: instance
  namespace: openshift-logging
spec:
  filters:
    # Only keep fully completed API responses (drop RequestReceived, ResponseStarted)
    - name: drop-non-complete
      type: drop
      drop:
        - test:
            - field: .stage
              notMatches: ResponseComplete

    # Drop automated system users (service accounts, nodes, internal components)
    - name: drop-system-users
      type: drop
      drop:
        - test:
            - field: .user.username
              matches: "^system:serviceaccount:"
        - test:
            - field: .user.username
              matches: "^system:node:"
        - test:
            - field: .user.username
              matches: "^system:(apiserver|kube-|anonymous|unauthenticated|openshift:|aggregator|monitoring|multus)"

    # Drop high-volume verbs that rarely have security/audit value
    - name: drop-noisy-verbs
      type: drop
      drop:
        - test:
            - field: .verb
              matches: "^(watch|deletecollection|proxy)$"

    # Optional: drop read-only verbs to keep only mutations
    # - name: drop-reads
    #   type: drop
    #   drop:
    #     - test:
    #         - field: .verb
    #           matches: "^(get|list)$"

    # Optional: drop health check endpoints
    # - name: drop-health-checks
    #   type: drop
    #   drop:
    #     - test:
    #         - field: .requestURI
    #           matches: "^/(healthz|livez|readyz|version|openapi)"

  pipelines:
    - name: audit-to-loki
      inputRefs:
        - audit
      filterRefs:
        - drop-non-complete
        - drop-system-users
        - drop-noisy-verbs
      outputRefs:
        - default
```

## What Each Filter Does

| Filter | Drops | Typical Reduction |
|--------|-------|-------------------|
| `drop-non-complete` | Intermediate audit stages (RequestReceived, ResponseStarted) | ~60-70% |
| `drop-system-users` | Service accounts, nodes, internal operators | ~80-90% of remaining |
| `drop-noisy-verbs` | watch (long-poll), deletecollection, proxy | ~10-20% |

Combined, these filters typically reduce audit log volume by **95%+** while retaining all human user activity and mutations.

## Applying the Filter

```bash
# Edit the existing ClusterLogForwarder
oc edit clusterlogforwarder instance -n openshift-logging

# Or apply from file
oc apply -f clusterlogforwarder.yaml
```

After applying, verify logs are being filtered:

```bash
# Check collector pods picked up the config
oc get pods -n openshift-logging -l component=collector

# Verify audit logs still flow (from a human user)
oc get pods -n default  # generates an audit event
# Then check Loki
oc exec -n openshift-logging <loki-pod> -- logcli query '{log_type="audit"}' --limit=5
```

## Tuning Tips

- **Keep `get`/`list` if** you need to track who reads secrets, configmaps, or RBAC resources
- **Add `drop-reads`** if you only care about mutations (create/update/patch/delete) and 403s
- **Add resource-specific filters** to drop noisy resources like `events`, `endpoints`, `leases`:

```yaml
    - name: drop-noisy-resources
      type: drop
      drop:
        - test:
            - field: .objectRef.resource
              matches: "^(events|endpoints|leases|tokenreviews|subjectaccessreviews)$"
```

- **Keep 403s even from system users** if you want to detect RBAC misconfigurations:

```yaml
    # Alternative: only drop system users that succeed
    - name: drop-system-users-success
      type: drop
      drop:
        - test:
            - field: .user.username
              matches: "^system:"
            - field: .responseStatus.code
              notMatches: "^(401|403)$"
```

## Alignment with Dashboard

The dashboard's "Exclude System Users" filter provides **display-time** filtering on top of what's already collected. The ClusterLogForwarder filters above operate at **collection-time** — events dropped here are never stored in Loki and cannot be queried.

Recommended approach:
1. Use ClusterLogForwarder filters to drop the bulk of noise (permanent, saves storage)
2. Use dashboard filters for interactive exploration of what remains
