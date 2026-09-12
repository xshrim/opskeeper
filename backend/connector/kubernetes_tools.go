package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opskeeper/backend/aiengine"
	kclient "opskeeper/backend/mcpserver/kubernetes/client"
	kt "opskeeper/backend/tool/kubernetes"
)

func (s *Service) resolveKubernetesTools(ctx context.Context, item aiengine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (aiengine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("Kubernetes agent resources must use the MCP provider")
	}
	connection, err := s.kubernetesConnection(ctx, item)
	if err != nil {
		return err
	}
	register := func(name, description string, extra map[string]any, fn func(context.Context, map[string]any) (any, error)) {
		add(name, description, directKubernetesSchema(extra), func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
			out, callErr := fn(runCtx, args)
			if callErr != nil {
				return aiengine.ToolResult{}, callErr
			}
			return aiengine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	register("kubernetes_cluster_info", "Read Kubernetes version and connection information.", nil, func(c context.Context, _ map[string]any) (any, error) { return kt.ClusterInfo(c, connection) })
	register("kubernetes_api_resources", "List API resources supported by the connected cluster.", nil, func(c context.Context, _ map[string]any) (any, error) { return kt.APIResources(c, connection) })
	for _, tool := range kt.ListTools() {
		item := tool
		register(item.Name, item.Description, kt.ListInputProperties(), func(c context.Context, a map[string]any) (any, error) {
			return kt.List(c, connection, item.Resource, stringArg(a, "namespace"), stringArg(a, "filters"), stringArg(a, "continue"), int(int64Arg(a, "limit")))
		})
	}
	register("kubernetes_workloads", "List Kubernetes workloads.", kt.ListInputProperties(), func(c context.Context, a map[string]any) (any, error) {
		return kt.Workloads(c, connection, stringArg(a, "namespace"), stringArg(a, "filters"), stringArg(a, "continue"), int(int64Arg(a, "limit")))
	})
	register("kubernetes_pod_stat", "Read current pod CPU and memory usage from the Kubernetes Metrics API.", kt.PodStatsInputProperties(), func(c context.Context, a map[string]any) (any, error) {
		return kt.PodStats(c, connection, stringArg(a, "namespace"), stringArg(a, "pod"), stringArg(a, "filters"), stringArg(a, "continue"), int(int64Arg(a, "limit")))
	})
	register("kubernetes_node_stat", "Read current node CPU and memory usage from the Kubernetes Metrics API.", kt.NodeStatsInputProperties(), func(c context.Context, a map[string]any) (any, error) {
		return kt.NodeStats(c, connection, stringArg(a, "node"), stringArg(a, "filters"), stringArg(a, "continue"), int(int64Arg(a, "limit")))
	})
	register("kubernetes_pod_logs", "Read bounded, non-following pod logs.", kt.PodLogsInputProperties(), func(c context.Context, a map[string]any) (any, error) {
		return kt.PodLogs(c, connection, stringArg(a, "namespace"), stringArg(a, "pod"), stringArg(a, "container_name"), int64Arg(a, "tail"), boolArg(a, "timestamps", false))
	})
	register("kubernetes_pod_file", "Read a bounded regular file from a Kubernetes pod.", kt.PodFileInputProperties(), func(c context.Context, a map[string]any) (any, error) {
		return kt.PodFile(c, connection, stringArg(a, "namespace"), stringArg(a, "pod"), stringArg(a, "container_name"), stringArg(a, "path"))
	})
	register("kubernetes_resource_get", "Get an allowlisted Kubernetes resource by name.", kt.GetInputProperties(), func(c context.Context, a map[string]any) (any, error) {
		return kt.Get(c, connection, stringArg(a, "resource"), stringArg(a, "namespace"), stringArg(a, "name"))
	})
	register("kubernetes_health", "Check Kubernetes API health.", nil, func(c context.Context, _ map[string]any) (any, error) { return kt.Health(c, connection) })
	return nil
}

func directKubernetesSchema(extra map[string]any) json.RawMessage {
	schema := kt.InputSchema(extra)
	if p, ok := schema["properties"].(map[string]any); ok {
		for _, k := range []string{"kubeconfig", "connection_mode", "context", "profile", "server", "ca", "token", "client_cert", "client_key", "skip_tls_verify"} {
			delete(p, k)
		}
	}
	b, _ := json.Marshal(schema)
	return b
}

func (s *Service) kubernetesConnection(ctx context.Context, item aiengine.ContextResource) (kclient.ConnectionInput, error) {
	c := kclient.ConnectionInput{}
	setKubernetesConnection(&c, item.Config)
	if item.CredentialID == nil || strings.TrimSpace(*item.CredentialID) == "" {
		return c, nil
	}
	if s.credentials == nil {
		return c, fmt.Errorf("credential service is unavailable")
	}
	secret, err := s.credentials.RevealLinked(ctx, *item.CredentialID)
	if err != nil {
		return c, err
	}
	var values map[string]any
	if json.Unmarshal(secret, &values) == nil {
		setKubernetesConnectionEmpty(&c, values)
		if c.ConnectionMode != "endpoint" && c.Kubeconfig == "" {
			c.Kubeconfig = stringValue(values, "kubeconfig")
		}
	}
	return c, nil
}
func setKubernetesConnection(c *kclient.ConnectionInput, v map[string]any) {
	if c == nil {
		return
	}
	c.Kubeconfig = stringValue(v, "kubeconfig")
	c.ConnectionMode = stringValue(v, "connection_mode")
	c.Context = stringValue(v, "context")
	c.Profile = stringValue(v, "profile")
	c.Server = stringValue(v, "server")
	c.CA = stringValue(v, "ca")
	c.Token = stringValue(v, "token")
	c.ClientCert = stringValue(v, "client_cert")
	c.ClientKey = stringValue(v, "client_key")
	c.SkipTLSVerify = boolValue(v, "skip_tls_verify")
}
func setKubernetesConnectionEmpty(c *kclient.ConnectionInput, v map[string]any) {
	if c.ConnectionMode != "endpoint" && c.Kubeconfig == "" {
		c.Kubeconfig = stringValue(v, "kubeconfig")
	}
	if c.ConnectionMode == "" {
		c.ConnectionMode = stringValue(v, "connection_mode")
	}
	if c.Context == "" {
		c.Context = stringValue(v, "context")
	}
	if c.Profile == "" {
		c.Profile = stringValue(v, "profile")
	}
	if c.Server == "" {
		c.Server = stringValue(v, "server")
	}
	if c.CA == "" {
		c.CA = stringValue(v, "ca")
	}
	if c.Token == "" {
		c.Token = stringValue(v, "token")
	}
	if c.ClientCert == "" {
		c.ClientCert = stringValue(v, "client_cert")
	}
	if c.ClientKey == "" {
		c.ClientKey = stringValue(v, "client_key")
	}
	if !c.SkipTLSVerify {
		c.SkipTLSVerify = boolValue(v, "skip_tls_verify")
	}
}
