### Problem Statement

The async-processor exposes Prometheus metrics on port 9090 and documents 7 counters/histograms with example PromQL queries. However, the Helm chart provides no:

- **ServiceMonitor** or **PodMonitor** for the async-processor itself (it only has one for the model server via `modelserver-podmonitor.yaml`). Users must manually configure Prometheus scraping.
- **PrometheusRule** for alerting (e.g. high retry rate, deadline exceeded rate, low success ratio).
- **Grafana dashboard** JSON for the documented metrics.

### Proposed Solution

1. Add a `ServiceMonitor` (or `PodMonitor`) template for the async-processor pod (gated by `values.yaml`).
2. Add a `PrometheusRule` template with alerts for:
   - High retry ratio (`rate(retries) / rate(total) > threshold`)
   - High deadline-exceeded rate
   - Low success ratio
   - High shedded request rate
3. Add a Grafana dashboard JSON covering the documented metrics (request rate, success/failure/retry breakdown, queue-level views, latency histogram).
4. Gate all three behind `values.yaml` toggles (e.g. `monitoring.serviceMonitor.enabled`, `monitoring.alerts.enabled`, `grafana.dashboards.enabled`).

### Alternatives Considered

- Providing only documentation with example PromQL queries (current state) — but this requires every user to manually build dashboards and alerts, which reduces adoption.

### Willingness to Contribute

Yes, with guidance

### Additional Context

- batch-gateway ships a ServiceMonitor, PodMonitor (for GC), PrometheusRule, and three Grafana dashboard JSON files (apiserver, gc, processor) as part of its Helm chart.
- The README already has example PromQL queries that could be turned into dashboard panels.
- Graduation criteria addressed: Documentation (operational), Deployment (production-readiness), Reliability (alerting).
