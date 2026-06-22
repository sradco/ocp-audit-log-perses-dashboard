# Recommended Loki Audit Log Filtering

By default, Kubernetes API audit logs are extremely verbose — most entries come from service accounts, node heartbeats, and watch/proxy operations. Filtering at the collection layer reduces Loki storage costs and improves query performance.

## Option A: Add to an Existing ClusterLogForwarder

If you already have a `ClusterLogForwarder` with a `kubeAPIAudit` policy filter (common in OpenShift), add `type: drop` filters to catch events the policy misses (e.g. unauthenticated requests with empty usernames, non-complete stages):

```bash
oc edit clusterlogforwarders.observability.openshift.io <name> -n openshift-logging
```

Add the filters to your existing `spec.filters` section:

```yaml
spec:
  filters:
    # ... your existing filters (e.g. audit-policy) ...

    # Drop unauthenticated requests (no user identity, typically 401s)
    - name: drop-unauthenticated
      type: drop
      drop:
        - test:
            - field: .user.username
              matches: "^$"

    # Drop non-complete stages (RequestReceived, ResponseStarted)
    - name: drop-non-complete
      type: drop
      drop:
        - test:
            - field: .stage
              notMatches: ResponseComplete
```

Then reference them in your audit pipeline's `filterRefs`:

```yaml
  pipelines:
    - name: audit-filtered-to-loki    # your existing audit pipeline
      inputRefs:
        - audit
      filterRefs:
        - audit-policy                 # existing kubeAPIAudit filter
        - drop-unauthenticated         # add this
        - drop-non-complete            # add this
      outputRefs:
        - default-lokistack
```

Filters in `filterRefs` are applied in order — the `kubeAPIAudit` policy runs first, then `drop` filters catch anything remaining.

---

## Option B: Standalone ClusterLogForwarder (New Setup)

If starting fresh without an existing `ClusterLogForwarder`, use this complete configuration:

```yaml
apiVersion: observability.openshift.io/v1
kind: ClusterLogForwarder
metadata:
  name: collector
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

    # Drop unauthenticated requests (no user identity resolved, typically 401s)
    - name: drop-unauthenticated
      type: drop
      drop:
        - test:
            - field: .user.username
              matches: "^$"

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
        - drop-unauthenticated
        - drop-system-users
        - drop-noisy-verbs
      outputRefs:
        - default-lokistack
```

---

## What Each Filter Does

| Filter | Drops | Typical Reduction |
|--------|-------|-------------------|
| `drop-non-complete` | Intermediate audit stages (RequestReceived, ResponseStarted) | ~60-70% |
| `drop-unauthenticated` | Requests with no user identity (failed auth, 401s) | ~5-10% |
| `drop-system-users` | Service accounts, nodes, internal operators | ~80-90% of remaining |
| `drop-noisy-verbs` | watch (long-poll), deletecollection, proxy | ~10-20% |

Combined, these filters typically reduce audit log volume by **95%+** while retaining all human user activity and mutations.

## Applying and Verifying

```bash
# Edit existing CLF
oc edit clusterlogforwarders.observability.openshift.io collector -n openshift-logging

# Or apply standalone from file
oc apply -f clusterlogforwarder.yaml
```

After applying, verify logs are being filtered:

```bash
# Check collector pods picked up the config
oc get pods -n openshift-logging -l component=collector

# Verify audit logs still flow (from a human user)
oc get pods -n default  # generates an audit event
# Then check Loki via the dashboard or directly:
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

The dashboard's "Exclude System Users" and "Hide Unauthenticated" filters provide **display-time** filtering on top of what's already collected. The ClusterLogForwarder filters above operate at **collection-time** — events dropped here are never stored in Loki and cannot be queried.

Recommended approach:
1. Use ClusterLogForwarder filters to drop the bulk of noise (permanent, saves storage)
2. Use dashboard filters for interactive exploration of what remains
