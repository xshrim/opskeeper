package main

import "testing"

func TestAccessURL(t *testing.T) {
	tests := []struct {
		name     string
		listen   string
		basePath string
		want     string
	}{
		{name: "wildcard ipv4", listen: ":8080", basePath: "/opskeeper", want: "http://localhost:8080/opskeeper/"},
		{name: "wildcard ipv6", listen: "[::]:9090", basePath: "/", want: "http://localhost:9090/"},
		{name: "explicit host", listen: "127.0.0.1:8080", basePath: "/opskeeper", want: "http://127.0.0.1:8080/opskeeper/"},
		{name: "explicit ipv6", listen: "[2001:db8::1]:8080", basePath: "/opskeeper", want: "http://[2001:db8::1]:8080/opskeeper/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := accessURL(test.listen, test.basePath); got != test.want {
				t.Fatalf("accessURL(%q, %q) = %q, want %q", test.listen, test.basePath, got, test.want)
			}
		})
	}
}
