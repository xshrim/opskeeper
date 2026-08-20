package client

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestToolKubeconfigHasHighestPriority(t *testing.T) {
	kubeconfig := []byte("apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: https://tool.example:6443\ncontexts:\n- name: test\n  context:\n    cluster: test\n    user: test\ncurrent-context: test\nusers:\n- name: test\n  user: {}\n")
	t.Setenv("KUBERNETES_MCP_MODE", "endpoint")
	t.Setenv("KUBERNETES_MCP_SERVER", "https://environment.invalid")
	config, profile, err := resolveRESTConfig(ConnectionInput{KubeconfigBase64: base64.StdEncoding.EncodeToString(kubeconfig)})
	if err != nil {
		t.Fatal(err)
	}
	if profile != "tool-kubeconfig" || !strings.HasPrefix(config.Host, "https://tool.example:6443") {
		t.Fatalf("config=%+v profile=%q", config, profile)
	}
}

func TestEndpointModeUsesEnvironment(t *testing.T) {
	t.Setenv("KUBERNETES_MCP_MODE", "endpoint")
	t.Setenv("KUBERNETES_MCP_SERVER", "https://cluster.example:6443")
	config, _, err := resolveRESTConfig(ConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "https://cluster.example:6443" {
		t.Fatalf("host=%q", config.Host)
	}
}
