package operation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"opskeeper/backend/resource"
)

type JobSubmitter interface {
	Submit(context.Context, Request, string) (string, error)
}

type KubernetesClientFactory func(context.Context, resource.Resource) (kubernetes.Interface, error)

type KubernetesSubmitter struct {
	Resources ResourceReader
	Factory   KubernetesClientFactory
	Image     string
}

func NewInClusterSubmitter(resources ResourceReader, image string) *KubernetesSubmitter {
	return &KubernetesSubmitter{Resources: resources, Image: image, Factory: func(_ context.Context, _ resource.Resource) (kubernetes.Interface, error) {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return kubernetes.NewForConfig(config)
	}}
}

func (s *KubernetesSubmitter) Submit(ctx context.Context, request Request, executionID string) (string, error) {
	if s == nil || s.Resources == nil || s.Factory == nil || strings.TrimSpace(s.Image) == "" {
		return "", fmt.Errorf("Kubernetes operation submitter is not configured")
	}
	target, err := s.Resources.Get(ctx, request.TargetResourceID)
	if err != nil {
		return "", err
	}
	cluster := target
	if target.SourceResourceID != "" {
		cluster, err = s.Resources.Get(ctx, target.SourceResourceID)
		if err != nil {
			return "", err
		}
	}
	namespace := stringValue(request.Parameters["namespace"])
	if namespace == "" {
		namespace = stringValue(target.Config["namespace"])
	}
	if namespace == "" || len(validation.IsDNS1123Label(namespace)) > 0 {
		return "", fmt.Errorf("a valid Kubernetes namespace is required")
	}
	workload := stringValue(request.Parameters["workload"])
	if workload == "" {
		workload = nestedString(target.Config, "kubernetes", "workload_name")
	}
	if workload == "" {
		workload = target.Name
	}
	if len(validation.IsDNS1123Subdomain(workload)) > 0 {
		return "", fmt.Errorf("a valid Kubernetes workload name is required")
	}
	kind := stringValue(request.Parameters["kind"])
	if kind == "" {
		kind = nestedString(target.Config, "kubernetes", "workload_kind")
	}
	var replicas *int64
	if request.OperationName == ScaleWorkload {
		value, parseErr := integerValue(request.Parameters["replicas"])
		if parseErr != nil {
			return "", parseErr
		}
		replicas = &value
	}
	request.Status = Executing
	raw, err := BuildOperationJob(request, executionID, namespace, s.Image, kind, workload, replicas)
	if err != nil {
		return "", err
	}
	object := &unstructured.Unstructured{Object: raw}
	job := &batchv1.Job{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, job); err != nil {
		return "", fmt.Errorf("convert operation Job: %w", err)
	}
	client, err := s.Factory(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("build Kubernetes client: %w", err)
	}
	created, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return namespace + "/" + job.Name, nil
	}
	if err != nil {
		return "", fmt.Errorf("submit operation Job: %w", err)
	}
	return namespace + "/" + created.Name, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func nestedString(value map[string]any, object, key string) string {
	nested, _ := value[object].(map[string]any)
	return stringValue(nested[key])
}

func integerValue(value any) (int64, error) {
	switch item := value.(type) {
	case float64:
		if item != float64(int64(item)) {
			return 0, fmt.Errorf("replicas must be an integer")
		}
		return int64(item), nil
	case int:
		return int64(item), nil
	case int64:
		return item, nil
	case string:
		return strconv.ParseInt(item, 10, 64)
	default:
		return 0, fmt.Errorf("replicas are required")
	}
}
