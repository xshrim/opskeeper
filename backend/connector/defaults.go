package connector

import "net/http"

func DefaultRegistry(limits Limits) (*Registry, error) {
	registry := NewRegistry()
	client := &http.Client{CheckRedirect: sameHostRedirect}
	registrations := []struct {
		kind    string
		factory Factory
	}{
		{kind: "Kubernetes", factory: func(target Target) (Adapter, error) {
			return newKubernetesAdapter(target, limits)
		}},
		{kind: "Prometheus", factory: func(target Target) (Adapter, error) {
			return newPrometheusAdapter(target, client, limits)
		}},
		{kind: "Loki", factory: func(target Target) (Adapter, error) {
			return newLokiAdapter(target, client, limits)
		}},
	}
	for _, registration := range registrations {
		if err := registry.Register(registration.kind, 1, 0, registration.factory); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
