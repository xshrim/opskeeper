package connector

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

const testKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://cluster.example:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user: {}
`

func TestKubeconfigSecretNormalizesStoredBase64(t *testing.T) {
	expected := strings.TrimSpace(testKubeconfig)
	encoded := base64.StdEncoding.EncodeToString([]byte(expected))
	secret, err := json.Marshal(map[string]string{"kubeconfig": encoded})
	if err != nil {
		t.Fatal(err)
	}
	value, err := kubeconfigSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if value != expected {
		t.Fatalf("normalized kubeconfig differs from original")
	}
}

func TestKubeconfigSecretAcceptsRawYAML(t *testing.T) {
	expected := strings.TrimSpace(testKubeconfig)
	secret, err := json.Marshal(map[string]string{"kubeconfig": expected})
	if err != nil {
		t.Fatal(err)
	}
	value, err := kubeconfigSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if value != expected {
		t.Fatalf("raw kubeconfig was unexpectedly changed")
	}
}
