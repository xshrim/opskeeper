# Kubernetes MCP

`kubernetes-mcp` is a standalone, read-only MCP server at `backend/mcpserver/kubernetes/`. It exposes Streamable HTTP at `/mcp` and SSE at `/sse`, listening on `0.0.0.0:8812` by default.

## Connection precedence

Every tool resolves its own cluster connection in this order:

1. `kubeconfig` tool parameter. The value is standard base64-encoded kubeconfig content and has the highest priority.
2. Other tool connection parameters, then `KUBERNETES_MCP_*` environment configuration (endpoint, kubeconfig content, context, profile).
3. client-go's default kubeconfig loading (`$KUBECONFIG`, then `$HOME/.kube/config`), then in-cluster service-account configuration when no endpoint or kubeconfig is configured.

The kubeconfig parameter is intentionally a flat string so browser clients do not need to compile a nested JSON schema. Credentials and kubeconfig contents are never logged or returned.

Connection parameters are `kubeconfig` (Base64 kubeconfig), `connection_mode`, `context`, `profile`, `server`, `ca`, `token`, `client_cert`, `client_key`, and `skip_tls_verify`. `kubeconfig`, `ca`, `client_cert`, and `client_key` carry content, not file paths; there are no path or token-file parameters.

Useful environment variables include:

```text
KUBERNETES_MCP_HTTP_ADDRESS=0.0.0.0:8812
KUBERNETES_MCP_MODE=auto
KUBERNETES_MCP_KUBECONFIG=<base64 kubeconfig content>
KUBERNETES_MCP_CONTEXT=dev
KUBERNETES_MCP_SERVER=https://cluster.example:6443
KUBERNETES_MCP_CA=<PEM or base64 PEM>
KUBERNETES_MCP_TOKEN=
KUBERNETES_MCP_CLIENT_CERT=<PEM or base64 PEM>
KUBERNETES_MCP_CLIENT_KEY=<PEM or base64 PEM>
KUBERNETES_MCP_BEARER_TOKEN=
KUBERNETES_MCP_SKIP_TLS_VERIFY=false
KUBERNETES_MCP_PROFILES_FILE=/etc/kubernetes-mcp/profiles.yaml
KUBERNETES_MCP_DEFAULT_PROFILE=local
```

`KUBERNETES_MCP_TOKEN` is the optional Kubernetes API bearer token. `KUBERNETES_MCP_BEARER_TOKEN` independently protects this MCP server's HTTP endpoints when set.

## Tools

The server provides cluster info, API resource discovery, namespaces, nodes, pods, workloads, `kubernetes_pod_stat`, `kubernetes_node_stat`, services, ConfigMaps, ingresses, EndpointSlices, events, bounded pod logs, allowlisted resource get, and an API health check. The two `*_stat` tools read current CPU, memory, and other reported resource usage from `metrics.k8s.io/v1beta1`; they require a working metrics-server or another Metrics API implementation and the caller's RBAC permission to read it. `kubernetes_pod_stat` accepts an optional namespace and pod name and returns per-container plus aggregated usage; `kubernetes_node_stat` accepts an optional node name and returns node usage. Lists accept a flat `filters` string such as `app:payments,environment:prod`; the tool converts it to a Kubernetes label selector. Lists default to a bounded page and cap `limit` at 500. Logs default to 100 lines, are never followed, and are capped at 256 KiB.

The resource allowlist includes namespaces, nodes, pods, ConfigMaps, services, events, deployments, statefulsets, daemonsets, jobs, cronjobs, ingresses, and endpoint slices. Secrets, RBAC objects, arbitrary API paths, watches, exec, and all write operations are unavailable.

All tool input schemas are hand-written primitive-only objects. Tools do not advertise an output schema; results are stable JSON summaries, which avoids client-side dynamic schema compilation under strict browser CSP.

Run with `make kubernetes-mcp-run`, test with `make kubernetes-mcp-test`, or build with `make kubernetes-mcp-build`.
