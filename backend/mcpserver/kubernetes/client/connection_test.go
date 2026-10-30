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
	config, profile, err := resolveRESTConfig(ConnectionInput{Kubeconfig: base64.StdEncoding.EncodeToString(kubeconfig)})
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
	t.Setenv("KUBERNETES_MCP_TOKEN", "cluster-token")
	config, _, err := resolveRESTConfig(ConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "https://cluster.example:6443" {
		t.Fatalf("host=%q", config.Host)
	}
	if config.BearerToken != "cluster-token" {
		t.Fatalf("token=%q", config.BearerToken)
	}
}

func TestToolKubeconfigUsesExplicitContext(t *testing.T) {
	kubeconfig := []byte("apiVersion: v1\nkind: Config\nclusters:\n- name: first\n  cluster:\n    server: https://first.example:6443\n- name: second\n  cluster:\n    server: https://second.example:6443\ncontexts:\n- name: first\n  context:\n    cluster: first\n    user: test\n- name: second\n  context:\n    cluster: second\n    user: test\ncurrent-context: first\nusers:\n- name: test\n  user: {}\n")
	config, _, err := resolveRESTConfig(ConnectionInput{Kubeconfig: base64.StdEncoding.EncodeToString(kubeconfig), Context: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "https://second.example:6443" {
		t.Fatalf("host=%q", config.Host)
	}
}

func TestContextsReturnsSortedOptionsAndMarksConfiguredContext(t *testing.T) {
	kubeconfig := []byte("apiVersion: v1\nkind: Config\nclusters:\n- name: first\n  cluster:\n    server: https://first.example:6443\n- name: second\n  cluster:\n    server: https://second.example:6443\ncontexts:\n- name: z-context\n  context:\n    cluster: first\n    user: test\n- name: a-context\n  context:\n    cluster: second\n    user: test\ncurrent-context: z-context\nusers:\n- name: test\n  user: {}\n")
	options, err := Contexts(ConnectionInput{Kubeconfig: base64.StdEncoding.EncodeToString(kubeconfig), Context: "a-context"})
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].Name != "a-context" || !options[0].Current || options[1].Name != "z-context" || options[1].Current {
		t.Fatalf("contexts = %#v", options)
	}
}

func TestContextsEndpointModeReturnsOnlyConfiguredEndpointContext(t *testing.T) {
	t.Setenv("KUBERNETES_MCP_KUBECONFIG", "")
	options, err := Contexts(ConnectionInput{ConnectionMode: "endpoint"})
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 1 || options[0].Name != "default" || !options[0].Current {
		t.Fatalf("contexts = %#v", options)
	}
}

func TestEndpointTLSMaterialsAcceptBase64PEM(t *testing.T) {
	pem := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"
	encoded := base64.StdEncoding.EncodeToString([]byte(pem))
	config, _, err := endpointConfig(ConnectionInput{Server: "https://cluster.example:6443", CA: encoded, ClientCert: encoded, ClientKey: encoded}, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(config.CAData) != pem || string(config.CertData) != pem || string(config.KeyData) != pem {
		t.Fatalf("TLS data was not decoded: ca=%q cert=%q key=%q", config.CAData, config.CertData, config.KeyData)
	}
}
