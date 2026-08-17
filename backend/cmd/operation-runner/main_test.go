package main

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRestartUpdatesPodTemplateAnnotation(t *testing.T) {
	client := fake.NewSimpleClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "payments", Namespace: "apps"}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{}}})
	if _, err := restart(context.Background(), client, "Deployment", "apps", "payments", "execution-1"); err != nil {
		t.Fatal(err)
	}
	item, err := client.AppsV1().Deployments("apps").Get(context.Background(), "payments", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if item.Spec.Template.Annotations["opskeeper.io/restarted-at"] == "" {
		t.Fatal("restart annotation was not persisted")
	}
}
