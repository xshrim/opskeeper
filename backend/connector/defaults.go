package connector

import "net/http"

func DefaultRegistry(limits Limits) (*Registry, error) {
	registry := NewRegistry()
	client := &http.Client{CheckRedirect: sameHostRedirect}
	registrations := []struct {
		kind    string
		factory Factory
	}{
		{kind: "Application", factory: func(target Target) (Adapter, error) { return applicationAdapter{}, nil }},
		{kind: "Docker", factory: func(target Target) (Adapter, error) {
			return newDockerAdapter(target), nil
		}},
		{kind: "Host", factory: func(target Target) (Adapter, error) {
			return newHostAdapter(target)
		}},
		{kind: "Kubernetes", factory: func(target Target) (Adapter, error) {
			return newKubernetesAdapter(target, limits)
		}},
		{kind: "Prometheus", factory: func(target Target) (Adapter, error) {
			return newPrometheusAdapter(target, client, limits)
		}},
		{kind: "Loki", factory: func(target Target) (Adapter, error) {
			return newLokiAdapter(target, client, limits)
		}},
		{kind: "PostgreSQL", factory: func(target Target) (Adapter, error) {
			return newPostgreSQLAdapter(target, limits)
		}},
		{kind: "Redis", factory: func(target Target) (Adapter, error) {
			return newRedisAdapter(target, limits)
		}},
		{kind: "Nacos", factory: func(target Target) (Adapter, error) { return newNacosAdapter(target, limits) }},
		{kind: "Kafka", factory: func(target Target) (Adapter, error) {
			return newKafkaAdapter(target, limits)
		}},
	}
	for _, registration := range registrations {
		if err := registry.Register(registration.kind, 1, 0, registration.factory); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
