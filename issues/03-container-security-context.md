### Problem Statement

The deployment template sets `runAsNonRoot: true` and `seccompProfile: RuntimeDefault` at the pod level, but is missing container-level security hardening:

- No `readOnlyRootFilesystem: true`
- No `allowPrivilegeEscalation: false`
- No `capabilities: { drop: [ALL] }`
- No `runAsUser` / `runAsGroup` specified (relies on the distroless image default)

The Dockerfile uses `gcr.io/distroless/static:nonroot` which is good, but the chart should enforce these constraints regardless of image choice.

### Proposed Solution

1. Add container-level `securityContext` to the deployment template:
   ```yaml
   securityContext:
     allowPrivilegeEscalation: false
     readOnlyRootFilesystem: true
     runAsNonRoot: true
     capabilities:
       drop: [ALL]
   ```
2. Optionally make these configurable via `values.yaml` for environments that need overrides.

### Alternatives Considered

- Relying on Pod Security Standards enforcement at the namespace level — but the chart should be self-contained and not depend on cluster-level policy.

### Willingness to Contribute

Yes, I can submit a PR

### Additional Context

- batch-gateway's chart sets container-level security context fields.
- The llm-d `THREAT-MODEL.md` and `SECURITY.md` imply a security-conscious posture; the chart should match.
- Pod Security Standards (Restricted profile) require all of these fields.
- Graduation criteria addressed: Security, Deployment (production-readiness).
