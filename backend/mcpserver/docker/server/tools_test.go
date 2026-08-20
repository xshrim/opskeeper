package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"opskeeper/backend/mcpserver/docker/client"
)

func TestToolsUseDockerEngineReadOnlyEndpoints(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/_ping", "/v1.51/_ping":
			writer.Header().Set("API-Version", "1.51")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("OK"))
		case "/v1.51/info":
			writeJSON(writer, map[string]any{"ID": "daemon-id", "Containers": 2, "Images": 3, "ServerVersion": "test"})
		case "/v1.51/images/json":
			writeJSON(writer, []map[string]any{{"Id": "sha256:image", "RepoTags": []string{"example:test"}}})
		case "/v1.51/containers/json":
			writeJSON(writer, []map[string]any{{"Id": "container-id", "Image": "example:test", "State": "running"}})
		case "/v1.51/containers/container-id/logs":
			payload := []byte("hello from docker\n")
			frame := make([]byte, 8+len(payload))
			frame[0] = 1
			binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
			copy(frame[8:], payload)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(frame)
		case "/v1.51/containers/container-id/json":
			writeJSON(writer, map[string]any{"Id": "container-id", "Config": map[string]any{"Env": []string{"PASSWORD=hidden", "SAFE=value"}}})
		case "/v1.51/containers/container-id/stats":
			writeJSON(writer, map[string]any{"id": "container-id", "name": "/demo", "memory_stats": map[string]any{"usage": 12}})
		default:
			http.NotFound(writer, request)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	input := client.ConnectionInput{DockerHost: server.URL}
	if _, output, err := dockerInfo(context.Background(), nil, DockerInfoInput{ConnectionInput: input}); err != nil || output.Info.ID == "" {
		t.Fatalf("docker info: output=%v err=%v", output, err)
	}
	if _, output, err := listImages(context.Background(), nil, ListImagesInput{ConnectionInput: input}); err != nil || len(output.Images) != 1 {
		t.Fatalf("docker images: output=%v err=%v", output, err)
	}
	if _, output, err := listContainers(context.Background(), nil, ListContainersInput{ConnectionInput: input}); err != nil || len(output.Containers) != 1 {
		t.Fatalf("docker containers: output=%v err=%v", output, err)
	}
	if _, output, err := containerLogs(context.Background(), nil, ContainerLogsInput{ConnectionInput: input, ContainerName: "container-id"}); err != nil || output.Logs != "hello from docker\n" {
		t.Fatalf("docker logs: output=%+v err=%v", output, err)
	}
	if _, output, err := containerInspect(context.Background(), nil, ContainerInspectInput{ConnectionInput: input, ContainerName: "container-id"}); err != nil || output.ID != "container-id" || output.Config == nil || len(output.Config.EnvNames) != 2 || output.Config.EnvNames[0] != "PASSWORD" {
		t.Fatalf("docker inspect: output=%v err=%v", output, err)
	}
	if _, output, err := containerStats(context.Background(), nil, ContainerStatsInput{ConnectionInput: input, ContainerName: "container-id"}); err != nil || output.ID != "container-id" {
		t.Fatalf("docker stats: output=%+v err=%v", output, err)
	}
}

func TestToFiltersParsesKeyValueString(t *testing.T) {
	args, err := toFilters(" label:com.example.env=prod, reference:nginx:latest ")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := args.Get("label"), []string{"com.example.env=prod"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("label filter = %#v, want %#v", got, want)
	}
	if got, want := args.Get("reference"), []string{"nginx:latest"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reference filter = %#v, want %#v", got, want)
	}
}

func TestToFiltersAcceptsEmptyString(t *testing.T) {
	args, err := toFilters("")
	if err != nil {
		t.Fatal(err)
	}
	if got := args.Get("label"); len(got) != 0 {
		t.Fatalf("empty filters produced values: %#v", got)
	}
}

func TestToFiltersRejectsMalformedItems(t *testing.T) {
	for _, raw := range []string{"label", ":value", "key:", "key:value,,other:value"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := toFilters(raw); err == nil {
				t.Fatalf("toFilters(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestResolveContainerIdentifier(t *testing.T) {
	if got, err := resolveContainerIdentifier(" id ", "name"); err != nil || got != "id" {
		t.Fatalf("ID should take precedence: got %q, err=%v", got, err)
	}
	if got, err := resolveContainerIdentifier("", " name "); err != nil || got != "name" {
		t.Fatalf("name fallback failed: got %q, err=%v", got, err)
	}
	if _, err := resolveContainerIdentifier(" ", " "); err == nil {
		t.Fatal("expected missing identifier error")
	}
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}
