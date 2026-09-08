package connector

import (
	"context"
	"encoding/base64"
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
	for _, r := range []struct{ name, desc, resource string }{
		{"kubernetes_namespaces", "List Kubernetes namespaces.", "namespaces"}, {"kubernetes_nodes", "List Kubernetes nodes.", "nodes"}, {"kubernetes_pods", "List Kubernetes pods.", "pods"}, {"kubernetes_services", "List Kubernetes services.", "services"}, {"kubernetes_configmaps", "List Kubernetes ConfigMaps.", "configmaps"}, {"kubernetes_ingresses", "List Kubernetes ingresses.", "ingresses"}, {"kubernetes_events", "List Kubernetes events.", "events"},
	} {
		item := r
		register(item.name, item.desc, listExtras(), func(c context.Context, a map[string]any) (any, error) {
			return kt.List(c, connection, item.resource, stringArg(a, "namespace"), stringArg(a, "filters"), stringArg(a, "continue"), int(int64Arg(a, "limit")))
		})
	}
	register("kubernetes_workloads", "List Kubernetes workloads.", listExtras(), func(c context.Context, a map[string]any) (any, error) {
		return kt.Workloads(c, connection, stringArg(a, "namespace"), stringArg(a, "filters"), stringArg(a, "continue"), int(int64Arg(a, "limit")))
	})
	register("kubernetes_pod_logs", "Read bounded, non-following pod logs.", map[string]any{"namespace": map[string]any{"type": "string"}, "pod": map[string]any{"type": "string"}, "container_name": map[string]any{"type": "string"}, "tail": map[string]any{"type": "integer"}, "timestamps": map[string]any{"type": "boolean"}}, func(c context.Context, a map[string]any) (any, error) {
		return kt.PodLogs(c, connection, stringArg(a, "namespace"), stringArg(a, "pod"), stringArg(a, "container_name"), int64Arg(a, "tail"), boolArg(a, "timestamps", false))
	})
	register("kubernetes_resource_get", "Get an allowlisted Kubernetes resource by name.", map[string]any{"resource": map[string]any{"type": "string"}, "namespace": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}, func(c context.Context, a map[string]any) (any, error) {
		return kt.Get(c, connection, stringArg(a, "resource"), stringArg(a, "namespace"), stringArg(a, "name"))
	})
	register("kubernetes_health", "Check Kubernetes API health.", nil, func(c context.Context, _ map[string]any) (any, error) { return kt.Health(c, connection) })
	return nil
}

func directKubernetesSchema(extra map[string]any) json.RawMessage {
	schema := kt.InputSchema(extra)
	if p, ok := schema["properties"].(map[string]any); ok {
		for _, k := range []string{"kubeconfig_base64", "connection_mode", "kubeconfig_path", "context", "profile", "server", "ca_file", "token", "token_file", "client_cert_file", "client_key_file", "skip_tls_verify"} {
			delete(p, k)
		}
	}
	b, _ := json.Marshal(schema)
	return b
}

func listExtras() map[string]any {
	return map[string]any{"namespace": map[string]any{"type": "string"}, "filters": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}, "continue": map[string]any{"type": "string"}}
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
		if c.KubeconfigBase64 == "" {
			if raw := stringValue(values, "kubeconfig"); raw != "" {
				c.KubeconfigBase64 = base64.StdEncoding.EncodeToString([]byte(raw))
			}
		}
	}
	return c, nil
}
func setKubernetesConnection(c *kclient.ConnectionInput, v map[string]any) {
	if c == nil {
		return
	}
	c.KubeconfigBase64 = stringValue(v, "kubeconfig_base64")
	c.KubeconfigPath = stringValue(v, "kubeconfig_path")
	c.ConnectionMode = stringValue(v, "connection_mode")
	c.Context = stringValue(v, "context")
	c.Profile = stringValue(v, "profile")
	c.Server = stringValue(v, "server")
	c.CAFile = stringValue(v, "ca_file")
	c.Token = stringValue(v, "token")
	c.TokenFile = stringValue(v, "token_file")
	c.ClientCertFile = stringValue(v, "client_cert_file")
	c.ClientKeyFile = stringValue(v, "client_key_file")
	c.SkipTLSVerify = boolValue(v, "skip_tls_verify")
}
func setKubernetesConnectionEmpty(c *kclient.ConnectionInput, v map[string]any) {
	if c.KubeconfigBase64 == "" {
		c.KubeconfigBase64 = stringValue(v, "kubeconfig_base64")
	}
	if c.KubeconfigPath == "" {
		c.KubeconfigPath = stringValue(v, "kubeconfig_path")
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
	if c.CAFile == "" {
		c.CAFile = stringValue(v, "ca_file")
	}
	if c.Token == "" {
		c.Token = stringValue(v, "token")
	}
	if c.TokenFile == "" {
		c.TokenFile = stringValue(v, "token_file")
	}
	if c.ClientCertFile == "" {
		c.ClientCertFile = stringValue(v, "client_cert_file")
	}
	if c.ClientKeyFile == "" {
		c.ClientKeyFile = stringValue(v, "client_key_file")
	}
	if !c.SkipTLSVerify {
		c.SkipTLSVerify = boolValue(v, "skip_tls_verify")
	}
}
