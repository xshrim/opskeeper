package client

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// ConnectionInput is deliberately flat and primitive-only. Kubeconfig content
// is accepted as base64 so clients never need to construct a nested schema.
type ConnectionInput struct {
	Kubeconfig     string `json:"kubeconfig,omitempty"`
	ConnectionMode string `json:"connection_mode,omitempty"`
	Context        string `json:"context,omitempty"`
	Profile        string `json:"profile,omitempty"`
	Server         string `json:"server,omitempty"`
	CA             string `json:"ca,omitempty"`
	Token          string `json:"token,omitempty"`
	ClientCert     string `json:"client_cert,omitempty"`
	ClientKey      string `json:"client_key,omitempty"`
	SkipTLSVerify  bool   `json:"skip_tls_verify,omitempty"`
}

type Config struct {
	Profile string
	Mode    string
}

// Connection owns clients created for one tool call.
type Connection struct {
	Dynamic    dynamic.Interface
	Kubernetes kubernetes.Interface
	Discovery  discovery.DiscoveryInterface
	RESTConfig *rest.Config
	Profile    string
}

type profileFile struct {
	Profiles map[string]profile `json:"profiles" yaml:"profiles"`
}
type profile struct {
	Mode          string `json:"mode" yaml:"mode"`
	Kubeconfig    string `json:"kubeconfig" yaml:"kubeconfig"`
	Context       string `json:"context" yaml:"context"`
	Server        string `json:"server" yaml:"server"`
	CA            string `json:"ca" yaml:"ca"`
	Token         string `json:"token" yaml:"token"`
	ClientCert    string `json:"client_cert" yaml:"client_cert"`
	ClientKey     string `json:"client_key" yaml:"client_key"`
	SkipTLSVerify bool   `json:"skip_tls_verify" yaml:"skip_tls_verify"`
}

func Open(ctx context.Context, input ConnectionInput) (*Connection, error) {
	config, profileName, err := resolveRESTConfig(input)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errors.New("Kubernetes connection is not configured")
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes dynamic client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes discovery client: %w", err)
	}
	typedClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes typed client: %w", err)
	}
	return &Connection{Dynamic: dynamicClient, Kubernetes: typedClient, Discovery: discoveryClient, RESTConfig: config, Profile: profileName}, nil
}

func resolveRESTConfig(input ConnectionInput) (*rest.Config, string, error) {
	// Tool-supplied kubeconfig is the explicit highest-priority connection.
	if raw := strings.TrimSpace(input.Kubeconfig); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "tool-kubeconfig", fmt.Errorf("decode kubeconfig: %w", err)
		}
		return restConfigFromBytes(decoded, strings.TrimSpace(input.Context), "tool-kubeconfig")
	}
	explicitMode := strings.TrimSpace(input.ConnectionMode)
	explicitServer := strings.TrimSpace(input.Server)
	resolved := input
	profileName := strings.TrimSpace(resolved.Profile)
	if resolved.ConnectionMode == "" {
		resolved.ConnectionMode = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_MODE"))
	}
	if resolved.Kubeconfig == "" {
		resolved.Kubeconfig = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_KUBECONFIG"))
	}
	if resolved.Context == "" {
		resolved.Context = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CONTEXT"))
	}
	if resolved.Server == "" {
		resolved.Server = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_SERVER"))
	}
	if resolved.CA == "" {
		resolved.CA = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CA"))
	}
	if resolved.Token == "" {
		resolved.Token = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_TOKEN"))
	}
	if resolved.ClientCert == "" {
		resolved.ClientCert = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CLIENT_CERT"))
	}
	if resolved.ClientKey == "" {
		resolved.ClientKey = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CLIENT_KEY"))
	}
	if !resolved.SkipTLSVerify {
		resolved.SkipTLSVerify = envBool("KUBERNETES_MCP_SKIP_TLS_VERIFY", false)
	}
	if profileName == "" {
		profileName = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_DEFAULT_PROFILE"))
	}
	if profileName != "" {
		p, err := loadProfile(profileName)
		if err != nil {
			return nil, profileName, err
		}
		if resolved.ConnectionMode == "" {
			resolved.ConnectionMode = p.Mode
		}
		if resolved.Kubeconfig == "" {
			resolved.Kubeconfig = p.Kubeconfig
		}
		if resolved.Context == "" {
			resolved.Context = p.Context
		}
		if resolved.Server == "" {
			resolved.Server = p.Server
		}
		if resolved.CA == "" {
			resolved.CA = p.CA
		}
		if resolved.Token == "" {
			resolved.Token = p.Token
		}
		if resolved.ClientCert == "" {
			resolved.ClientCert = p.ClientCert
		}
		if resolved.ClientKey == "" {
			resolved.ClientKey = p.ClientKey
		}
		if !resolved.SkipTLSVerify {
			resolved.SkipTLSVerify = p.SkipTLSVerify
		}
	}
	if resolved.ConnectionMode == "" {
		resolved.ConnectionMode = "auto"
	}
	if explicitMode == "" || strings.EqualFold(explicitMode, "auto") {
		switch {
		case explicitServer != "":
			resolved.ConnectionMode = "endpoint"
		}
	}

	mode := strings.ToLower(strings.TrimSpace(resolved.ConnectionMode))
	if mode == "auto" {
		switch {
		case resolved.Kubeconfig != "":
			mode = "kubeconfig"
		case resolved.Server != "":
			mode = "endpoint"
		default:
			return defaultRESTConfig(resolved.Context, profileName)
		}
	}
	switch mode {
	case "kubeconfig":
		if strings.TrimSpace(resolved.Kubeconfig) == "" {
			return nil, profileName, errors.New("Kubernetes kubeconfig is required")
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resolved.Kubeconfig))
		if err != nil {
			return nil, profileName, fmt.Errorf("decode kubeconfig: %w", err)
		}
		return restConfigFromBytes(decoded, resolved.Context, profileName)
	case "endpoint":
		return endpointConfig(resolved, profileName)
	case "in_cluster", "in-cluster":
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, profileName, fmt.Errorf("load in-cluster config: %w", err)
		}
		return config, profileName, nil
	default:
		return nil, profileName, fmt.Errorf("unsupported Kubernetes connection_mode %q", resolved.ConnectionMode)
	}
}

func endpointConfig(input ConnectionInput, profileName string) (*rest.Config, string, error) {
	if strings.TrimSpace(input.Server) == "" {
		return nil, profileName, errors.New("Kubernetes server is required for endpoint mode")
	}
	config := &rest.Config{Host: input.Server}
	config.TLSClientConfig.Insecure = input.SkipTLSVerify
	applyTLSMaterial(input.CA, &config.CAData)
	if strings.TrimSpace(input.Token) != "" {
		config.BearerToken = strings.TrimSpace(input.Token)
	}
	if input.ClientCert != "" || input.ClientKey != "" {
		if input.ClientCert == "" || input.ClientKey == "" {
			return nil, profileName, errors.New("Kubernetes client certificate and key are required together")
		}
		applyTLSMaterial(input.ClientCert, &config.CertData)
		applyTLSMaterial(input.ClientKey, &config.KeyData)
	}
	return config, profileName, nil
}

func applyTLSMaterial(value string, data *[]byte) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if strings.Contains(value, "-----BEGIN") {
		*data = []byte(value)
		return
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		*data = decoded
		return
	}
	*data = []byte(value)
}

func restConfigFromBytes(data []byte, contextName, profileName string) (*rest.Config, string, error) {
	config, err := clientcmd.Load(data)
	if err != nil {
		return nil, profileName, fmt.Errorf("load kubeconfig: %w", err)
	}
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{CurrentContext: strings.TrimSpace(contextName)}).ClientConfig()
	if err != nil {
		return nil, profileName, fmt.Errorf("resolve kubeconfig: %w", err)
	}
	return restConfig, profileName, nil
}

func defaultRESTConfig(contextName, profileName string) (*rest.Config, string, error) {
	loading := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{CurrentContext: strings.TrimSpace(contextName)})
	config, kubeconfigErr := loading.ClientConfig()
	if kubeconfigErr == nil {
		return config, profileName, nil
	}
	config, inClusterErr := rest.InClusterConfig()
	if inClusterErr == nil {
		return config, profileName, nil
	}
	return nil, profileName, fmt.Errorf("load Kubernetes default config: %v; load in-cluster config: %v", kubeconfigErr, inClusterErr)
}

func loadProfile(name string) (profile, error) {
	path := strings.TrimSpace(os.Getenv("KUBERNETES_MCP_PROFILES_FILE"))
	if path == "" {
		return profile{}, fmt.Errorf("profile %q requested but KUBERNETES_MCP_PROFILES_FILE is empty", name)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return profile{}, fmt.Errorf("read Kubernetes profiles: %w", err)
	}
	var file profileFile
	if err := yamlUnmarshal(data, &file); err != nil {
		return profile{}, fmt.Errorf("parse Kubernetes profiles: %w", err)
	}
	p, ok := file.Profiles[name]
	if !ok {
		return profile{}, fmt.Errorf("Kubernetes profile %q not found", name)
	}
	return p, nil
}

func envBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

// kept as a variable for small tests without coupling callers to a YAML package.
var yamlUnmarshal = yaml.Unmarshal
