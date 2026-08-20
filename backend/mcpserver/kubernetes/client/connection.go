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
	KubeconfigBase64 string `json:"kubeconfig_base64,omitempty"`
	ConnectionMode   string `json:"connection_mode,omitempty"`
	KubeconfigPath   string `json:"kubeconfig_path,omitempty"`
	Context          string `json:"context,omitempty"`
	Profile          string `json:"profile,omitempty"`
	Server           string `json:"server,omitempty"`
	CAFile           string `json:"ca_file,omitempty"`
	TokenFile        string `json:"token_file,omitempty"`
	ClientCertFile   string `json:"client_cert_file,omitempty"`
	ClientKeyFile    string `json:"client_key_file,omitempty"`
	SkipTLSVerify    bool   `json:"skip_tls_verify,omitempty"`
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
	Mode           string `json:"mode" yaml:"mode"`
	Path           string `json:"kubeconfig_path" yaml:"kubeconfig_path"`
	Context        string `json:"context" yaml:"context"`
	Server         string `json:"server" yaml:"server"`
	CAFile         string `json:"ca_file" yaml:"ca_file"`
	TokenFile      string `json:"token_file" yaml:"token_file"`
	ClientCertFile string `json:"client_cert_file" yaml:"client_cert_file"`
	ClientKeyFile  string `json:"client_key_file" yaml:"client_key_file"`
	SkipTLSVerify  bool   `json:"skip_tls_verify" yaml:"skip_tls_verify"`
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
	if raw := strings.TrimSpace(input.KubeconfigBase64); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "tool-kubeconfig", fmt.Errorf("decode kubeconfig_base64: %w", err)
		}
		config, err := clientcmd.RESTConfigFromKubeConfig(decoded)
		if err != nil {
			return nil, "tool-kubeconfig", fmt.Errorf("load tool kubeconfig: %w", err)
		}
		return config, "tool-kubeconfig", nil
	}
	resolved := input
	profileName := strings.TrimSpace(resolved.Profile)
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
		if resolved.KubeconfigPath == "" {
			resolved.KubeconfigPath = p.Path
		}
		if resolved.Context == "" {
			resolved.Context = p.Context
		}
		if resolved.Server == "" {
			resolved.Server = p.Server
		}
		if resolved.CAFile == "" {
			resolved.CAFile = p.CAFile
		}
		if resolved.TokenFile == "" {
			resolved.TokenFile = p.TokenFile
		}
		if resolved.ClientCertFile == "" {
			resolved.ClientCertFile = p.ClientCertFile
		}
		if resolved.ClientKeyFile == "" {
			resolved.ClientKeyFile = p.ClientKeyFile
		}
		if !resolved.SkipTLSVerify {
			resolved.SkipTLSVerify = p.SkipTLSVerify
		}
	}
	if resolved.ConnectionMode == "" {
		resolved.ConnectionMode = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_MODE"))
	}
	if resolved.ConnectionMode == "" {
		resolved.ConnectionMode = "auto"
	}
	if resolved.KubeconfigPath == "" {
		resolved.KubeconfigPath = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_KUBECONFIG"))
	}
	if resolved.Context == "" {
		resolved.Context = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CONTEXT"))
	}
	if resolved.Server == "" {
		resolved.Server = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_SERVER"))
	}
	if resolved.CAFile == "" {
		resolved.CAFile = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CA_FILE"))
	}
	if resolved.TokenFile == "" {
		resolved.TokenFile = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_TOKEN_FILE"))
	}
	if resolved.ClientCertFile == "" {
		resolved.ClientCertFile = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CLIENT_CERT_FILE"))
	}
	if resolved.ClientKeyFile == "" {
		resolved.ClientKeyFile = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_CLIENT_KEY_FILE"))
	}
	if !resolved.SkipTLSVerify {
		resolved.SkipTLSVerify = envBool("KUBERNETES_MCP_SKIP_TLS_VERIFY", false)
	}

	mode := strings.ToLower(strings.TrimSpace(resolved.ConnectionMode))
	if mode == "auto" {
		switch {
		case resolved.Server != "":
			mode = "endpoint"
		case resolved.KubeconfigPath != "":
			mode = "kubeconfig"
		default:
			mode = "in_cluster"
		}
	}
	switch mode {
	case "kubeconfig":
		if resolved.KubeconfigPath == "" {
			return nil, profileName, errors.New("Kubernetes kubeconfig path is required")
		}
		loading := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: resolved.KubeconfigPath}, &clientcmd.ConfigOverrides{CurrentContext: resolved.Context})
		config, err := loading.ClientConfig()
		if err != nil {
			return nil, profileName, fmt.Errorf("load kubeconfig: %w", err)
		}
		return config, profileName, nil
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
	if input.CAFile != "" {
		config.CAFile = input.CAFile
	}
	if input.TokenFile != "" {
		data, err := os.ReadFile(input.TokenFile)
		if err != nil {
			return nil, profileName, fmt.Errorf("read Kubernetes token file: %w", err)
		}
		config.BearerToken = strings.TrimSpace(string(data))
	}
	if config.BearerToken == "" {
		config.BearerToken = strings.TrimSpace(os.Getenv("KUBERNETES_MCP_BEARER_TOKEN"))
	}
	if input.ClientCertFile != "" || input.ClientKeyFile != "" {
		if input.ClientCertFile == "" || input.ClientKeyFile == "" {
			return nil, profileName, errors.New("Kubernetes client certificate and key are required together")
		}
		config.CertFile, config.KeyFile = input.ClientCertFile, input.ClientKeyFile
	}
	return config, profileName, nil
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
