package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var executionID, operation, kind, namespace, workload string
	var replicas int
	flag.StringVar(&executionID, "execution-id", "", "immutable OpsKeeper execution id")
	flag.StringVar(&operation, "operation", "", "approved operation")
	flag.StringVar(&kind, "kind", "Deployment", "workload kind")
	flag.StringVar(&namespace, "namespace", "", "workload namespace")
	flag.StringVar(&workload, "workload", "", "workload name")
	flag.IntVar(&replicas, "replicas", -1, "desired replica count")
	flag.Parse()
	if strings.TrimSpace(executionID) == "" || strings.TrimSpace(namespace) == "" || strings.TrimSpace(workload) == "" {
		fatal("execution-id, namespace and workload are required")
	}
	if operation != "kubernetes.restart_workload" && operation != "kubernetes.scale_workload" {
		fatal("operation is not approved")
	}
	if kind != "Deployment" && kind != "StatefulSet" && kind != "DaemonSet" {
		fatal("workload kind is not approved")
	}
	if operation == "kubernetes.scale_workload" && (replicas < 0 || replicas > 100 || kind == "DaemonSet") {
		fatal("scale requires 0..100 replicas and does not support DaemonSet")
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		fatal("load in-cluster config: " + err.Error())
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fatal("create Kubernetes client: " + err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var result map[string]any
	if operation == "kubernetes.restart_workload" {
		result, err = restart(ctx, client, kind, namespace, workload, executionID)
	} else {
		result, err = scale(ctx, client, kind, namespace, workload, replicas)
	}
	if err != nil {
		fatal(err.Error())
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
}

func restart(ctx context.Context, client kubernetes.Interface, kind, namespace, workload, executionID string) (map[string]any, error) {
	annotation := "opskeeper.io/restarted-at"
	value := time.Now().UTC().Format(time.RFC3339Nano) + ":" + executionID
	switch kind {
	case "Deployment":
		item, err := client.AppsV1().Deployments(namespace).Get(ctx, workload, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ensureAnnotations(&item.Spec.Template.ObjectMeta.Annotations)[annotation] = value
		if _, err = client.AppsV1().Deployments(namespace).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
			return nil, err
		}
	case "StatefulSet":
		item, err := client.AppsV1().StatefulSets(namespace).Get(ctx, workload, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ensureAnnotations(&item.Spec.Template.ObjectMeta.Annotations)[annotation] = value
		if _, err = client.AppsV1().StatefulSets(namespace).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
			return nil, err
		}
	case "DaemonSet":
		item, err := client.AppsV1().DaemonSets(namespace).Get(ctx, workload, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ensureAnnotations(&item.Spec.Template.ObjectMeta.Annotations)[annotation] = value
		if _, err = client.AppsV1().DaemonSets(namespace).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported workload kind %s", kind)
	}
	return map[string]any{"operation": "restart", "kind": kind, "namespace": namespace, "workload": workload}, nil
}

func scale(ctx context.Context, client kubernetes.Interface, kind, namespace, workload string, replicas int) (map[string]any, error) {
	desired := int32(replicas)
	scale := &autoscalingv1.Scale{ObjectMeta: metav1.ObjectMeta{Name: workload, Namespace: namespace}, Spec: autoscalingv1.ScaleSpec{Replicas: desired}}
	var err error
	switch kind {
	case "Deployment":
		current, getErr := client.AppsV1().Deployments(namespace).GetScale(ctx, workload, metav1.GetOptions{})
		if getErr != nil {
			return nil, getErr
		}
		scale.ResourceVersion = current.ResourceVersion
		_, err = client.AppsV1().Deployments(namespace).UpdateScale(ctx, workload, scale, metav1.UpdateOptions{})
	case "StatefulSet":
		current, getErr := client.AppsV1().StatefulSets(namespace).GetScale(ctx, workload, metav1.GetOptions{})
		if getErr != nil {
			return nil, getErr
		}
		scale.ResourceVersion = current.ResourceVersion
		_, err = client.AppsV1().StatefulSets(namespace).UpdateScale(ctx, workload, scale, metav1.UpdateOptions{})
	default:
		return nil, fmt.Errorf("unsupported scale workload kind %s", kind)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"operation": "scale", "kind": kind, "namespace": namespace, "workload": workload, "replicas": strconv.Itoa(replicas)}, nil
}

func ensureAnnotations(value *map[string]string) map[string]string {
	if *value == nil {
		*value = map[string]string{}
	}
	return *value
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
