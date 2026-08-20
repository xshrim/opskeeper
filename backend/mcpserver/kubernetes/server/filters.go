package server

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func listOptions(raw string, limit int, continuation string) (metav1.ListOptions, error) {
	if limit < 0 {
		return metav1.ListOptions{}, fmt.Errorf("limit must not be negative")
	}
	if limit == 0 {
		limit = 100
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	selector, err := labelsFromString(raw)
	if err != nil {
		return metav1.ListOptions{}, err
	}
	return metav1.ListOptions{Limit: int64(limit), Continue: strings.TrimSpace(continuation), LabelSelector: selector}, nil
}

func labelsFromString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parts := strings.Split(raw, ",")
	for _, item := range parts {
		if !strings.Contains(item, ":") {
			return "", fmt.Errorf("invalid filter %q: expected key:value", strings.TrimSpace(item))
		}
	}
	labels := make([]string, 0, len(parts))
	for _, item := range parts {
		key, value, _ := strings.Cut(item, ":")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "" || value == "" {
			return "", fmt.Errorf("invalid filter %q: key and value are required", item)
		}
		labels = append(labels, key+"="+value)
	}
	return strings.Join(labels, ","), nil
}
