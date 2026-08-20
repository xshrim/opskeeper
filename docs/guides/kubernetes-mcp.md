# Kubernetes MCP

`kubernetes-mcp` is a standalone, read-only MCP server at `backend/mcpserver/kubernetes/`. It exposes Streamable HTTP at `/mcp` and SSE at `/sse`, listening on `0.0.0.0:8812` by default.

## Connection precedence

Every tool resolves its own cluster connection in this order:

1. `kubeconfig_base64` tool parameter. The value is standard base64-encoded kubeconfig content and has the highest priority.
2. Tool connection parameters, then `KUBERNETES_MCP_*` environment configuration (endpoint, kubeconfig path, context, profile).
3. `in_cluster` service-account configuration when `KUBERNETES_MCP_MODE` is `auto` and no endpoint or kubeconfig is configured.

The kubeconfig parameter is intentionally a flat string so browser clients do not need to compile a nested JSON schema. Credentials and kubeconfig contents are never logged or returned.

Useful environment variables include:

```text
KUBERNETES_MCP_HTTP_ADDRESS=0.0.0.0:8812
KUBERNETES_MCP_MODE=auto
KUBERNETES_MCP_KUBECONFIG=/home/user/.kube/config
KUBERNETES_MCP_CONTEXT=dev
KUBERNETES_MCP_SERVER=https://cluster.example:6443
KUBERNETES_MCP_CA_FILE=/etc/kubernetes/ca.crt
KUBERNETES_MCP_TOKEN_FILE=/etc/kubernetes/token
KUBERNETES_MCP_BEARER_TOKEN=
KUBERNETES_MCP_SKIP_TLS_VERIFY=false
KUBERNETES_MCP_PROFILES_FILE=/etc/kubernetes-mcp/profiles.yaml
KUBERNETES_MCP_DEFAULT_PROFILE=local
```

## Tools

The server provides cluster info, API resource discovery, namespaces, nodes, pods, workloads, services, ConfigMaps, ingresses, events, bounded pod logs, allowlisted resource get, and an API health check. Lists accept a flat `filters` string such as `app:payments,environment:prod`; the tool converts it to a Kubernetes label selector. Lists default to a bounded page and cap `limit` at 500. Logs default to 100 lines, are never followed, and are capped at 256 KiB.

The resource allowlist includes namespaces, nodes, pods, ConfigMaps, services, events, deployments, statefulsets, daemonsets, jobs, cronjobs, ingresses, and endpoint slices. Secrets, RBAC objects, arbitrary API paths, watches, exec, and all write operations are unavailable.

All tool input schemas are hand-written primitive-only objects. Tools do not advertise an output schema; results are stable JSON summaries, which avoids client-side dynamic schema compilation under strict browser CSP.

Run with `make kubernetes-mcp-run`, test with `make kubernetes-mcp-test`, or build with `make kubernetes-mcp-build`.
